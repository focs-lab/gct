package main

import (
	"github.com/focs-lab/gct/instrumentation/passes"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(passes.PersistAnalyzer)
}
