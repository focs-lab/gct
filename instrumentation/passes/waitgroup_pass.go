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

var WaitGroupAnalyzer = &analysis.Analyzer{
	Name: "waitgroup_instr",
	Doc:  "instrument sync.WaitGroup Add/Done/Wait",
	Run:  runWaitGroupPass,
	Requires: []*analysis.Analyzer{
		GoroutineAnalyzer,
	},
}

type WaitGroupRewriter struct {
	pass     *analysis.Pass
	instrCtx *InstrContext
}

func isWaitGroupTypeOrEmbedded(pass *analysis.Pass, t types.Type) bool {
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		t = ptr.Elem()
	}

	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "sync" && obj.Name() == "WaitGroup" {
			return true
		}
	}

	if s, ok := t.Underlying().(*types.Struct); ok {
		for i := 0; i < s.NumFields(); i++ {
			field := s.Field(i)
			if field.Embedded() && isWaitGroupTypeOrEmbedded(pass, field.Type()) {
				return true
			}
		}
	}

	return false
}

func (m *WaitGroupRewriter) isWaitGroupRelatedCallExpr(callExpr *ast.CallExpr) (string, bool) {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	receiverType := utils.GetReceiverType(m.pass.TypesInfo, selExpr.X)
	if receiverType == nil {
		return "", false
	}

	if !isWaitGroupTypeOrEmbedded(m.pass, receiverType) {
		return "", false
	}

	methodName := selExpr.Sel.Name
	if m.isWaitGroupRelated(methodName) {
		return methodName, true
	} else {
		return "", false
	}
}

func (m *WaitGroupRewriter) getReceiverOfCallExpr(callExpr *ast.CallExpr) ast.Expr {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Check if this is a method call on an embedded field.
	if selection, ok := m.pass.TypesInfo.Selections[selExpr]; ok && len(selection.Index()) > 1 {
		recv := selExpr.X
		var finalFieldType types.Type
		for _, i := range selection.Index()[:len(selection.Index())-1] {
			currentRecvType := selection.Recv()
			if ptr, isPtr := currentRecvType.(*types.Pointer); isPtr {
				currentRecvType = ptr.Elem()
			}
			structType := currentRecvType.Underlying().(*types.Struct)
			f := structType.Field(i)
			finalFieldType = f.Type()
			recv = &ast.SelectorExpr{X: recv, Sel: ast.NewIdent(f.Name())}
		}

		if _, isPtr := finalFieldType.(*types.Pointer); !isPtr {
			return &ast.UnaryExpr{Op: token.AND, X: recv}
		}
		return recv
	}

	if utils.IsPointerType(m.pass, selExpr.X) {
		return selExpr.X
	}
	return &ast.UnaryExpr{Op: token.AND, X: selExpr.X}
}

func (m *WaitGroupRewriter) handleExprStmt(c *astutil.Cursor, exprStmt *ast.ExprStmt) bool {
	callExpr, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return true
	}

	methodName, isWaitGroupRelated := m.isWaitGroupRelatedCallExpr(callExpr)

	if !isWaitGroupRelated {
		return true
	}

	receiverExpr := m.getReceiverOfCallExpr(callExpr)

	if methodName == "Go" {
		if len(callExpr.Args) != 1 {
			return true // wg.Go should have one argument
		}
		funcLit, ok := callExpr.Args[0].(*ast.FuncLit)
		if !ok {
			return true // Argument is not a function literal, skip.
		}

		taskRunOpId := m.instrCtx.GetNewOpid()
		taskRunOpIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(taskRunOpId, 10)}
		beforeTaskRunCall := utils.CreateMonitorSyncObjCall(monitor.AFTER_WAITGROUP_GO_RUN, receiverExpr, taskRunOpIdLit)
		afterTaskRunCall := utils.CreateMonitorSyncObjCall(monitor.BEFORE_WAITGROUP_GO_END, receiverExpr, taskRunOpIdLit)

		funcLit.Body.List = append([]ast.Stmt{&ast.ExprStmt{X: beforeTaskRunCall}, &ast.DeferStmt{Call: afterTaskRunCall}}, funcLit.Body.List...)
		return true // We have instrumented the inner function, continue.
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName)
	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	// Prepare additional arguments for the monitor call.
	// The receiverExpr is passed as a separate argument to CreateMonitorSyncObjCall.
	var additionalArgs []ast.Expr
	if methodName == "Add" {
		additionalArgs = []ast.Expr{callExpr.Args[0], opIdLit}
	} else {
		additionalArgs = []ast.Expr{opIdLit}
	}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, additionalArgs...)
	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, additionalArgs...)
	c.InsertBefore(&ast.ExprStmt{X: beforeExpr})
	c.InsertAfter(&ast.ExprStmt{X: afterExpr})

	return true
}

func (m *WaitGroupRewriter) handleDeferStmt(c *astutil.Cursor, deferStmt *ast.DeferStmt) bool {
	callExpr := deferStmt.Call
	if callExpr == nil {
		return true
	}

	methodName, isWaitGroupRelated := m.isWaitGroupRelatedCallExpr(callExpr)

	// Only `Done` is commonly deferred.
	if !isWaitGroupRelated || methodName != "Done" {
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

func (m *WaitGroupRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func (m *WaitGroupRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	n := c.Node()
	if n == nil {
		return true
	}

	switch stmt := n.(type) {
	case *ast.ExprStmt:
		return m.handleExprStmt(c, stmt)
	case *ast.DeferStmt:
		return m.handleDeferStmt(c, stmt)
	}

	return true
}

func (m *WaitGroupRewriter) getInstrumentCallName(callName string) (string, string) {
	switch callName {
	case "Add":
		return monitor.BEFORE_WAITGROUP_ADD, monitor.AFTER_WAITGROUP_ADD
	case "Done":
		return monitor.BEFORE_WAITGROUP_DONE, monitor.AFTER_WAITGROUP_DONE
	case "Wait":
		return monitor.BEFORE_WAITGROUP_WAIT, monitor.AFTER_WAITGROUP_WAIT
	}
	return "", ""
}

func (m *WaitGroupRewriter) isWaitGroupRelated(funcName string) bool {
	switch funcName {
	case "Add",
		"Done", "Go",
		"Wait": // Add "Go" here
		return true
	default:
		return false
	}
}

func runWaitGroupPass(pass *analysis.Pass) (interface{}, error) {
	rewriter := &WaitGroupRewriter{
		pass:     pass,
		instrCtx: SharedInstrCtx,
	}
	for _, file := range pass.Files {
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
	}
	return nil, nil
}
