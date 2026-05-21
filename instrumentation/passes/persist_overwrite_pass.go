package passes

import (
	"bytes"
	"go/format"
	"os"
	"golang.org/x/tools/go/analysis"
)

var PersistOverwriteAnalyzer = &analysis.Analyzer{
	Name: "persist_overwrite",
	Doc:  "persist instrumented code to disk by overwriting",
	Run:  runPersistOverwritePass,
	Requires: []*analysis.Analyzer{
		TestAnalyzer,
		GoroutineAnalyzer,
		WaitGroupAnalyzer,
		CondVarAnalyzer,
		// SyncShadowAnalyzer,
		SelectAnalyzer,
		ChannelAnalyzer,
		LoopAnalyzer,
	},
}

func runPersistOverwritePass(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		var buf bytes.Buffer
		if err := format.Node(&buf, pass.Fset, file); err != nil {
			return nil, err
		}

		fileName := pass.Fset.File(file.Pos()).Name()
		if err := os.WriteFile(fileName, buf.Bytes(), 0644); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
