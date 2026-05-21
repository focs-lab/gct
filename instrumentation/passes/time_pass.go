package passes

import (
	"go/ast"
	"go/types"

	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/monitor"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var TimeAnalyzer = &analysis.Analyzer{
	Name: "time_instrumentation",
	Doc:  "instrument time-related operations",
	Run:  runTimePass,
	Requires: []*analysis.Analyzer{
		SelectAnalyzer,
	},
}

type TimeRewriter struct {
	pass           *analysis.Pass
	instrCtx       *InstrContext
	usesMonitor    bool
	usesSyncShadow bool
}

func (r *TimeRewriter) isTimeAfter(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Match `time.After(d time.Duration)`, which returns a channel
	// Note that time.Time also has an After() method, which returns a bool
	if ident, ok := sel.X.(*ast.Ident); ok {
		if obj, ok := r.pass.TypesInfo.Uses[ident]; ok {
			if pkg, ok := obj.(*types.PkgName); ok && pkg.Imported().Path() == "time" {
				return sel.Sel.Name == "After"
			}
		}
	}

	return false
}

func (r *TimeRewriter) isNewTimer(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Use types info to be precise
	if obj, ok := r.pass.TypesInfo.Uses[sel.Sel]; ok && obj != nil {
		if pkg := obj.Pkg(); pkg != nil && pkg.Path() == "time" && obj.Name() == "NewTimer" {
			return true
		}
	}

	return false
}

func (r *TimeRewriter) isNewTicker(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Use types info to be precise
	if obj, ok := r.pass.TypesInfo.Uses[sel.Sel]; ok && obj != nil {
		if pkg := obj.Pkg(); pkg != nil && pkg.Path() == "time" && obj.Name() == "NewTicker" {
			return true
		}
	}

	return false
}

func (r *TimeRewriter) handleTimeAfter(c *astutil.Cursor, call *ast.CallExpr) bool {
	// Replace time.After with monitor.TimeAfter
	call.Fun = &ast.SelectorExpr{
		X:   ast.NewIdent(config.MONITOR_IMPORT_NAME),
		Sel: ast.NewIdent(monitor.TIME_AFTER),
	}
	r.usesMonitor = true
	return true
}

func (r *TimeRewriter) handleNewTimer(c *astutil.Cursor, call *ast.CallExpr) bool {
	// Replace time.NewTimer with monitor.NewTimer
	call.Fun = &ast.SelectorExpr{
		X:   ast.NewIdent(config.SYNC_SHADOW_IMPORT_NAME),
		Sel: ast.NewIdent(monitor.NEW_TIMER),
	}
	r.usesSyncShadow = true
	return true
}

func (r *TimeRewriter) handleNewTicker(c *astutil.Cursor, call *ast.CallExpr) bool {
	// Replace time.NewTimer with monitor.NewTimer
	call.Fun = &ast.SelectorExpr{
		X:   ast.NewIdent(config.SYNC_SHADOW_IMPORT_NAME),
		Sel: ast.NewIdent(monitor.NEW_TICKER),
	}
	r.usesSyncShadow = true
	return true
}

func (r *TimeRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	if call, ok := c.Node().(*ast.CallExpr); ok {
		if r.isTimeAfter(call) {
			return r.handleTimeAfter(c, call)
		}
		if r.isNewTimer(call) {
			return r.handleNewTimer(c, call)
		}
		if r.isNewTicker(call) {
			return r.handleNewTicker(c, call)
		}
	}

	return true
}

func (r *TimeRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	if sel, ok := c.Node().(*ast.SelectorExpr); ok {
		if r.replaceTimerType(sel) {
			r.usesSyncShadow = true
		}
	}
	return true
}

func (r *TimeRewriter) replaceTimerType(sel *ast.SelectorExpr) bool {
	// Check if it's a time.Timer or time.Ticker
	if id, ok := sel.X.(*ast.Ident); ok {
		if obj := r.pass.TypesInfo.Uses[id]; obj != nil {
			// Check if the identifier is a package name
			if pkg, ok := obj.(*types.PkgName); ok {
				if pkg.Imported().Path() == "time" {
					if sel.Sel.Name == "Timer" || sel.Sel.Name == "Ticker" {
						// Replace `time` with `sync_shadow_go_cct`
						sel.X = ast.NewIdent(config.SYNC_SHADOW_IMPORT_NAME)
						return true
					}
				}
			}
		}
	}
	return false
}

func runTimePass(pass *analysis.Pass) (interface{}, error) {
	rewriter := &TimeRewriter{pass: pass, instrCtx: SharedInstrCtx}
	for _, file := range pass.Files {
		rewriter.usesMonitor = false
		rewriter.usesSyncShadow = false
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)

		if rewriter.usesMonitor {
			astutil.AddNamedImport(rewriter.pass.Fset, file, config.MONITOR_IMPORT_NAME, config.MONITOR_IMPORT_PATH)
		}
		if rewriter.usesSyncShadow {
			astutil.AddNamedImport(rewriter.pass.Fset, file, config.SYNC_SHADOW_IMPORT_NAME, config.SYNC_SHADOW_IMPORT_PATH)
		}
	}
	return nil, nil
}
