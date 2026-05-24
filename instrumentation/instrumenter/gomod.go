package instrumenter

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/focs-lab/gct/config"
	"golang.org/x/mod/modfile"
)

const (
	defaultRequiredVersion = "v0.1.0"
	localReplaceVersion    = "v0.0.0"
)

func updateAllGoMod(root string, opts Options) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		goModPath := filepath.Join(path, "go.mod")

		if _, err := os.Stat(goModPath); err != nil {
			return nil
		}

		fmt.Println("processing:", goModPath)

		if err := addTempImportFile(path); err != nil {
			fmt.Println("add temp import file failed:", err)
			return err
		}

		if err := updateOneGoMod(goModPath, opts); err != nil {
			fmt.Println("update go.mod failed:", err)
			return err
		}

		if err := tidyGoMod(goModPath); err != nil {
			fmt.Println("go mod tidy failed:", err)
			return err
		}

		return nil
	})
}

func addTempImportFile(folderPath string) error {
	pkg, err := detectPackageName(folderPath)
	if err != nil {
		return err
	}

	filePath := filepath.Join(folderPath, "go_cct_temp_import.go")

	content := fmt.Sprintf(`package %s 
	import _ "github.com/focs-lab/gct/monitor"`, pkg)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return err
	}

	fmt.Println("generated:", filePath)
	return nil
}

func detectPackageName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "_cct_temp_import", err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}

		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)

			if strings.HasPrefix(line, "package ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[1], nil
				}
			}
		}
	}

	return "_cct_temp_import", nil
}

func updateOneGoMod(path string, opts Options) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return fmt.Errorf("parse go.mod failed: %w", err)
	}

	if err := file.AddRequire(config.ROOT_PROJ_IMPORT_PATH, requiredVersion(opts)); err != nil {
		return fmt.Errorf("add require failed: %w", err)
	}

	if opts.ReplaceRoot != "" {
		if err := file.AddReplace(config.ROOT_PROJ_IMPORT_PATH, "", opts.ReplaceRoot, ""); err != nil {
			_ = file.DropReplace(config.ROOT_PROJ_IMPORT_PATH, "")
			_ = file.AddReplace(config.ROOT_PROJ_IMPORT_PATH, "", opts.ReplaceRoot, "")
		}
	}

	out, err := file.Format()
	if err != nil {
		return fmt.Errorf("format failed: %w", err)
	}

	return os.WriteFile(path, out, 0644)
}

func requiredVersion(opts Options) string {
	if opts.Version != "" {
		return opts.Version
	}
	if opts.ReplaceRoot != "" {
		return localReplaceVersion
	}
	return defaultRequiredVersion
}

func tidyGoMod(goModPath string) error {
	dir := filepath.Dir(goModPath)

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("tidy:", dir)

	return cmd.Run()
}
