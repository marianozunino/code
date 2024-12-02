package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"mzunino.com.uy/go/code/internal/editor"
	"mzunino.com.uy/go/code/internal/mru"
	"mzunino.com.uy/go/code/internal/must"
	"mzunino.com.uy/go/code/internal/project"
	"mzunino.com.uy/go/code/internal/ui"

	"github.com/spf13/cobra"
)

func launchProject(cmd *cobra.Command, args []string) error {
	mruList := mru.NewMRUList(cfg.MruFile, cfg.BaseDir)
	allProjects := project.FindProjects(cfg.BaseDir)
	uniqueProjects := project.RemoveDuplicates(append(mruList.Items(), allProjects...))

	selector := must.Must(ui.NewProjectSelector(cfg.SelectorFile))
	opt, _ := selector.Select(uniqueProjects)

	if opt == "" {
		return nil
	}

	fullPath := filepath.Join(cfg.BaseDir, opt)

	if !isDirectory(fullPath) {
		return fmt.Errorf("not a directory: %s", fullPath)
	}

	if err := editor.LaunchNeovim(fullPath); err != nil {
		return err
	}

	return mruList.Update(opt)
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
