package passes

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/instrumentation/utils"
	"github.com/focs-lab/gct/monitor"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var SelectAnalyzer = &analysis.Analyzer{
	Name:     "select_instrumentation",
	Doc:      "select_instrumentation",
	Run:      runSelectPass,
	Requires: []*analysis.Analyzer{RWMutexAnalyzer},
}

type SelectRewriter struct {
	pass     *analysis.Pass
	currFunc *ast.FuncDecl
	counter  int
	instrCtx *InstrContext
}

func (rewriter *SelectRewriter) handleChanExpr(chExpr ast.Expr, exprToIdent map[ast.Expr]ast.Expr, idx_val string) (ast.Expr, ast.Expr) {
	scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]
	varType := rewriter.pass.TypesInfo.TypeOf(chExpr)
	if _, ok := chExpr.(*ast.CallExpr); ok {
		chIdent := utils.GetFreshIdent(config.CHANNEL_PREFIX, scope, varType)
		exprToIdent[chExpr] = chIdent
	} else {
		exprToIdent[chExpr] = chExpr
	}

	return exprToIdent[chExpr], &ast.BasicLit{Kind: token.INT, Value: idx_val}
}

func (rewriter *SelectRewriter) handleSelectStmt(c *astutil.Cursor, stmt *ast.SelectStmt) bool {
	scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]
	switchTag := utils.GetFreshIdent(config.SELECT_TAG_PREFIX, scope, types.Typ[types.Int])
	bodyList := stmt.Body.List

	exprToIdent := make(map[ast.Expr]ast.Expr)

	sendChExprs := make([]ast.Expr, 0)
	recvChExprs := make([]ast.Expr, 0)

	sendChsAreTimer := make([]ast.Expr, 0)
	recvChsAreTimer := make([]ast.Expr, 0)

	sendChIdxs := make([]ast.Expr, 0)
	recvChIdxs := make([]ast.Expr, 0)

	switchStmt := &ast.SwitchStmt{
		Tag: switchTag,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{},
		},
	}

	hasDefault := false
	allCasesReturn := true

	opid := rewriter.instrCtx.GetNewOpid()
	opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
	var afterCallStmt *ast.ExprStmt

	// Use a separate counter for non-default cases to ensure indices start from 0.
	caseCounter := 0

	if len(bodyList) == 0 {
		allCasesReturn = false
	}

	for _, stmt := range bodyList {
		var switchCaseTag []ast.Expr

		clause, ok := stmt.(*ast.CommClause)
		if !ok {
			panic("A stmt inside select stmt is not *ast.CommClause")
		}

		// Check if the case body ends with a return statement
		if len(clause.Body) == 0 || !isTerminatingStmt(clause.Body[len(clause.Body)-1]) {
			allCasesReturn = false
		}

		if utils.IsDefaultCaseInSelect(clause) {
			// default case
			switchStmt.Body.List = append(
				switchStmt.Body.List,
				&ast.CaseClause{
					List: nil, // default in select -> default in switch. AfterCall will be added later.
					Body: clause.Body,
				},
			)
			hasDefault = true
			continue
		}

		// This is a non-default case.
		idx_val := strconv.Itoa(caseCounter)
		switchCaseTag = []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: idx_val}}

		var mappedClauseStmt ast.Stmt

		switch caseStmt := clause.Comm.(type) {
		case *ast.SendStmt:
			// utils.RemovePos(caseStmt.Chan)
			chExpr := caseStmt.Chan
			sendChExpr, sendChIdx := rewriter.handleChanExpr(chExpr, exprToIdent, idx_val)
			sendChExprs = append(sendChExprs, sendChExpr)
			sendChsAreTimer = append(sendChsAreTimer, isTimerChan(rewriter.pass, chExpr))
			sendChIdxs = append(sendChIdxs, sendChIdx)
			mappedClauseStmt = &ast.SendStmt{
				Chan:  exprToIdent[chExpr],
				Value: caseStmt.Value,
			}

		case *ast.AssignStmt:
			// x = <- ch
			if len(caseStmt.Rhs) > 1 {
				panic("One case of select stmt has more than one RHS")
			}
			rhsExpr := caseStmt.Rhs[0]
			recvExpr, ok := rhsExpr.(*ast.UnaryExpr)
			if !ok || recvExpr.Op != token.ARROW {
				panic("One case of select has non recv expr on RHS")
			}
			// utils.RemovePos(recvExpr.X)
			chExpr := recvExpr.X
			recvChExpr, recvChIdx := rewriter.handleChanExpr(chExpr, exprToIdent, idx_val)
			recvChExprs = append(recvChExprs, recvChExpr)
			recvChsAreTimer = append(recvChsAreTimer, isTimerChan(rewriter.pass, chExpr))
			recvChIdxs = append(recvChIdxs, recvChIdx)
			mappedClauseStmt = &ast.AssignStmt{
				Lhs: caseStmt.Lhs,
				Tok: caseStmt.Tok,
				Rhs: []ast.Expr{&ast.UnaryExpr{
					Op: token.ARROW,
					X:  exprToIdent[chExpr],
				}},
			}

		case *ast.ExprStmt:
			// <- ch
			recvExpr, ok := caseStmt.X.(*ast.UnaryExpr)
			if !ok || recvExpr.Op != token.ARROW {
				panic("One case of select is neither send nor recv")
			}
			// utils.RemovePos(recvExpr.X)
			chExpr := recvExpr.X
			recvChExpr, recvChIdx := rewriter.handleChanExpr(chExpr, exprToIdent, idx_val)
			recvChExprs = append(recvChExprs, recvChExpr)
			recvChsAreTimer = append(recvChsAreTimer, isTimerChan(rewriter.pass, chExpr))
			recvChIdxs = append(recvChIdxs, recvChIdx)
			mappedClauseStmt = &ast.ExprStmt{
				X: &ast.UnaryExpr{
					X:  exprToIdent[chExpr],
					Op: recvExpr.Op,
				},
			}
		}

		switchStmt.Body.List = append(
			switchStmt.Body.List,
			&ast.CaseClause{
				List: switchCaseTag,
				Body: append([]ast.Stmt{mappedClauseStmt}, clause.Body...),
			},
		)

		// Increment the counter only for non-default cases.
		caseCounter++
	}

	// Now create the BeforeSelect and AfterSelect calls with the complete timer info
	beforeCall := utils.CreateMonitorSelectCall(monitor.BEFORE_SELECT, sendChExprs, recvChExprs, sendChsAreTimer,
		recvChsAreTimer, sendChIdxs, recvChIdxs, hasDefault, opidLit)
	afterCall := utils.CreateMonitorSelectCall(monitor.AFTER_SELECT, sendChExprs, recvChExprs, sendChsAreTimer,
		recvChsAreTimer, sendChIdxs, recvChIdxs, hasDefault, &ast.Ident{Name: switchTag.Name}, opidLit)
	afterCallStmt = &ast.ExprStmt{X: afterCall}

	// Inject the afterCallStmt into each case clause's body as the first statement.
	for _, stmt := range switchStmt.Body.List {
		if caseClause, ok := stmt.(*ast.CaseClause); ok {
			caseClause.Body = append([]ast.Stmt{afterCallStmt}, caseClause.Body...)
		}
	}

	// Manually instrument the new switch statement to handle nested selects
	astutil.Apply(switchStmt, rewriter.PreHandleASTNode, nil)

	for expr, ident := range exprToIdent { // This loop now handles only CallExpr cases
		if expr != ident { // ch_i := Expr
			c.InsertBefore(&ast.AssignStmt{Lhs: []ast.Expr{ident}, Tok: token.DEFINE, Rhs: []ast.Expr{expr}})
		}
	}
	c.InsertBefore(&ast.AssignStmt{Lhs: []ast.Expr{switchTag}, Tok: token.DEFINE, Rhs: []ast.Expr{beforeCall}})

	if allCasesReturn {
		// If all original select cases had a return, the compiler expects the block to terminate.
		// Add a panic to the end to satisfy the compiler. This should be unreachable.
		panicCall := &ast.CallExpr{
			Fun:  ast.NewIdent("panic"),
			Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"Go-CCT instrumentation error: unexpected fallthrough in select-to-switch"`}},
		}
		c.Replace(switchStmt)
		c.InsertAfter(&ast.ExprStmt{X: panicCall})
	} else {
		c.Replace(switchStmt)
	}

	return false
}

func isTerminatingStmt(stmt ast.Stmt) bool {
	_, isReturn := stmt.(*ast.ReturnStmt)
	return isReturn
}

func (rewriter *SelectRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	n := c.Node()
	if n == nil {
		return false
	}

	switch stmt := n.(type) {
	case *ast.SelectStmt:
		return rewriter.handleSelectStmt(c, stmt)

	case *ast.FuncDecl:
		rewriter.currFunc = stmt
		return true

	default:
		return true
	}
}

func (rewriter *SelectRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func runSelectPass(pass *analysis.Pass) (interface{}, error) {
	files := pass.Files
	rewriter := &SelectRewriter{pass: pass, instrCtx: SharedInstrCtx}
	for _, file := range files {
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
		// utils.PersistInstrumentations(pass, file)
	}
	return nil, nil
}
