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

var CondVarAnalyzer = &analysis.Analyzer{
	Name: "cond_instr",
	Doc:  "instrument sync.Cond Wait/Signal/Broadcast",
	Run:  runCondPass,
	Requires: []*analysis.Analyzer{
		GoroutineAnalyzer,
	},
}

type CondRewriter struct {
	pass     *analysis.Pass
	instrCtx *InstrContext
}

// isCondTypeOrEmbedded checks if a type is sync.Cond or a struct embedding sync.Cond.
func isCondTypeOrEmbedded(pass *analysis.Pass, t types.Type) bool {
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		t = ptr.Elem()
	}

	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "sync" && obj.Name() == "Cond" {
			return true
		}
	}

	if s, ok := t.Underlying().(*types.Struct); ok {
		for i := 0; i < s.NumFields(); i++ {
			field := s.Field(i)
			if field.Embedded() && isCondTypeOrEmbedded(pass, field.Type()) {
				return true
			}
		}
	}
	return false
}

func (m *CondRewriter) isCondRelatedCallExpr(callExpr *ast.CallExpr) (string, bool) {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	receiverType := utils.GetReceiverType(m.pass.TypesInfo, selExpr.X)
	if receiverType == nil {
		return "", false
	}

	if !isCondTypeOrEmbedded(m.pass, receiverType) {
		return "", false
	}

	methodName := selExpr.Sel.Name
	if m.isCondMethod(methodName) {
		return methodName, true
	}
	return "", false
}

func (m *CondRewriter) getReceiverOfCallExpr(callExpr *ast.CallExpr) ast.Expr {
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

func (m *CondRewriter) handleExprStmt(c *astutil.Cursor, exprStmt *ast.ExprStmt) bool {
	callExpr, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return true
	}

	methodName, isCondRelated := m.isCondRelatedCallExpr(callExpr)

	if !isCondRelated {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName)
	receiverExpr := m.getReceiverOfCallExpr(callExpr)

	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	// Cond methods (Wait, Signal, Broadcast) take no arguments other than the receiver.
	// The monitor functions will take the receiver and opId.
	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.ExprStmt{X: beforeExpr})
	c.Replace(&ast.ExprStmt{X: afterExpr})

	return true
}

func (m *CondRewriter) handleDeferStmt(c *astutil.Cursor, deferStmt *ast.DeferStmt) bool {
	callExpr := deferStmt.Call
	if callExpr == nil {
		return true
	}

	methodName, isCondRelated := m.isCondRelatedCallExpr(callExpr)

	// Signal and Broadcast might be deferred. Wait is typically not.
	if !isCondRelated || (methodName != "Signal" && methodName != "Broadcast") {
		return true
	}

	beforeCallName, afterCallName := m.getInstrumentCallName(methodName)
	receiverExpr := m.getReceiverOfCallExpr(callExpr)

	opId := m.instrCtx.GetNewOpid()
	opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

	beforeExpr := utils.CreateMonitorSyncObjCall(beforeCallName, receiverExpr, opIdLit)
	afterExpr := utils.CreateMonitorSyncObjCall(afterCallName, receiverExpr, opIdLit)
	c.InsertBefore(&ast.DeferStmt{Call: afterExpr})
	c.Replace(&ast.ExprStmt{X: beforeExpr}) // Before is called immediately, After is deferred.

	return true
}

func (m *CondRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func (m *CondRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
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

func (m *CondRewriter) getInstrumentCallName(callName string) (string, string) {
	switch callName {
	case "Wait":
		return monitor.BEFORE_COND_VAR_WAIT, monitor.AFTER_COND_VAR_WAIT
	case "Signal":
		return monitor.BEFORE_COND_VAR_SIGNAL, monitor.AFTER_COND_VAR_SIGNAL
	case "Broadcast":
		return monitor.BEFORE_COND_VAR_BROADCAST, monitor.AFTER_COND_VAR_BROADCAST
	}
	return "", ""
}

func (m *CondRewriter) isCondMethod(funcName string) bool {
	switch funcName {
	case "Wait", "Signal", "Broadcast":
		return true
	default:
		return false
	}
}

func runCondPass(pass *analysis.Pass) (interface{}, error) {
	rewriter := &CondRewriter{
		pass:     pass,
		instrCtx: SharedInstrCtx,
	}
	for _, file := range pass.Files {
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
	}
	return nil, nil
}