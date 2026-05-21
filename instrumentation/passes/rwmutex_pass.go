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

var RWMutexAnalyzer = &analysis.Analyzer{
	Name: "rwmutex_instr",
	Doc:  "instrument rwmutex lock/unlock/rlock/runlock/trylock/tryrlock",
	Run:  runRWMutexPass,
	Requires: []*analysis.Analyzer{
		MutexAnalyzer,
	},
}

type RWMutexRewriter struct {
	pass                 *analysis.Pass
	instrCtx             *InstrContext
	processedAssignStmts map[*ast.AssignStmt]bool
}

func (m *RWMutexRewriter) isLockRelatedCallExpr(callExpr *ast.CallExpr) (string, bool) {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	receiverType := utils.GetReceiverType(m.pass.TypesInfo, selExpr.X)
	if receiverType == nil {
		return "", false
	}

	// Handle calls on rlocker variables, e.g., sr.Lock() where sr is from RWMutex.RLocker()
	if selection, ok := m.pass.TypesInfo.Selections[selExpr]; ok {
		recvType := selection.Recv()

		// 1. Unwrap pointer if the receiver is a pointer to rlocker
		if ptr, isPtr := recvType.(*types.Pointer); isPtr {
			recvType = ptr.Elem()
		}

		// 2. Check if the type is the concrete unexported "sync.rlocker"
		if named, ok := recvType.(*types.Named); ok && named.Obj() != nil {
			obj := named.Obj()

			// Match the secret unexported type returned by RWMutex.RLocker()
			if obj.Pkg() != nil && obj.Pkg().Path() == "sync" && obj.Name() == "rlocker" {
				switch selExpr.Sel.Name {
				case "Lock":
					return "RLock", true
				case "Unlock":
					return "RUnlock", true
				}
			}
		}
	}

	if !isRWMutexTypeOrEmbedded(m.pass, receiverType) {
		return "", false
	}

	methodName := selExpr.Sel.Name
	if m.isRWMutexRelated(methodName) {
		return methodName, true
	} else {
		return "", false
	}
}

/*
For CallExpr like "m.Lock()", it returns "m" as an Expr.
Note that if "m" is a pointer type, it returns "m"
If "m" is not a pointer type, it returns "&m"
*/
func (m *RWMutexRewriter) getReceiverOfCallExpr(callExpr *ast.CallExpr) ast.Expr {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// If it's a call on an rlocker, we need to get the underlying RWMutex.
	// We can do this with a type assertion.
	receiverType := utils.GetReceiverType(m.pass.TypesInfo, selExpr.X)
	if named, ok := receiverType.Underlying().(*types.Named); ok && named.Obj() != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "sync" && named.Obj().Name() == "Locker" {
		if selection, ok := m.pass.TypesInfo.Selections[selExpr]; ok {
			methodSet := types.NewMethodSet(selection.Recv())
			if sel := methodSet.Lookup(m.pass.Pkg, "RLocker"); sel != nil && sel.Obj() != nil {
				// The receiver is the rlocker (e.g., `sr`). We need to cast it back to *sync.RWMutex.
				// The instrumentation should have already replaced `sync.RWMutex` with `sync_go_cct.RWMutex`.
				return &ast.TypeAssertExpr{
					X:    selExpr.X,
					Type: ast.NewIdent("*sync.RWMutex"), // This will be resolved to the correct shadow type.
				}
			}
		}
	}

	// Check if this is a method call on an embedded field.
	// e.g., type S struct { sync.RWMutex }; var s S; s.Lock()
	// Here, s.Lock is implicitly s.RWMutex.Lock().
	if selection, ok := m.pass.TypesInfo.Selections[selExpr]; ok && len(selection.Index()) > 1 {
		recv := selExpr.X
		var finalFieldType types.Type
		for _, i := range selection.Index()[:len(selection.Index())-1] {
			// We need to look at the receiver type of the selection to find the struct
			// and then the field at index i.
			currentRecvType := selection.Recv()
			if ptr, isPtr := currentRecvType.(*types.Pointer); isPtr {
				currentRecvType = ptr.Elem()
			}
			structType := currentRecvType.Underlying().(*types.Struct)
			f := structType.Field(i)
			finalFieldType = f.Type()
			recv = &ast.SelectorExpr{X: recv, Sel: ast.NewIdent(f.Name())}
		}

		// `recv` is now the expression for the embedded field, e.g., `s.RWMutex`.
		// `finalFieldType` is the type of that field, e.g., `sync.RWMutex` or `*sync.RWMutex`.
		if _, isPtr := finalFieldType.(*types.Pointer); !isPtr {
			return &ast.UnaryExpr{Op: token.AND, X: recv}
		}
		return recv
	}

	// Original logic for non-embedded or direct calls
	if utils.IsPointerType(m.pass, selExpr.X) {
		return selExpr.X
	}
	return &ast.UnaryExpr{Op: token.AND, X: selExpr.X}
}

func (m *RWMutexRewriter) handleExprStmt(c *astutil.Cursor, exprStmt *ast.ExprStmt) bool {
	callExpr, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}

	methodName, isLockRelated := m.isLockRelatedCallExpr(callExpr)

	if !isLockRelated || m.isRWMutexTry(methodName) {
		return false
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName)

	receiverExpr := m.getReceiverOfCallExpr(callExpr)

	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.ExprStmt{X: beforeExpr})
	c.InsertAfter(&ast.ExprStmt{X: afterExpr})

	return true
}

func (m *RWMutexRewriter) handleAssignStmt(c *astutil.Cursor, assignStmt *ast.AssignStmt) bool {
	// If this assignment statement has been processed by handleIfStmt, skip it.
	if m.processedAssignStmts[assignStmt] {
		return true
	}

	lhsExprs := assignStmt.Lhs
	if len(lhsExprs) > 1 {
		// TryLock should only have one lhs
		return true
	}

	callExpr, ok := assignStmt.Rhs[0].(*ast.CallExpr)
	if !ok {
		return true
	}

	methodName, isLockRelated := m.isLockRelatedCallExpr(callExpr)

	if !isLockRelated || !m.isRWMutexTry(methodName) {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName)

	receiverExpr := m.getReceiverOfCallExpr(callExpr)

	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.ExprStmt{X: beforeExpr})

	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	lhsExpr := lhsExprs[0]
	c.InsertAfter(&ast.IfStmt{
		Cond: lhsExpr,
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: afterExpr}}},
	})

	return true
}

func (m *RWMutexRewriter) handleDeferStmt(c *astutil.Cursor, deferStmt *ast.DeferStmt) bool {
	/*
		before: defer m.Unlock()

		after:
			defer monitor.AfterUnlock()
			defer m.Unlock()
			defer monitor.BeforeUnlock()
	*/
	callExpr := deferStmt.Call
	if callExpr == nil {
		return true
	}

	methodName, isLockRelated := m.isLockRelatedCallExpr(callExpr)

	if !isLockRelated || !m.isRWMutexUnlock(methodName) {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName)
	receiverExpr := m.getReceiverOfCallExpr(callExpr)

	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.DeferStmt{Call: afterExpr})
	c.InsertAfter(&ast.DeferStmt{Call: beforeExpr})

	return true
}

func (m *RWMutexRewriter) handleIfStmt(c *astutil.Cursor, stmt *ast.IfStmt) bool {
	/*
		before:
			if flag := TryLock(); flag {
				...
			}

		after:
			monitor.BeforeTryLock();
			if flag := m.TryLock(); flag {
				monitor.AfterTryLock();
			}
	*/
	assignStmt, ok := stmt.Init.(*ast.AssignStmt)
	if !ok {
		return true
	}

	lhsExprs := assignStmt.Lhs
	if len(lhsExprs) > 1 {
		return true
	}

	// lhsExpr := lhsExprs[0]
	rhsExpr := assignStmt.Rhs[0]

	rhsCallExpr, ok := rhsExpr.(*ast.CallExpr)
	if !ok {
		return true
	}

	methodName, ok := m.isLockRelatedCallExpr(rhsCallExpr)
	if !ok || !m.isRWMutexTry(methodName) {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName)

	receiverExpr := m.getReceiverOfCallExpr(rhsCallExpr)

	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.ExprStmt{X: beforeExpr})

	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	stmt.Body.List = append(stmt.Body.List, &ast.ExprStmt{X: afterExpr})

	// Mark this assignment as processed to avoid double instrumentation by handleAssignStmt.
	m.processedAssignStmts[assignStmt] = true

	return true // Continue traversal into the if block.
}

func (m *RWMutexRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func (m *RWMutexRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	n := c.Node()
	if n == nil {
		return true
	}

	switch stmt := n.(type) {
	case *ast.ExprStmt:
		// Case 1: Standalone expression "m.Lock()"
		return m.handleExprStmt(c, stmt)

	case *ast.AssignStmt:
		// Case 2: Assignment statement (e.g., "ok := m.TryLock()")
		return m.handleAssignStmt(c, stmt)

	case *ast.DeferStmt:
		// Case 3: Defer statement (e.g., "defer m.Unlock()")
		return m.handleDeferStmt(c, stmt)

	case *ast.IfStmt:
		// Case 4: If ok := m.TryLock(); ok {...}
		return m.handleIfStmt(c, stmt)
	}

	return true
}

func (m *RWMutexRewriter) getInstrumentCallName(callName string) (string, string) {
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

func (m *RWMutexRewriter) isRWMutexUnlock(funcName string) bool {
	return funcName == "Unlock" || funcName == "RUnlock"
}

func (m *RWMutexRewriter) isRWMutexTry(funcName string) bool {
	return funcName == "TryLock" || funcName == "TryRLock"
}

func (m *RWMutexRewriter) isRWMutexRelated(funcName string) bool {
	switch funcName {
	case "Lock",
		"Unlock",
		"RLock",
		"RUnlock",
		"TryLock",
		"TryRLock":
		return true
	default:
		return false
	}
}

func runRWMutexPass(pass *analysis.Pass) (interface{}, error) {
	rewriter := &RWMutexRewriter{
		pass:                 pass,
		instrCtx:             SharedInstrCtx,
		processedAssignStmts: make(map[*ast.AssignStmt]bool),
	}
	for _, file := range pass.Files {
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
	}
	return nil, nil
}
