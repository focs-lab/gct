package passes

import (
	"go/ast"

	"github.com/focs-lab/gct/instrumentation/utils"
	"github.com/focs-lab/gct/monitor"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var LoopAnalyzer = &analysis.Analyzer{
	Name: "loop_instrumentation",
	Doc:  "instrument infinite for loops to yield to the scheduler",
	Run:  runLoopPass,
	Requires: []*analysis.Analyzer{
		ChannelAnalyzer,
	},
}

type LoopRewriter struct {
	pass     *analysis.Pass
	instrCtx *InstrContext
}

func (rewriter *LoopRewriter) handleForStmt(c *astutil.Cursor, forStmt *ast.ForStmt) bool {
	// Instrument `for {}` loops that have no condition.
	if forStmt.Cond == nil {
		onIterationCall := utils.CreateMonitorNonSyncPrimCall(monitor.ON_EACH_LOOP_ITERATION)
		onIterationStmt := &ast.ExprStmt{X: onIterationCall}

		// Append the call to the end of the loop body.
		forStmt.Body.List = append(forStmt.Body.List, onIterationStmt)
	}
	return true
}

func runLoopPass(pass *analysis.Pass) (interface{}, error) {
	rewriter := &LoopRewriter{pass: pass, instrCtx: SharedInstrCtx}
	for _, file := range pass.Files {
		astutil.Apply(file, func(c *astutil.Cursor) bool {
			if n, ok := c.Node().(*ast.ForStmt); ok {
				return rewriter.handleForStmt(c, n)
			}
			return true
		}, nil)
	}
	return nil, nil
}
