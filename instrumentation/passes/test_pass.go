package passes

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/instrumentation/utils"
	"github.com/focs-lab/gct/monitor"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var TestAnalyzer = &analysis.Analyzer{
	Name: "test_instrument",
	Doc:  "add init statements for tests",
	Run:  runTestPass,
}

type TestRewriter struct {
	pass     *analysis.Pass
	currFunc *ast.FuncDecl
	instrCtx *InstrContext
}

func (rewriter *TestRewriter) isAssertionCall(call *ast.CallExpr) bool {
	// Returns true only for assertion calls like `assert.Equal(t, ...)`
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver of the selector is a package name
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	obj := rewriter.pass.TypesInfo.Uses[ident]
	pkg, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}

	// Check if the package is a known assertion library
	if pkg.Imported().Path() == monitor.ASSERTION_ASSERT_PKG_PATH || pkg.Imported().Path() == monitor.ASSERTION_REQUIRE_PKG_PATH {
		return true
	}

	return false
}

// Returns true if call is of type:
// t.Run(name, func(t *testing.T) { ... })
func (rewriter *TestRewriter) isTRunCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	if sel.Sel.Name != "Run" {
		return false
	}

	// Check type of sel.X
	tv := rewriter.pass.TypesInfo.TypeOf(sel.X)
	if tv != nil {
		// The type should be *testing.T
		if ptr, ok := tv.(*types.Pointer); ok {
			if named, ok := ptr.Elem().(*types.Named); ok {
				if obj := named.Obj(); obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "testing" && obj.Name() == "T" {
					return true
				}
			}
		}
	}
	return false
}

func (rewriter *TestRewriter) hasTParallelCall(fn *ast.FuncLit) bool {
	if fn.Body == nil {
		return false
	}
	hasParallel := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				// Heuristic check for t.Parallel()
				// A more robust check would use type information to confirm 't' is *testing.T
				if sel.Sel.Name == "Parallel" {
					hasParallel = true
					return false // Stop inspecting
				}
			}
		}
		return !hasParallel // Continue if not found
	})
	return hasParallel
}


func (rewriter *TestRewriter) handleExprStmt(c *astutil.Cursor, exprStmt *ast.ExprStmt) bool {
	if call, ok := exprStmt.X.(*ast.CallExpr); ok {
		if rewriter.isAssertionCall(call) {
			beforeAssertionCall := utils.CreateMonitorNonSyncPrimCall(monitor.BEFORE_ASSERTION)
			beforeAssertionStmt := &ast.ExprStmt{X: beforeAssertionCall}

			c.InsertBefore(beforeAssertionStmt)
		} else if rewriter.isTRunCall(call) {
			if len(call.Args) != 2 {
				return true // t.Run should have 2 arguments
			}
			fn, ok := call.Args[1].(*ast.FuncLit)
			if !ok {
				return true // The second argument should be a function literal
			}

			if rewriter.hasTParallelCall(fn) {
				// ASYNC: t.Run with t.Parallel() -> Use standard goroutine creation hooks
				scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]
				parentGoidIdent := utils.GetFreshIdent(config.PARENT_GOID_PREFIX, scope, types.Typ[types.Uint64])
				beforeCallExpr := utils.CreateMonitorNonSyncPrimCall(monitor.BEFORE_GOROUTINE_CREATION)
				c.InsertBefore(&ast.AssignStmt{Lhs: []ast.Expr{parentGoidIdent}, Rhs: []ast.Expr{beforeCallExpr}, Tok: token.DEFINE})

				afterCreatorCallExpr := utils.CreateMonitorNonSyncPrimCall(monitor.AFTER_GOROUTINE_CREATION_CREATOR)
				c.InsertAfter(&ast.ExprStmt{X: afterCreatorCallExpr})

				afterCallExpr := utils.CreateMonitorNonSyncPrimCall(monitor.AFTER_GOROUTINE_CREATION, parentGoidIdent)
				beforeEndCallExpr := utils.CreateMonitorNonSyncPrimCall(monitor.BEFORE_GOROUTINE_END)
				fn.Body.List = append([]ast.Stmt{&ast.ExprStmt{X: afterCallExpr}, &ast.DeferStmt{Call: beforeEndCallExpr}}, fn.Body.List...)
			} else {
				// SYNC: t.Run without t.Parallel() -> Use new lightweight TRun hooks
				scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]
				parentGoidIdent := utils.GetFreshIdent(config.PARENT_GOID_PREFIX, scope, types.Typ[types.Uint64])
				beforeCallExpr := utils.CreateMonitorNonSyncPrimCall(monitor.BEFORE_T_RUN)
				c.InsertBefore(&ast.AssignStmt{Lhs: []ast.Expr{parentGoidIdent}, Rhs: []ast.Expr{beforeCallExpr}, Tok: token.DEFINE})

				afterCallExpr := utils.CreateMonitorNonSyncPrimCall(monitor.AFTER_T_RUN, parentGoidIdent)
				beforeEndCallExpr := utils.CreateMonitorNonSyncPrimCall(monitor.BEFORE_GOROUTINE_END)
				fn.Body.List = append([]ast.Stmt{&ast.ExprStmt{X: afterCallExpr}, &ast.DeferStmt{Call: beforeEndCallExpr}}, fn.Body.List...)
			}
		}
	}
	return true
}

func (rewriter *TestRewriter) handleFuncDecl(c *astutil.Cursor, funcDecl *ast.FuncDecl) bool {
	if !utils.IsTestFunction(funcDecl) {
		return false
	}

	afterMainGoStarts := utils.CreateMonitorNonSyncPrimCall(monitor.AFTER_MAIN_GOROUTINE_CREATION)
	beforeMainGoEnds := utils.CreateMonitorNonSyncPrimCall(monitor.BEFORE_MAIN_GOROUTINE_END)

	afterMainGoStartsStmt := &ast.ExprStmt{X: afterMainGoStarts}
	beforeMainGoEndsStmt := &ast.DeferStmt{Call: beforeMainGoEnds}
	funcDecl.Body.List = append([]ast.Stmt{afterMainGoStartsStmt, beforeMainGoEndsStmt}, funcDecl.Body.List...)
	return true
}

func (rewriter *TestRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	n := c.Node()
	if n == nil {
		return false
	}

	switch stmt := n.(type) {
	case *ast.FuncDecl:
		rewriter.currFunc = stmt
		return rewriter.handleFuncDecl(c, stmt)

	case *ast.ExprStmt:
		return rewriter.handleExprStmt(c, stmt)
	default:
		return true
	}
}

func (rewriter *TestRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func runTestPass(pass *analysis.Pass) (interface{}, error) {
	fset := pass.Fset
	files := pass.Files
	testRewriter := &TestRewriter{pass: pass, instrCtx: SharedInstrCtx}

	for _, file := range files {
		if utils.IsTestFile(file, fset) {
			astutil.Apply(file, testRewriter.PreHandleASTNode, testRewriter.PostHandleASTNode)
		}
	}
	return nil, nil
}
