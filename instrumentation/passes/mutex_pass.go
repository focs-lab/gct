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

var MutexAnalyzer = &analysis.Analyzer{
	Name: "mutex_instr",
	Doc:  "instrument mutex lock/unlock/trylock",
	Run:  runMutexPass,
	Requires: []*analysis.Analyzer{
		GoroutineAnalyzer,
	},
}

type MutexRewriter struct {
	pass                 *analysis.Pass
	instrCtx             *InstrContext
	processedAssignStmts map[*ast.AssignStmt]bool
}

func (m *MutexRewriter) isLockRelatedCallExpr(callExpr *ast.CallExpr) (string, bool) {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	receiverType := utils.GetReceiverType(m.pass.TypesInfo, selExpr.X)
	if receiverType == nil {
		return "", false
	}

	if !isMutexTypeOrEmbedded(m.pass, receiverType) {
		return "", false
	}

	methodName := selExpr.Sel.Name
	if m.isMutexRelated(methodName) {
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
func (m *MutexRewriter) getReceiverOfCallExpr(callExpr *ast.CallExpr) ast.Expr {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Check if this is a method call on an embedded field.
	// e.g., type S struct { sync.Mutex }; var s S; s.Lock()
	// Here, s.Lock is implicitly s.Mutex.Lock().
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

		// `recv` is now the expression for the embedded field, e.g., `s.Mutex`.
		// `finalFieldType` is the type of that field, e.g., `sync.Mutex` or `*sync.Mutex`.
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

func (m *MutexRewriter) handleExprStmt(c *astutil.Cursor, exprStmt *ast.ExprStmt) bool {
	callExpr, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return true
	}

	methodName, isLockRelated := m.isLockRelatedCallExpr(callExpr)

	if !isLockRelated || m.isMutexTryLock(methodName) {
		return true
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

func (m *MutexRewriter) handleAssignStmt(c *astutil.Cursor, assignStmt *ast.AssignStmt) bool {
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

	if !isLockRelated || !m.isMutexTryLock(methodName) {
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

func (m *MutexRewriter) handleDeferStmt(c *astutil.Cursor, deferStmt *ast.DeferStmt) bool {
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

	if !isLockRelated || !m.isMutexUnock(methodName) {
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

func (m *MutexRewriter) handleIfStmt(c *astutil.Cursor, stmt *ast.IfStmt) bool {
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
	if !ok || !m.isMutexTryLock(methodName) {
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

func (m *MutexRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func (m *MutexRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
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

func (m *MutexRewriter) getInstrumentCallName(callName string) (string, string) {
	switch callName {
	case "Lock":
		return monitor.BEFORE_MUTEX_LOCK, monitor.AFTER_MUTEX_LOCK
	case "Unlock":
		return monitor.BEFORE_MUTEX_UNLOCK, monitor.AFTER_MUTEX_UNLOCK
	case "TryLock":
		return monitor.BEFORE_MUTEX_TRYLOCK, monitor.AFTER_MUTEX_TRYLOCK
	}
	return "", ""
}

func (m *MutexRewriter) isMutexLock(funcName string) bool {
	return funcName == "Lock"
}

func (m *MutexRewriter) isMutexUnock(funcName string) bool {
	return funcName == "Unlock"
}

func (m *MutexRewriter) isMutexTryLock(funcName string) bool {
	return funcName == "TryLock"
}

func (m *MutexRewriter) isMutexRelated(funcName string) bool {
	return funcName == "Lock" || funcName == "Unlock" || funcName == "TryLock"
}

func runMutexPass(pass *analysis.Pass) (interface{}, error) {
	rewriter := &MutexRewriter{
		pass:                 pass,
		instrCtx:             SharedInstrCtx,
		processedAssignStmts: make(map[*ast.AssignStmt]bool),
	}
	for _, file := range pass.Files {
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
	}
	return nil, nil
}
