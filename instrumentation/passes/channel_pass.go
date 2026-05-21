package passes

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/instrumentation/utils"
	"github.com/focs-lab/gct/monitor"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var ChannelAnalyzer = &analysis.Analyzer{
	Name:     "channel_instrumentation",
	Doc:      "channel instrumentation",
	Run:      runChannelPass,
	Requires: []*analysis.Analyzer{TimeAnalyzer},
}

type ChannelRewriter struct {
	pass           *analysis.Pass
	currFunc       *ast.FuncDecl
	selectChanStmt map[ast.Stmt]bool
	file           *ast.File
	instrCtx       *InstrContext
}

func (rewriter *ChannelRewriter) handleMakeChan(c *astutil.Cursor, call *ast.CallExpr) {
	var capArg ast.Expr

	if len(call.Args) > 1 {
		capArg = call.Args[1]
	} else {
		// If capacity is not specified, it's 0.
		capArg = &ast.BasicLit{Value: "0", Kind: token.INT}
	}

	// Get the channel type expression directly from the `make` call arguments.
	// e.g., for `make(chan struct{}, 1)`, chanTypeExpr is `chan struct{}`.
	chanTypeExpr, ok := call.Args[0].(*ast.ChanType)
	if !ok {
		return
	}
	elemTypeExpr := chanTypeExpr.Value // This is the `struct{}` part.
	opid := rewriter.instrCtx.GetNewOpid()
	opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
	replacedCall := utils.CreateMonitorGenericMakeChanCall(monitor.CHANNEL_CREATE, elemTypeExpr, capArg, opidLit)

	c.Replace(replacedCall)
}

func (rewriter *ChannelRewriter) handleAssignStmt(c *astutil.Cursor, stmt *ast.AssignStmt) bool {
	length := len(stmt.Rhs)

	if length != 1 {
		return false
	}

	rhsExpr := stmt.Rhs[0]

	// Case: x, ok := <- ch
	if recvExpr, ok := isRecvExpr(rhsExpr); ok {
		ch := recvExpr.X

		var isSelect ast.Expr
		isTimer := isTimerChan(rewriter.pass, ch)
		if rewriter.selectChanStmt[stmt] {
			isSelect = ast.NewIdent("true")
		} else {
			isSelect = ast.NewIdent("false")
		}

		opid := rewriter.instrCtx.GetNewOpid()
		opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
		// Replace `<-ch` with `monitor.ChannelRecv(ch, isSelect)`
		channelRecvCall := utils.CreateMonitorSyncObjCall(monitor.CHANNEL_RECV, ch, isSelect, isTimer, opidLit)
		stmt.Rhs[0] = channelRecvCall

		// If the original assignment was `x := <-ch`, it now becomes `x, _ := monitor.ChannelRecv(...)`
		// to correctly handle the two return values from the wrapper.
		if len(stmt.Lhs) == 1 {
			stmt.Lhs = append(stmt.Lhs, ast.NewIdent("_"))
		}

		c.Replace(stmt)
	}

	return true
}

func (rewriter *ChannelRewriter) handleIfStmt(c *astutil.Cursor, stmt *ast.IfStmt) bool {
	initStmt := stmt.Init

	if assignStmt, ok := initStmt.(*ast.AssignStmt); ok {
		length := len(assignStmt.Rhs)

		if length != 1 {
			return true
		}

		rhsExpr := assignStmt.Rhs[0]
		if recvExpr, ok := isRecvExpr(rhsExpr); ok {
			ch := recvExpr.X

			// InitStmt in if stmt cannot be selected
			isSelect := ast.NewIdent("false")
			isTimer := isTimerChan(rewriter.pass, ch)

			opid := rewriter.instrCtx.GetNewOpid()
			opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
			channelRecvCall := utils.CreateMonitorSyncObjCall(monitor.CHANNEL_RECV, ch, isSelect, isTimer, opidLit)
			assignStmt.Rhs[0] = channelRecvCall

			if len(assignStmt.Lhs) == 1 {
				assignStmt.Lhs = append(assignStmt.Lhs, ast.NewIdent("_"))
			}

			assignStmt.Rhs[0] = channelRecvCall
			return true
		}
	}

	return true
}

func (rewriter *ChannelRewriter) handleSendStmt(c *astutil.Cursor, sendStmt *ast.SendStmt) bool {
	chExpr := sendStmt.Chan
	valExpr := sendStmt.Value

	valType := rewriter.pass.TypesInfo.TypeOf(valExpr) // The tranformed value expression

	var chElemType types.Type // The transformed channel expression
	if chanType, ok := rewriter.pass.TypesInfo.TypeOf(chExpr).Underlying().(*types.Chan); ok {
		chElemType = chanType.Elem()
	}

	// If the channel or value is a function call, evaluate it first
	// and store it in a temporary variable to ensure it's only called once.
	if _, ok := chExpr.(*ast.CallExpr); ok {
		scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]
		varType := rewriter.pass.TypesInfo.TypeOf(chExpr)
		tempChVar := utils.GetFreshIdent(config.TEMP_VAR_PREFIX, scope, varType)

		c.InsertBefore(&ast.AssignStmt{Lhs: []ast.Expr{tempChVar}, Tok: token.DEFINE, Rhs: []ast.Expr{chExpr}})
		chExpr = tempChVar
	}

	if _, ok := valExpr.(*ast.CallExpr); ok {
		scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]
		varType := rewriter.pass.TypesInfo.TypeOf(valExpr)
		tempValVar := utils.GetFreshIdent(config.TEMP_VAR_PREFIX, scope, varType)

		c.InsertBefore(&ast.AssignStmt{Lhs: []ast.Expr{tempValVar}, Tok: token.DEFINE, Rhs: []ast.Expr{valExpr}})
		valExpr = tempValVar
	}

	// If the channel's element type is an interface, and the value being sent
	// is a concrete type, we need to add an explicit type conversion.
	// e.g., `ch <- myStruct` -> `monitor.ChannelSend(ch, T(myStruct), ...)`
	// where T is the interface type.
	if types.IsInterface(chElemType) {
		if !types.IsInterface(valType) && !types.Identical(chElemType, valType) {
			// Wrap the value in a type conversion to the channel's element type.
			typeExpr := utils.TypesExpr(chElemType, rewriter.pass.Pkg, rewriter.file)
			valExpr = &ast.CallExpr{Fun: typeExpr, Args: []ast.Expr{valExpr}}
		}
	}

	var isSelect ast.Expr
	isTimer := isTimerChan(rewriter.pass, chExpr)
	if rewriter.selectChanStmt[sendStmt] {
		isSelect = ast.NewIdent("true")
	} else {
		isSelect = ast.NewIdent("false")
	}

	opid := rewriter.instrCtx.GetNewOpid()
	opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
	replacedCall := utils.CreateMonitorSyncObjCall(monitor.CHANNEL_SEND, chExpr, valExpr, isSelect, isTimer, opidLit)
	replacedStmt := &ast.ExprStmt{X: replacedCall}

	c.Replace(replacedStmt)
	return true
}

func (rewriter *ChannelRewriter) handleExprStmt(c *astutil.Cursor, exprStmt *ast.ExprStmt) bool {
	// InsertBefore/InsertAfter only work when the node is in a list
	if c.Index() < 0 {
		return false
	}

	// Case 1: close(ch)
	if callExpr, ok := isCloseChanExpr(exprStmt.X); ok {
		chVar := callExpr.Args[0]
		opid := rewriter.instrCtx.GetNewOpid()
		opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
		replace := utils.CreateMonitorSyncObjCall(monitor.CHANNEL_CLOSE, chVar, opidLit)
		exprStmt.X = replace
	}

	// Case 2: <- ch
	if recvExpr, ok := isRecvExpr(exprStmt.X); ok {
		chVar := recvExpr.X

		isSelect := ast.NewIdent("false")
		isTimer := isTimerChan(rewriter.pass, chVar)
		if rewriter.selectChanStmt[exprStmt] {
			isSelect.Name = "true"
		}

		opid := rewriter.instrCtx.GetNewOpid()
		opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
		channelRecvCall := utils.CreateMonitorSyncObjCall(monitor.CHANNEL_RECV, chVar, isSelect, isTimer, opidLit)
		c.Replace(&ast.ExprStmt{X: channelRecvCall})
	}

	return true
}

func (rewriter *ChannelRewriter) handleReturnStmt(c *astutil.Cursor, stmt *ast.ReturnStmt) bool {
	// Only handle single return value for simplicity.
	if len(stmt.Results) != 1 {
		return true
	}

	retExpr := stmt.Results[0]

	// Case: return <-ch
	if recvExpr, ok := isRecvExpr(retExpr); ok {
		ch := recvExpr.X
		// `return <-ch` cannot be part of a select statement
		isSelect := ast.NewIdent("false")
		isTimer := isTimerChan(rewriter.pass, ch)
		opid := rewriter.instrCtx.GetNewOpid()
		opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
		channelRecvCall := utils.CreateMonitorSyncObjCall(monitor.CHANNEL_RECV, ch, isSelect, isTimer, opidLit)

		// Create a temporary variable to store the first return value from ChannelRecv
		scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]
		varType := rewriter.pass.TypesInfo.TypeOf(retExpr)
		tempVar := utils.GetFreshIdent(config.TEMP_VAR_PREFIX, scope, varType)

		// Replace `return <-ch` with:
		// temp_1, _ := monitor.ChannelRecv(...)
		// return temp_1
		assignStmt := &ast.AssignStmt{
			Lhs: []ast.Expr{tempVar, ast.NewIdent("_")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{channelRecvCall},
		}
		c.Replace(assignStmt)
		c.InsertAfter(&ast.ReturnStmt{Results: []ast.Expr{tempVar}})
		return true
	}

	return true
}

func (rewriter *ChannelRewriter) handleRangeStmt(c *astutil.Cursor, rangeStmt *ast.RangeStmt) bool {
	// this function rewrites `for msg := range ch {}` into
	// `for {msg, ok := ChannelRecv(); if !ok {break} }`

	if _, ok := rewriter.pass.TypesInfo.TypeOf(rangeStmt.X).Underlying().(*types.Chan); !ok {
		return true
	}

	chExpr := rangeStmt.X

	// The receive cannot be part of a select statement
	isSelect := ast.NewIdent("false")
	isTimer := isTimerChan(rewriter.pass, chExpr)
	opid := rewriter.instrCtx.GetNewOpid()
	opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
	channelRecvCall := utils.CreateMonitorSyncObjCall(monitor.CHANNEL_RECV, chExpr, isSelect, isTimer, opidLit)

	// Create the `v, ok := monitor.ChannelRecv(ch, false)` statement
	scope := rewriter.pass.TypesInfo.Scopes[rewriter.currFunc.Type]
	okIdent := utils.GetFreshIdent(config.TEMP_VAR_PREFIX, scope, types.Typ[types.Bool])
	assignStmt := &ast.AssignStmt{
		Tok: token.DEFINE, // always :=, because ok Ident is always new
		Rhs: []ast.Expr{channelRecvCall},
	}

	if rangeStmt.Key != nil {
		assignStmt.Lhs = []ast.Expr{rangeStmt.Key, okIdent}
	} else {
		// for range ch { ... } -> _, ok := ...
		assignStmt.Lhs = []ast.Expr{ast.NewIdent("_"), okIdent}
	}

	// Create the `if !ok { break }` statement
	ifStmt := &ast.IfStmt{
		Cond: &ast.UnaryExpr{
			Op: token.NOT,
			X:  okIdent,
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}},
		},
	}

	// Prepend the assignment and the check to the original loop body
	newBody := &ast.BlockStmt{
		List: append([]ast.Stmt{assignStmt, ifStmt}, rangeStmt.Body.List...),
	}

	// Replace the `for range` with `for { ... }`
	c.Replace(&ast.ForStmt{Body: newBody})
	return false // We replaced the node, so the cursor should not proceed further down this branch
}

func (rewriter *ChannelRewriter) handleDeferStmt(c *astutil.Cursor, deferStmt *ast.DeferStmt) bool {
	if callExpr, ok := isCloseChanExpr(deferStmt.Call); ok {
		chVar := callExpr.Args[0]
		opid := rewriter.instrCtx.GetNewOpid()
		opidLit := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(opid, 10)}
		replace := utils.CreateMonitorSyncObjCall(monitor.CHANNEL_CLOSE, chVar, opidLit)
		deferStmt.Call = replace
	}
	return true
}

func (rewriter *ChannelRewriter) PreHandleASTNode(c *astutil.Cursor) bool {
	n := c.Node()
	if n == nil {
		return false
	}

	switch stmt := n.(type) {
	case *ast.FuncDecl:
		rewriter.currFunc = stmt
		return true

	case *ast.AssignStmt:
		return rewriter.handleAssignStmt(c, stmt)

	case *ast.SendStmt:
		return rewriter.handleSendStmt(c, stmt)

	case *ast.ExprStmt:
		return rewriter.handleExprStmt(c, stmt)

	case *ast.ReturnStmt:
		return rewriter.handleReturnStmt(c, stmt)

	case *ast.IfStmt:
		return rewriter.handleIfStmt(c, stmt)

	case *ast.RangeStmt:
		return rewriter.handleRangeStmt(c, stmt)

	case *ast.DeferStmt:
		return rewriter.handleDeferStmt(c, stmt)

	case *ast.CallExpr:
		if callExpr, ok := isMakeChanExpr(stmt); ok {
			rewriter.handleMakeChan(c, callExpr)
		}
		return true

	case *ast.SwitchStmt:
		if tag, ok := stmt.Tag.(*ast.Ident); ok && strings.HasPrefix(tag.Name, config.SELECT_TAG_PREFIX) {
			for _, s := range stmt.Body.List {
				if cc, ok := s.(*ast.CaseClause); ok {
					if cc.List != nil && len(cc.Body) > 1 {
						// 1st stmt is AfterSelect
						// 2nd stmt is the original case body, i.e. channel send/recv
						rewriter.selectChanStmt[cc.Body[1]] = true
					}
				}
			}
		}
		return true

	default:
		return true
	}
}

func (rewriter *ChannelRewriter) PostHandleASTNode(c *astutil.Cursor) bool {
	return true
}

func runChannelPass(pass *analysis.Pass) (interface{}, error) {
	files := pass.Files
	rewriter := &ChannelRewriter{
		pass:           pass,
		selectChanStmt: make(map[ast.Stmt]bool),
		currFunc:       nil,
		file:           nil,
		instrCtx:       SharedInstrCtx,
	}

	for _, file := range files {
		rewriter.file = file
		astutil.Apply(file, rewriter.PreHandleASTNode, rewriter.PostHandleASTNode)
	}
	return nil, nil
}

func isCloseChanExpr(expr ast.Expr) (*ast.CallExpr, bool) {
	if call, ok := expr.(*ast.CallExpr); ok {
		if fun, ok := call.Fun.(*ast.Ident); ok && fun.Name == "close" && len(call.Args) == 1 {
			return call, true
		}
	}
	return nil, false
}

// Check if the channel is from a time.Ticker or time.Timer.
// This is a heuristic. We check if the expression is a selector expression
// like `t.C` where `t` is of type `*time.Ticker` or `*time.Timer`.
func isTimerChan(pass *analysis.Pass, chExpr ast.Expr) ast.Expr {
	if selExpr, ok := chExpr.(*ast.SelectorExpr); ok {
		if selExpr.Sel.Name == "C" {
			xType := pass.TypesInfo.TypeOf(selExpr.X)
			if named, ok := xType.(*types.Pointer); ok {
				xType = named.Elem()
			}
			if named, ok := xType.(*types.Named); ok {
				typeName := named.Obj()
				if typeName != nil && typeName.Pkg() != nil {
					pkgPath := typeName.Pkg().Path()
					if (pkgPath == "time" || pkgPath == config.SYNC_SHADOW_IMPORT_PATH) && (typeName.Name() == "Ticker" || typeName.Name() == "Timer") {
						return ast.NewIdent("true")
					}
				}
			}
		}
	}

	// Fallback: check the type of the expression. If it's <-chan time.Time,
	// it's likely a timer channel that was assigned to a variable.
	// if typeOfCh := pass.TypesInfo.TypeOf(chExpr); typeOfCh != nil {
	// 	if ch, ok := typeOfCh.Underlying().(*types.Chan); ok {
	// 		if ch.Dir() == types.RecvOnly && ch.Elem().String() == "time.Time" {
	// 			return ast.NewIdent("true")
	// 		}
	// 	}
	// }
	return ast.NewIdent("false")
}

func isMakeChanExpr(expr ast.Expr) (*ast.CallExpr, bool) {
	if call, ok := expr.(*ast.CallExpr); ok {
		if funIdent, ok := call.Fun.(*ast.Ident); ok && funIdent.Name == "make" {
			if len(call.Args) > 0 {
				if _, ok := call.Args[0].(*ast.ChanType); ok {
					return call, true
				}
			}
		}
	}
	return nil, false
}

func isRecvExpr(expr ast.Expr) (*ast.UnaryExpr, bool) {
	if recv, ok := expr.(*ast.UnaryExpr); ok && recv.Op == token.ARROW {
		return recv, true
	}
	return nil, false
}
