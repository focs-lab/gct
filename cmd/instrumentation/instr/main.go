package main

import (
	"fmt"

	"go/ast"

	"github.com/focs-lab/gct/config"

	"path/filepath"
	"strings"

	"github.com/focs-lab/gct/instrumentation/passes"
	"github.com/focs-lab/gct/instrumentation/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// usage:
// go run main.go -path [dir_to_folder]
func main() {
	cfg := config.ParseArgs()

	absPath, err := filepath.Abs(filepath.Clean(cfg.Path))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Scheduler Name: %s\n", cfg.SchedulerName)
	fmt.Printf("Path: %s\n", cfg.Path)
	fmt.Printf("Abs Path: %s \n", absPath)

	updateAllGoMod(absPath)

	analyzers := []*analysis.Analyzer{
		passes.TestAnalyzer,
		passes.GoroutineAnalyzer,
		passes.SyncMutexAnalyzer,
		passes.WaitGroupAnalyzer,
		passes.CondVarAnalyzer,
		// passes.ContextAnalyzer,
		passes.SelectAnalyzer,
		passes.TimeAnalyzer,
		passes.ChannelAnalyzer,
	}

	for _, currAnalyzer := range analyzers {
		println("Instrumenting with pass:", currAnalyzer.Name)
		applyOnePass(currAnalyzer, absPath, cfg.Path)
	}
}

func applyOnePass(analyzer *analysis.Analyzer, absPath string, path string) {
	loadConfig := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedFiles,
		Dir:   path,
		Tests: true,
	}

	pkgs, err := packages.Load(loadConfig, "./...")
	println("len of pkgs = ", len(pkgs))
	if err != nil {
		panic("Errors loading packages: " + err.Error())
	}

	// When setting Tests: true, the loader will load both
	// the package and its corresponding _test package (if it exists).
	// This can lead to duplicate files being instrumented.
	// To avoid this, we can keep track of which files have
	// already been seen and skip them if they appear again.
	seen := make(map[string]bool)

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				fmt.Printf("Error loading package %s: %v\n", pkg.ID, e)
			}
		}

		for _, file := range pkg.Syntax {
			filename := pkg.Fset.Position(file.Pos()).Filename

			if seen[filename] {
				continue
			}
			seen[filename] = true

			if !strings.HasPrefix(filepath.Clean(filename), absPath) {
				continue
			}

			pass := &analysis.Pass{
				Fset:      pkg.Fset,
				Files:     []*ast.File{file},
				Pkg:       pkg.Types,
				TypesInfo: pkg.TypesInfo,
			}

			if _, err = analyzer.Run(pass); err != nil {
				panic(err)
			}

			afterPassDone(pass, file, pkg)
		}
	}
}

func afterPassDone(pass *analysis.Pass, file *ast.File, pkg *packages.Package) {
	// Check if monitor_go_cct is used and add import if necessary
	used := false
	ast.Inspect(file, func(n ast.Node) bool {
		if used {
			return false
		}
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == config.MONITOR_IMPORT_NAME {
				used = true
				return false
			}
		}
		return true
	})

	if used {
		utils.AddImportForMonitors(pkg.Fset, file)
	}

	// Persist only the target file
	persistPass := *pass
	persistPass.Files = []*ast.File{file}
	if _, err := passes.PersistOverwriteAnalyzer.Run(&persistPass); err != nil {
		panic(err)
	}
}
