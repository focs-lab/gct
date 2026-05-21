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

var ContextAnalyzer = &analysis.Analyzer{
	Name: "context_instr",
	Doc:  "instrument context creation and cancellation",
	Run:  runContextPass,
	Requires: []*analysis.Analyzer{
		GoroutineAnalyzer,
	},
}

type ContextRewriter struct {
	pass        *analysis.Pass
	instrCtx    *InstrContext
	cancelFuncs map[types.Object]bool
}

func runContextPass(pass *analysis.Pass) (interface{}, error) {
	rewriter := &ContextRewriter{
		pass:        pass,
		instrCtx:    SharedInstrCtx,
		cancelFuncs: make(map[types.Object]bool),
	}
	for _, file := range pass.Files {
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
	}
	return nil, nil
}

func (r *ContextRewriter) isContextCreationFunc(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	// Check if the receiver is the 'context' package
	if x, ok := sel.X.(*ast.Ident); ok {
		if obj, ok := r.pass.TypesInfo.Uses[x]; ok {
			if pkg, ok := obj.(*types.PkgName); ok && pkg.Imported().Path() == "context" {
				switch sel.Sel.Name {
				case "Background", "TODO", "WithValue", "WithCancel", "WithDeadline", "WithTimeout":
					return sel.Sel.Name, true
				}
			}
		}
	}
	return "", false
}

func (r *ContextRewriter) isCancelFunc(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	if obj, ok := r.pass.TypesInfo.Uses[ident]; ok {
		return r.cancelFuncs[obj]
	}

	return false
}

func (r *ContextRewriter) handleAssignStmt(c *astutil.Cursor, stmt *ast.AssignStmt) bool {
	if len(stmt.Rhs) != 1 {
		return true
	}

	call, ok := stmt.Rhs[0].(*ast.CallExpr)
	if !ok {
		return true
	}

	funcName, isContextCreation := r.isContextCreationFunc(call)
	if !isContextCreation {
		return true
	}

	// Track the cancel function if it's returned
	switch funcName {
	case "WithCancel", "WithDeadline", "WithTimeout":
		if len(stmt.Lhs) == 2 {
			// This is `ctx, cancel := ...`
			if cancelIdent, ok := stmt.Lhs[1].(*ast.Ident); ok {
				if obj := r.pass.TypesInfo.Defs[cancelIdent]; obj != nil {
					r.cancelFuncs[obj] = true
				}
			}
		}
	}

	// Instrument the creation call
	r.instrumentContextCreation(c, call, funcName, stmt.Lhs[0])

	return true
}

func (r *ContextRewriter) handleExprStmt(c *astutil.Cursor, stmt *ast.ExprStmt) bool {
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return true
	}

	if r.isCancelFunc(call) {
		opId := r.instrCtx.GetNewOpid()
		opIdLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opId, 10)}

		beforeCall := utils.CreateMonitorNonSyncPrimCall(monitor.BEFORE_CONTEXT_CANCEL,  opIdLit)
		afterCall := utils.CreateMonitorNonSyncPrimCall(monitor.AFTER_CONTEXT_CANCEL, opIdLit)

		c.InsertBefore(&ast.ExprStmt{X: beforeCall})
		c.InsertAfter(&ast.ExprStmt{X: afterCall})
	}

	return true
}

func (r *ContextRewriter) instrumentContextCreation(c *astutil.Cursor, call *ast.CallExpr, funcName string, ctxExpr ast.Expr) {
	newCallName := monitor.AFTER_CONTEXT_CREATION
	ctxTypeLit := &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(funcName)}

	newCall := utils.CreateMonitorSyncObjCall(newCallName, ctxExpr, ctxTypeLit)
	c.InsertAfter(&ast.ExprStmt{X: newCall})
}

func (r *ContextRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	n := c.Node()
	if n == nil {
		return true
	}

	switch stmt := n.(type) {
	case *ast.AssignStmt:
		return r.handleAssignStmt(c, stmt)
	case *ast.ExprStmt:
		return r.handleExprStmt(c, stmt)
	}

	return true
}

func (r *ContextRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}