package passes

import (
	"fmt"
	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/instrumentation/utils"
	"github.com/focs-lab/gct/monitor"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var GoroutineAnalyzer = &analysis.Analyzer{
	Name:     "goroutine_instrument",
	Doc:      "goroutine instrumented code to disk",
	Run:      runGoroutinePass,
	Requires: []*analysis.Analyzer{TestAnalyzer},
}

type GoroutineRewriter struct {
	pass     *analysis.Pass
	currFunc *ast.FuncDecl
	instrCtx *InstrContext
	file     *ast.File
}

func (rewriter *GoroutineRewriter) handleGoStmt(c *astutil.Cursor, goStmt *ast.GoStmt) bool {
	origCall := goStmt.Call
	if origCall == nil {
		return true
	}

	beforeCall := utils.CreateMonitorNonSyncPrimCall(monitor.BEFORE_GOROUTINE_CREATION)
	afterCreate := utils.CreateMonitorNonSyncPrimCall(monitor.AFTER_GOROUTINE_CREATION_CREATOR)

	scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]

	parentType := types.Typ[types.Uint64]
	parentIdent := utils.GetFreshIdent(config.PARENT_GOID_PREFIX, scope, parentType)

	c.InsertBefore(&ast.AssignStmt{
		Lhs: []ast.Expr{parentIdent},
		Rhs: []ast.Expr{beforeCall},
		Tok: token.DEFINE,
	})

	c.InsertAfter(&ast.ExprStmt{X: afterCreate})

	afterCall := utils.CreateMonitorNonSyncPrimCall(
		monitor.AFTER_GOROUTINE_CREATION,
		parentIdent,
	)
	beforeEnd := utils.CreateMonitorNonSyncPrimCall(
		monitor.BEFORE_GOROUTINE_END,
	)

	// Eager eval input args and use temp args to replace
	callArgs := make([]ast.Expr, len(origCall.Args))

	for i, arg := range origCall.Args {
		// 1. Nil: never touch
		if ident, ok := arg.(*ast.Ident); ok && ident.Name == "nil" {
			callArgs[i] = arg
			continue
		}

		typ := rewriter.pass.TypesInfo.TypeOf(arg)

		// Constant: never tempify
		if rewriter.pass.TypesInfo.Types[arg].Value != nil {
			callArgs[i] = arg
			continue
		}

		tmpIdent := utils.GetFreshIdent(fmt.Sprintf("_cct_arg_%d", i), scope, typ)

		c.InsertBefore(&ast.AssignStmt{
			Lhs: []ast.Expr{tmpIdent},
			Rhs: []ast.Expr{arg},
			Tok: token.DEFINE,
		})

		callArgs[i] = tmpIdent
	}

	newCall := &ast.CallExpr{
		Fun:      origCall.Fun,
		Args:     callArgs,
		Ellipsis: origCall.Ellipsis,
	}

	wrapper := &ast.FuncLit{
		Type: &ast.FuncType{},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ExprStmt{X: afterCall},
				&ast.DeferStmt{Call: beforeEnd},
				&ast.ExprStmt{X: newCall},
			},
		},
	}

	goStmt.Call = &ast.CallExpr{
		Fun:  wrapper,
		Args: []ast.Expr{},
	}

	return true
}

func (rewriter *GoroutineRewriter) isAfterFunc(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if obj, ok := rewriter.pass.TypesInfo.Uses[sel.Sel]; !ok || obj.Pkg() == nil || obj.Pkg().Path() != "time" || obj.Name() != "AfterFunc" {
			return false // Not time.AfterFunc
		}
	} else {
		return false // Not a selector expression, so not time.AfterFunc
	}

	return true
}

func (rewriter *GoroutineRewriter) handleAfterFunc(c *astutil.Cursor, call *ast.CallExpr) bool {
	/*
		before: time.AfterFunc(d, f)
		after:  monitor.TimeAfterFunc(d, f)
	*/

	call.Fun = &ast.SelectorExpr{
		X:   ast.NewIdent(config.MONITOR_IMPORT_NAME),
		Sel: ast.NewIdent(monitor.TIME_AFTER_FUNC),
	}

	return true
}

func (rewriter *GoroutineRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	n := c.Node()
	if n == nil {
		return false
	}

	switch stmt := n.(type) {
	case *ast.GoStmt:
		return rewriter.handleGoStmt(c, stmt)

	case *ast.FuncDecl:
		rewriter.currFunc = stmt
		return true

	case *ast.CallExpr:
		if rewriter.isAfterFunc(stmt) {
			return rewriter.handleAfterFunc(c, stmt)
		}
		return true

	default:
		return true
	}
}

func (rewriter *GoroutineRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func runGoroutinePass(pass *analysis.Pass) (interface{}, error) {
	files := pass.Files
	rewriter := &GoroutineRewriter{
		pass:     pass,
		currFunc: nil,
		instrCtx: SharedInstrCtx,
		file:     nil,
	}
	for _, file := range files {
		rewriter.file = file
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
		// utils.PersistInstrumentations(pass, file)
	}
	return nil, nil
}
