package instrumentation

import (
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/analysis"
)

type Rewriter interface {
	PreHandleASTNode(c *astutil.Cursor) bool
	PostHandleASTNode(c *astutil.Cursor) bool
}

type ASTPatternHandler func(pass *analysis.Pass, c *astutil.Cursor) bool