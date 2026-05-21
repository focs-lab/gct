package utils

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"strings"

	"github.com/focs-lab/gct/config"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

func IsIdent(expr ast.Expr) (*ast.Ident, bool) {
	if id, ok := expr.(*ast.Ident); ok {
		return id, true
	}
	return nil, false
}

func GetReceiverType(typeIfo *types.Info, expr ast.Expr) types.Type {
	tv, ok := typeIfo.Types[expr]
	if !ok {
		return nil
	}
	return tv.Type
}

func GetFreshIdent(base string, scope *types.Scope, varType types.Type) *ast.Ident {
	i := 0
	name := fmt.Sprintf("%s%d", base, i)
	for scope.Lookup(name) != nil {
		i++
		name = fmt.Sprintf("%s%d", base, i)
	}

	// insert this new ident into scope,
	// so multiple created idents don't overlap
	obj := types.NewVar(
		token.NoPos,
		nil,
		name,
		varType,
	)
	scope.Insert(obj)

	return ast.NewIdent(name)
}

func IsDefaultCaseInSelect(stmt *ast.CommClause) bool {
	return stmt.Comm == nil
}

func HasImport(file *ast.File, importPath string) bool {
	for _, imp := range file.Imports {
		if imp.Path.Value == `"`+importPath+`"` || imp.Path.Value == "`"+importPath+"`" {
			return true
		}
	}
	return false
}

func AddImport(fset *token.FileSet, file *ast.File, importName, importPath string) {
	if !HasImport(file, importPath) {
		astutil.AddNamedImport(fset, file, importName, importPath)
	}
}

func AddImportForMonitors(fset *token.FileSet, file *ast.File) {
	if !HasImport(file, config.MONITOR_IMPORT_PATH) {
		astutil.AddNamedImport(fset, file, config.MONITOR_IMPORT_NAME, config.MONITOR_IMPORT_PATH)
	}
}

func CreateWrappersCall(methodName string, metaArgs ...ast.Expr) *ast.CallExpr {
	var callExpr *ast.CallExpr
	monitorPkgName := config.SYNC_WRAPPER_IMPORT_NAME

	callExpr = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: monitorPkgName},
			Sel: &ast.Ident{Name: methodName},
		},
		Args: metaArgs,
	}
	return callExpr
}

func CreateSyncShadowObjCall(methodName string, syncObj ast.Expr, metaArgs ...ast.Expr) *ast.CallExpr {
	var callExpr *ast.CallExpr
	monitorPkgName := config.SYNC_SHADOW_IMPORT_NAME
	if syncObj == nil {
		panic("Sync object = nil!")
	}
	callExpr = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: monitorPkgName},
			Sel: &ast.Ident{Name: methodName},
		},
		Args: append([]ast.Expr{syncObj}, metaArgs...),
	}
	return callExpr
}

func CreateMonitorSyncObjCall(methodName string, syncObj ast.Expr, metaArgs ...ast.Expr) *ast.CallExpr {
	var callExpr *ast.CallExpr
	monitorPkgName := config.MONITOR_IMPORT_NAME
	if syncObj == nil {
		panic("Sync object = nil!")
	}
	callExpr = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: monitorPkgName},
			Sel: &ast.Ident{Name: methodName},
		},
		Args: append([]ast.Expr{syncObj}, metaArgs...),
	}
	return callExpr
}

func CreateMonitorGenericMakeChanCall(methodName string, typeExpr ast.Expr, metaArgs ...ast.Expr) *ast.CallExpr {
	var callExpr *ast.CallExpr
	monitorPkgName := config.MONITOR_IMPORT_NAME

	callExpr = &ast.CallExpr{
		Fun: &ast.IndexExpr{
			X: &ast.SelectorExpr{
				X:   &ast.Ident{Name: monitorPkgName},
				Sel: &ast.Ident{Name: methodName},
			},
			Index: typeExpr,
		},
		Args: metaArgs,
	}

	return callExpr
}

func CreateMonitorNonSyncPrimCall(methodName string, args ...ast.Expr) *ast.CallExpr {
	var callExpr *ast.CallExpr
	monitorPkgName := config.MONITOR_IMPORT_NAME

	callExpr = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: monitorPkgName},
			Sel: &ast.Ident{Name: methodName},
		},
		Args: args,
	}
	return callExpr
}

func CreateMonitorSelectCall(methodName string, sendChs, recvChs, isSendTimer, isRecvTimer, sendIdxs, recvIdxs []ast.Expr,
	hasDefault bool, selectedCaseId ...ast.Expr) *ast.CallExpr {
	var callExpr *ast.CallExpr
	monitorPkgName := config.MONITOR_IMPORT_NAME

	sendChExprs := &ast.CompositeLit{
		Type: &ast.ArrayType{
			Len: nil,
			Elt: ast.NewIdent("any"),
		},
		Elts: sendChs,
	}

	recvChExprs := &ast.CompositeLit{
		Type: &ast.ArrayType{
			Len: nil,
			Elt: ast.NewIdent("any"),
		},
		Elts: recvChs,
	}

	isSendTimerExprs := &ast.CompositeLit{
		Type: &ast.ArrayType{
			Len: nil,
			Elt: ast.NewIdent("bool"),
		},
		Elts: isSendTimer,
	}

	isRecvTimerExprs := &ast.CompositeLit{
		Type: &ast.ArrayType{
			Len: nil,
			Elt: ast.NewIdent("bool"),
		},
		Elts: isRecvTimer,
	}

	sendIdxExprs := &ast.CompositeLit{
		Type: &ast.ArrayType{
			Len: nil,
			Elt: ast.NewIdent("int"),
		},
		Elts: sendIdxs,
	}

	recvIdxExprs := &ast.CompositeLit{
		Type: &ast.ArrayType{
			Len: nil,
			Elt: ast.NewIdent("int"),
		},
		Elts: recvIdxs,
	}

	var hasDefaultExpr ast.Expr

	if hasDefault {
		hasDefaultExpr = &ast.Ident{Name: "true"}
	} else {
		hasDefaultExpr = &ast.Ident{Name: "false"}
	}

	args := append([]ast.Expr{sendChExprs, recvChExprs, isSendTimerExprs, isRecvTimerExprs, sendIdxExprs, recvIdxExprs,
		hasDefaultExpr}, selectedCaseId...)

	callExpr = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: monitorPkgName},
			Sel: &ast.Ident{Name: methodName},
		},
		Args: args,
	}
	return callExpr
}

func IsRLockerType(typ types.Type) bool {
	named, ok := typ.(*types.Named)
	if !ok {
		// Check pointer types
		ptr, ok := typ.(*types.Pointer)
		if !ok {
			return false
		}
		named, ok = ptr.Elem().(*types.Named)
		if !ok {
			return false
		}
	}

	if named.Obj().Pkg() == nil {
		return false
	}

	return named.Obj().Pkg().Path() == "sync" && named.Obj().Name() == "rlocker"
}

func IsPointerType(pass *analysis.Pass, expr ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[expr]
	if !ok || tv.Type == nil {
		return false
	}
	_, ok = tv.Type.(*types.Pointer)
	return ok
}

func GetInstrumentedFileName(currFileName string) string {
	suffix := config.OutputSuffix
	if strings.HasSuffix(currFileName, "_test.go") {
		return strings.Replace(currFileName, "_test.go", suffix+"_test.go", -1)
	}
	if strings.HasSuffix(currFileName, ".go") {
		return strings.Replace(currFileName, ".go", suffix+".go", -1)
	}
	return currFileName
}

func CommentOutOriginalFile(pass *analysis.Pass, file *ast.File) bool {
	name := pass.Fset.File(file.Pos()).Name()
	content, err := os.ReadFile(name)
	if err != nil {
		return false
	}

	lines := strings.Split(string(content), "\n")
	var buildTags []string
	var otherLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//go:build") || strings.HasPrefix(trimmed, "// +build") {
			buildTags = append(buildTags, line)
		} else {
			if strings.HasPrefix(trimmed, "package ") {
				otherLines = append(otherLines, line)
			} else {
				otherLines = append(otherLines, "// "+line)
			}
		}
	}

	var newContent []string
	newContent = append(newContent, buildTags...)
	if len(buildTags) > 0 {
		newContent = append(newContent, "")
	}

	newContent = append(newContent, otherLines...)

	if err := os.WriteFile(name, []byte(strings.Join(newContent, "\n")), 0644); err != nil {
		return false
	}
	return true
}

func PersistInstrumentations(pass *analysis.Pass, file *ast.File) {
	var buf bytes.Buffer
	fset := pass.Fset
	pos := file.Pos()
	f := fset.File(pos) // this gives the actual path to the file I'm modifying

	// RemovePos(file)
	printer.Fprint(&buf, fset, file)

	// file name "impl.go" -> "impl_instrumented.go"
	instrFileName := GetInstrumentedFileName(f.Name())
	os.WriteFile(instrFileName, buf.Bytes(), 0644)
}

func IsTestFile(f *ast.File, fset *token.FileSet) bool {
	if f == nil {
		return false
	}

	pos := f.Pos()
	file := fset.File(pos)
	if file == nil {
		return false
	}

	filename := file.Name()
	return strings.HasSuffix(filename, "_test.go")
}

func IsTestFunction(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Recv != nil {
		return false // not a function or is a method
	}

	// 1. Name must start with "Test"
	if !strings.HasPrefix(fn.Name.Name, "Test") {
		return false
	}

	// 2. Must be exported
	if !ast.IsExported(fn.Name.Name) {
		return false
	}

	// 3. Must have exactly one parameter
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}

	// 4. Parameter must be *testing.T
	param := fn.Type.Params.List[0].Type

	star, ok := param.(*ast.StarExpr)
	if !ok {
		return false
	}

	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "testing" {
		return false
	}

	if sel.Sel.Name != "T" {
		return false
	}

	// 5. Must return no values
	if fn.Type.Results != nil && len(fn.Type.Results.List) != 0 {
		return false
	}

	return true
}

func TypeToASTType(typ types.Type, currPkg *types.Package) *ast.FuncType {
	// Unwrap named types (e.g., context.CancelFunc) to get the underlying signature
	if named, ok := typ.(*types.Named); ok {
		typ = named.Underlying()
	}

	sigType, ok := typ.(*types.Signature)
	if !ok {
		panic("input type is not Signature type")
	}

	return &ast.FuncType{
		Params:  tupleToFieldList(sigType.Params(), currPkg),
		Results: tupleToFieldList(sigType.Results(), currPkg),
	}
}

func TypesExpr(t types.Type, currPkg *types.Package, file *ast.File) ast.Expr {
	switch tt := t.(type) {
	case *types.Basic:
		return ast.NewIdent(tt.Name())
	case *types.Pointer:
		return &ast.StarExpr{X: TypesExpr(tt.Elem(), currPkg, file)}
	case *types.Slice:
		return &ast.ArrayType{Elt: TypesExpr(tt.Elem(), currPkg, file)}
	case *types.Array:
		return &ast.ArrayType{
			Len: &ast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprint(tt.Len()),
			},
			Elt: TypesExpr(tt.Elem(), currPkg, file),
		}
	case *types.Map:
		return &ast.MapType{
			Key:   TypesExpr(tt.Key(), currPkg, file),
			Value: TypesExpr(tt.Elem(), currPkg, file),
		}
	case *types.Named:
		pkg := tt.Obj().Pkg()
		if pkg != nil && pkg.Path() != currPkg.Path() {
			pkgName := pkg.Name()

			if file != nil {
				targetPath := fmt.Sprintf(`"%s"`, pkg.Path())
				for _, imp := range file.Imports {
					if imp.Path != nil && imp.Path.Value == targetPath {
						if imp.Name != nil {

							if imp.Name.Name != "_" && imp.Name.Name != "." {
								pkgName = imp.Name.Name
							}
						}
						break
					}
				}
			}

			return &ast.SelectorExpr{
				X:   ast.NewIdent(pkgName),
				Sel: ast.NewIdent(tt.Obj().Name()),
			}
		}
		return ast.NewIdent(tt.Obj().Name())
	case *types.Interface:
		return &ast.InterfaceType{Methods: &ast.FieldList{}}
	case *types.Signature:
		return &ast.InterfaceType{Methods: &ast.FieldList{}}
	case *types.Chan:
		var dir ast.ChanDir
		switch tt.Dir() {
		case types.SendRecv:
			dir = ast.SEND | ast.RECV
		case types.SendOnly:
			dir = ast.SEND
		case types.RecvOnly:
			dir = ast.RECV
		}
		return &ast.ChanType{
			Dir:   dir,
			Value: TypesExpr(tt.Elem(), currPkg, file),
		}
	case *types.Struct:
		fields := make([]*ast.Field, 0, tt.NumFields())
		for i := 0; i < tt.NumFields(); i++ {
			f := tt.Field(i)
			field := &ast.Field{
				Type: TypesExpr(f.Type(), currPkg, file),
			}
			if !f.Anonymous() {
				field.Names = []*ast.Ident{ast.NewIdent(f.Name())}
			}
			fields = append(fields, field)
		}
		return &ast.StructType{Fields: &ast.FieldList{List: fields}}

	case *types.Alias:
	    // If this alias behaves like a named object (has a definition symbol),
		// we try to represent it by its name just like a *types.Named.
		if obj := tt.Obj(); obj != nil && obj.Name() != "" {
			pkg := obj.Pkg()
			if pkg != nil && pkg.Path() != currPkg.Path() {
				pkgName := pkg.Name()
				if file != nil {
					targetPath := fmt.Sprintf(`"%s"`, pkg.Path())
					for _, imp := range file.Imports {
						if imp.Path != nil && imp.Path.Value == targetPath {
							if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
								pkgName = imp.Name.Name
							}
							break
						}
					}
				}
				return &ast.SelectorExpr{
					X:   ast.NewIdent(pkgName),
					Sel: ast.NewIdent(obj.Name()),
				}
			}
			return ast.NewIdent(obj.Name())
		}
		// Fallback: If it's a completely anonymous alias or we just need the structure,
		// unroll it directly via .Rhs() to get the underlying actual type.
		return TypesExpr(tt.Rhs(), currPkg, file)

	default:
		panic(fmt.Sprintf("unsupported type: %T", t))
	}
}

func tupleToFieldList(tup *types.Tuple, currPkg *types.Package) *ast.FieldList {
	if tup.Len() == 0 {
		return &ast.FieldList{}
	}
	fields := make([]*ast.Field, tup.Len())
	for i := 0; i < tup.Len(); i++ {
		v := tup.At(i)
		fields[i] = &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(v.Name())},
			Type:  TypesExpr(v.Type(), currPkg, nil),
		}
	}
	return &ast.FieldList{List: fields}
}

func RemovePos(n ast.Node) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.File:
			x.Package = token.NoPos
			x.Comments = nil
		case *ast.Ident:
			x.NamePos = token.NoPos
		case *ast.BasicLit:
			x.ValuePos = token.NoPos
		case *ast.CompositeLit:
			x.Lbrace = token.NoPos
			x.Rbrace = token.NoPos
		case *ast.ArrayType:
			x.Lbrack = token.NoPos
		case *ast.FuncType:
			x.Func = token.NoPos
		case *ast.FieldList:
			x.Opening = token.NoPos
			x.Closing = token.NoPos
		case *ast.InterfaceType:
			x.Interface = token.NoPos
		case *ast.MapType:
			x.Map = token.NoPos
		case *ast.ChanType:
			x.Begin = token.NoPos
			x.Arrow = token.NoPos
		case *ast.StructType:
			x.Struct = token.NoPos
		case *ast.SliceExpr:
			x.Lbrack = token.NoPos
			x.Rbrack = token.NoPos
		case *ast.IndexExpr:
			x.Lbrack = token.NoPos
			x.Rbrack = token.NoPos
		case *ast.CallExpr:
			x.Lparen = token.NoPos
			x.Rparen = token.NoPos
		case *ast.StarExpr:
			x.Star = token.NoPos
		case *ast.UnaryExpr:
			x.OpPos = token.NoPos
		case *ast.BinaryExpr:
			x.OpPos = token.NoPos
		case *ast.KeyValueExpr:
			x.Colon = token.NoPos
		case *ast.ParenExpr:
			x.Lparen = token.NoPos
			x.Rparen = token.NoPos
		case *ast.BlockStmt:
			x.Lbrace = token.NoPos
			x.Rbrace = token.NoPos
		case *ast.AssignStmt:
			x.TokPos = token.NoPos
		case *ast.GoStmt:
			x.Go = token.NoPos
		case *ast.DeferStmt:
			x.Defer = token.NoPos
		case *ast.ReturnStmt:
			x.Return = token.NoPos
		case *ast.IfStmt:
			x.If = token.NoPos
		case *ast.ForStmt:
			x.For = token.NoPos
		case *ast.RangeStmt:
			x.For = token.NoPos
			x.TokPos = token.NoPos
		case *ast.SelectStmt:
			x.Select = token.NoPos
		case *ast.SwitchStmt:
			x.Switch = token.NoPos
		case *ast.TypeSwitchStmt:
			x.Switch = token.NoPos
		case *ast.CaseClause:
			x.Case = token.NoPos
			x.Colon = token.NoPos
		case *ast.CommClause:
			x.Case = token.NoPos
			x.Colon = token.NoPos
		case *ast.LabeledStmt:
			x.Colon = token.NoPos
		case *ast.SendStmt:
			x.Arrow = token.NoPos
		case *ast.IncDecStmt:
			x.TokPos = token.NoPos
		case *ast.BranchStmt:
			x.TokPos = token.NoPos
		case *ast.GenDecl:
			x.TokPos = token.NoPos
			x.Lparen = token.NoPos
			x.Rparen = token.NoPos
		case *ast.TypeAssertExpr:
			x.Lparen = token.NoPos
			x.Rparen = token.NoPos
		case *ast.ImportSpec:
			x.EndPos = token.NoPos
		case *ast.TypeSpec:
			x.Assign = token.NoPos
		case *ast.Comment:
			x.Slash = token.NoPos
		case *ast.Ellipsis:
			x.Ellipsis = token.NoPos
		}
		return true
	})
}

func IsWaitGroupType(typ types.Type) bool {
	// Normalize pointer types: handle *T by checking T.
	if ptr, ok := typ.(*types.Pointer); ok {
		return IsWaitGroupType(ptr.Elem())
	}
	// Direct sync.WaitGroup named type.
	if named, ok := typ.(*types.Named); ok {
		if obj := named.Obj(); obj != nil && obj.Pkg() != nil {
			if obj.Pkg().Path() == "sync" && obj.Name() == "WaitGroup" {
				return true
			}
		}
		// If this named type's underlying type is a struct, check for embedded WaitGroups.
		if st, ok := named.Underlying().(*types.Struct); ok {
			for i := 0; i < st.NumFields(); i++ {
				field := st.Field(i)
				if field.Embedded() && IsWaitGroupType(field.Type()) {
					return true
				}
			}
		}
		return false
	}
	// For plain struct types, check embedded fields for WaitGroups.
	if st, ok := typ.(*types.Struct); ok {
		for i := 0; i < st.NumFields(); i++ {
			field := st.Field(i)
			if field.Embedded() && IsWaitGroupType(field.Type()) {
				return true
			}
		}
	}
	return false
}
