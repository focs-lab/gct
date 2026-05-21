package utils

import (
	"os"
	"path/filepath"
)

func ListAllGoFiles(d string) []string {
	var files []string

	err := filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".go" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return files
}

func ListAllGoTestFiles(d string) []string {
	var files []string

	err := filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".go" && filepath.Base(path) == "test.go" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return files
}
