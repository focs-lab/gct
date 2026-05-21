package passes

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/focs-lab/gct/instrumentation/utils"
	"github.com/focs-lab/gct/monitor"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var SyncMutexAnalyzer = &analysis.Analyzer{
	Name: "all_mutex_instr",
	Doc:  "instrument sync.Mutex and sync.RWMutex methods",
	Run:  runSyncPass,
	Requires: []*analysis.Analyzer{
		GoroutineAnalyzer,
	},
}

type SyncRewriter struct {
	pass                 *analysis.Pass
	instrCtx             *InstrContext
	processedAssignStmts map[*ast.AssignStmt]bool
}

func (m *SyncRewriter) isLockRelatedCallExpr(callExpr *ast.CallExpr) (string, bool, bool, bool, bool) {
    selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
    if !ok {
        return "", false, false, false, false
    }

    methodName := selExpr.Sel.Name
    if !m.isSyncRelated(methodName) {
        return "", false, false, false, false
    }


    if selection, ok := m.pass.TypesInfo.Selections[selExpr]; ok && selection.Obj() != nil {
        if recvFunc, isFunc := selection.Obj().(*types.Func); isFunc {
            sig := recvFunc.Type().(*types.Signature)
            if sig != nil && sig.Recv() != nil {
                recvType := sig.Recv().Type()
                if ptr, isPtr := recvType.(*types.Pointer); isPtr {
                    recvType = ptr.Elem()
                }
                
                if named, isNamed := recvType.(*types.Named); isNamed && named.Obj() != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "sync" {
                    typeName := named.Obj().Name()
                    if typeName == "Mutex" {
                        return methodName, true, true, false, false
                    } else if typeName == "RWMutex" {
                        return methodName, true, false, true, false
                    }
                }
            }
        }
    }

    receiverType := utils.GetReceiverType(m.pass.TypesInfo, selExpr.X)
    if receiverType == nil {
        return "", false, false, false, false
    }

    if _, ok := receiverType.Underlying().(*types.Interface); ok {
        if methodName == "Lock" || methodName == "Unlock" {
            return methodName, true, false, false, true
        }
    }

    return "", false, false, false, false
}

func (m *SyncRewriter) getReceiverOfCallExpr(callExpr *ast.CallExpr) ast.Expr {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	var finalType types.Type
	recv := selExpr.X
	hasSelection := false

	if selection, ok := m.pass.TypesInfo.Selections[selExpr]; ok && len(selection.Index()) > 0 {
		hasSelection = true
		currentRecvType := selection.Recv()
		indices := selection.Index()

		for _, idx := range indices {
			// 1. Core breaking check: if we are already at sync.Mutex, stop instantly
			checkType := currentRecvType
			for {
				if ptr, isPtr := checkType.(*types.Pointer); isPtr {
					checkType = ptr.Elem()
					continue
				}
				break
			}
			if named, isNamed := checkType.(*types.Named); isNamed && named.Obj() != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "sync" {
				typeName := named.Obj().Name()
				if typeName == "Mutex" || typeName == "RWMutex" {
					break
				}
			}

			// 2. Unroll the struct to find the next field
			baseType := currentRecvType
			for {
				if ptr, isPtr := baseType.(*types.Pointer); isPtr {
					baseType = ptr.Elem()
					continue
				}
				if named, isNamed := baseType.(*types.Named); isNamed {
					baseType = named.Underlying()
					continue
				}
				break
			}

			structType, ok := baseType.(*types.Struct)
			if !ok || idx < 0 || idx >= structType.NumFields() {
				break
			}

			f := structType.Field(idx)
			recv = &ast.SelectorExpr{
				X:   recv,
				Sel: ast.NewIdent(f.Name()),
			}

			currentRecvType = f.Type()
		}
		finalType = currentRecvType
	} else {
		finalType = m.pass.TypesInfo.TypeOf(selExpr.X)
	}

	// 3. Determine whether we need to take the address (&) of the resolved expression
	if finalType != nil {
		underlyingType := finalType
		for {
			if ptr, isPtr := underlyingType.(*types.Pointer); isPtr {
				underlyingType = ptr.Elem()
				continue
			}
			if named, isNamed := underlyingType.(*types.Named); isNamed {
				underlyingType = named.Underlying()
				continue
			}
			break
		}

		// If the final field type is a pointer or an interface, it's already addressable/valid
		if _, isPtr := finalType.(*types.Pointer); isPtr {
			return recv
		}
		if _, isInterface := underlyingType.(*types.Interface); isInterface {
			return recv
		}
	}

	// If we successfully resolved a selection path, trust the path and check its value-type nature
	if hasSelection {
		return &ast.UnaryExpr{Op: token.AND, X: recv}
	}

	// Fallback mechanism for non-selection direct calls
	receiverType := utils.GetReceiverType(m.pass.TypesInfo, selExpr.X)
	if receiverType != nil {
		if named, isNamed := receiverType.(*types.Named); isNamed {
			receiverType = named.Underlying()
		}
		if _, ok := receiverType.(*types.Interface); ok {
			return selExpr.X
		}
	}

	if utils.IsPointerType(m.pass, selExpr.X) {
		return selExpr.X
	}

	return &ast.UnaryExpr{Op: token.AND, X: recv}
}

func (m *SyncRewriter) handleExprStmt(c *astutil.Cursor, exprStmt *ast.ExprStmt) bool {
	callExpr, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return true
	}

	methodName, isLockRelated, isMutex, isRWMutex, isInterface := m.isLockRelatedCallExpr(callExpr)

	if !isLockRelated || m.isTry(methodName) {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName, isMutex, isRWMutex, isInterface)
	if beforeCallName == "" {
		return true
	}

	receiverExpr := m.getReceiverOfCallExpr(callExpr) // This will correctly handle rlocker type assertion
	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.ExprStmt{X: beforeExpr})
	c.InsertAfter(&ast.ExprStmt{X: afterExpr})

	return true
}

func (m *SyncRewriter) handleAssignStmt(c *astutil.Cursor, assignStmt *ast.AssignStmt) bool {
	if m.processedAssignStmts[assignStmt] {
		return true
	}

	if len(assignStmt.Lhs) > 1 {
		return true
	}

	callExpr, ok := assignStmt.Rhs[0].(*ast.CallExpr)
	if !ok {
		return true
	}

	methodName, isLockRelated, isMutex, isRWMutex, isInterface := m.isLockRelatedCallExpr(callExpr)

	if !isLockRelated || !m.isTry(methodName) {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName, isMutex, isRWMutex, isInterface)
	if beforeCallName == "" {
		return true
	}

	receiverExpr := m.getReceiverOfCallExpr(callExpr) // This will correctly handle rlocker type assertion
	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.ExprStmt{X: beforeExpr})

	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	lhsExpr := assignStmt.Lhs[0]
	c.InsertAfter(&ast.IfStmt{
		Cond: lhsExpr,
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: afterExpr}}},
	})

	return true
}

func (m *SyncRewriter) handleDeferStmt(c *astutil.Cursor, deferStmt *ast.DeferStmt) bool {
	callExpr := deferStmt.Call
	if callExpr == nil {
		return true
	}

	methodName, isLockRelated, isMutex, isRWMutex, isInterface := m.isLockRelatedCallExpr(callExpr)

	if !isLockRelated || !m.isUnlock(methodName) {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName, isMutex, isRWMutex, isInterface)
	if beforeCallName == "" {
		return true
	}

	receiverExpr := m.getReceiverOfCallExpr(callExpr) // This will correctly handle rlocker type assertion
	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.DeferStmt{Call: afterExpr})
	c.InsertAfter(&ast.DeferStmt{Call: beforeExpr})

	return true
}

func (m *SyncRewriter) handleIfStmt(c *astutil.Cursor, stmt *ast.IfStmt) bool {
	assignStmt, ok := stmt.Init.(*ast.AssignStmt)
	if !ok {
		return true
	}

	if len(assignStmt.Lhs) > 1 {
		return true
	}

	rhsExpr := assignStmt.Rhs[0]
	rhsCallExpr, ok := rhsExpr.(*ast.CallExpr)
	if !ok {
		return true
	}

	methodName, isLockRelated, isMutex, isRWMutex, isInterface := m.isLockRelatedCallExpr(rhsCallExpr)
	if !isLockRelated || !m.isTry(methodName) {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName, isMutex, isRWMutex, isInterface)
	if beforeCallName == "" {
		return true
	}

	receiverExpr := m.getReceiverOfCallExpr(rhsCallExpr) // This will correctly handle rlocker type assertion
	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.ExprStmt{X: beforeExpr})

	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	stmt.Body.List = append(stmt.Body.List, &ast.ExprStmt{X: afterExpr})

	m.processedAssignStmts[assignStmt] = true

	return true
}

func (m *SyncRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func (m *SyncRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	n := c.Node()
	if n == nil {
		return true
	}

	switch stmt := n.(type) {
	case *ast.ExprStmt:
		return m.handleExprStmt(c, stmt)
	case *ast.AssignStmt:
		return m.handleAssignStmt(c, stmt)
	case *ast.DeferStmt:
		return m.handleDeferStmt(c, stmt)
	case *ast.IfStmt:
		return m.handleIfStmt(c, stmt)
	}

	return true
}

func (m *SyncRewriter) getInstrumentCallName(callName string, isMutex bool, isRWMutex bool, isInterface bool) (string, string) {
	if isInterface {
		switch callName {
		case "Lock":
			return monitor.BEFORE_INTERFACE_LOCK, monitor.AFTER_INTERFACE_LOCK
		case "Unlock":
			return monitor.BEFORE_INTERFACE_UNLOCK, monitor.AFTER_INTERFACE_UNLOCK
		}
	}

	if isMutex {
		switch callName {
		case "Lock":
			return monitor.BEFORE_MUTEX_LOCK, monitor.AFTER_MUTEX_LOCK
		case "Unlock":
			return monitor.BEFORE_MUTEX_UNLOCK, monitor.AFTER_MUTEX_UNLOCK
		case "TryLock":
			return monitor.BEFORE_MUTEX_TRYLOCK, monitor.AFTER_MUTEX_TRYLOCK
		}
	} else if isRWMutex {
		switch callName {
		case "Lock":
			return monitor.BEFORE_RWMUTEX_LOCK, monitor.AFTER_RWMUTEX_LOCK
		case "Unlock":
			return monitor.BEFORE_RWMUTEX_UNLOCK, monitor.AFTER_RWMUTEX_UNLOCK
		case "TryLock":
			return monitor.BEFORE_RWMUTEX_TRYLOCK, monitor.AFTER_RWMUTEX_TRYLOCK
		case "RLock", "RUnlock", "TryRLock": // These are exclusively RWMutex methods
			return m.getRWMutexInstrumentCallName(callName)
		}
	}
	return "", ""
}

func (m *SyncRewriter) isUnlock(funcName string) bool {
	return funcName == "Unlock" || funcName == "RUnlock"
}

func (m *SyncRewriter) isTry(funcName string) bool {
	return funcName == "TryLock" || funcName == "TryRLock"
}

func (m *SyncRewriter) getRWMutexInstrumentCallName(callName string) (string, string) {
	switch callName {
	case "Lock":
		return monitor.BEFORE_RWMUTEX_LOCK, monitor.AFTER_RWMUTEX_LOCK
	case "Unlock":
		return monitor.BEFORE_RWMUTEX_UNLOCK, monitor.AFTER_RWMUTEX_UNLOCK
	case "TryLock":
		return monitor.BEFORE_RWMUTEX_TRYLOCK, monitor.AFTER_RWMUTEX_TRYLOCK
	case "RLock":
		return monitor.BEFORE_RWMUTEX_RLOCK, monitor.AFTER_RWMUTEX_RLOCK
	case "RUnlock":
		return monitor.BEFORE_RWMUTEX_RUNLOCK, monitor.AFTER_RWMUTEX_RUNLOCK
	case "TryRLock":
		return monitor.BEFORE_RWMUTEX_TRYRLOCK, monitor.AFTER_RWMUTEX_TRYRLOCK
	}
	return "", ""
}

func (m *SyncRewriter) isSyncRelated(funcName string) bool {
	switch funcName {
	case "Lock", "Unlock", "TryLock", "RLock", "RUnlock", "TryRLock":
		return true
	default:
		return false
	}
}

func isMutexTypeOrEmbedded(pass *analysis.Pass, typ types.Type) bool {
	// Step 1: get the sync package
	var syncPkg *types.Package
	for _, p := range pass.Pkg.Imports() {
		if p.Path() == "sync" {
			syncPkg = p
			break
		}
	}
	if syncPkg == nil {
		return false
	}

	// Step 2: get sync.Mutex type
	mutexObj := syncPkg.Scope().Lookup("Mutex")
	if mutexObj == nil {
		return false
	}
	mutexType := mutexObj.Type() // *types.Named

	// Step 3: check if typ is identical to Mutex or *Mutex
	if types.Identical(typ, mutexType) {
		return true
	}
	if ptr, ok := typ.(*types.Pointer); ok && types.Identical(ptr.Elem(), mutexType) {
		return true
	}

	// Step 4: unwrap struct and check embedded fields
	st, ok := typ.Underlying().(*types.Struct)
	if !ok {
		return false
	}

	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		ft := f.Type()
		if types.Identical(ft, mutexType) {
			return true
		}
		if ptr, ok := ft.(*types.Pointer); ok && types.Identical(ptr.Elem(), mutexType) {
			return true
		}
	}

	return false
}

func isRWMutexTypeOrEmbedded(pass *analysis.Pass, typ types.Type) bool {
	// Step 1: get the sync package
	var syncPkg *types.Package
	for _, p := range pass.Pkg.Imports() {
		if p.Path() == "sync" {
			syncPkg = p
			break
		}
	}
	if syncPkg == nil {
		return false
	}

	// Step 2: get sync.Mutex type
	mutexObj := syncPkg.Scope().Lookup("RWMutex")
	if mutexObj == nil {
		return false
	}
	rwmutexType := mutexObj.Type() // *types.Named

	// Step 3: check if typ is identical to RWMutex or *RWMutex
	if types.Identical(typ, rwmutexType) {
		return true
	}
	if ptr, ok := typ.(*types.Pointer); ok && types.Identical(ptr.Elem(), rwmutexType) {
		return true
	}

	// Step 4: unwrap struct and check embedded fields
	st, ok := typ.Underlying().(*types.Struct)
	if !ok {
		return false
	}

	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		ft := f.Type()
		if types.Identical(ft, rwmutexType) {
			return true
		}
		if ptr, ok := ft.(*types.Pointer); ok && types.Identical(ptr.Elem(), rwmutexType) {
			return true
		}
	}

	return false
}

func runSyncPass(pass *analysis.Pass) (interface{}, error) {
	rewriter := &SyncRewriter{
		pass:                 pass,
		instrCtx:             SharedInstrCtx,
		processedAssignStmts: make(map[*ast.AssignStmt]bool),
	}
	for _, file := range pass.Files {
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
	}
	return nil, nil
}

