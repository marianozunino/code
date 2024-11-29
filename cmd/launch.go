package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/marianozunino/code/internal/editor"
	"github.com/marianozunino/code/internal/mru"
	"github.com/marianozunino/code/internal/must"
	"github.com/marianozunino/code/internal/project"
	"github.com/marianozunino/code/internal/ui"

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
