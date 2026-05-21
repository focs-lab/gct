package passes

import (
	"github.com/focs-lab/gct/instrumentation/utils"
	"golang.org/x/tools/go/analysis"
)

var PersistAnalyzer = &analysis.Analyzer{
	Name:     "persist_instrument",
	Doc:      "persist instrumented code to disk",
	Run:      runPersistPass,
	Requires: []*analysis.Analyzer{ChannelAnalyzer},
}

func runPersistPass(pass *analysis.Pass) (interface{}, error) {
	files := pass.Files
	for _, file := range files {
		utils.PersistInstrumentations(pass, file)
		utils.CommentOutOriginalFile(pass, file)
	}
	return nil, nil
}
