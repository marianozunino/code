/*
Copyright © 2024 Mariano Zunino <marianoz@posteo.net>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marianozunino/code/v2/internal/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	BaseDir      string `mapstructure:"base_dir"`
	MruFile      string `mapstructure:"mru_file"`
	SelectorFile string `mapstructure:"selector_file"`
}

const (
	maxWaitTime    = 2 * time.Second
	initialBackoff = 100 * time.Millisecond
	backoffFactor  = 2
)

var (
	cfgFile      string
	cfg          Config
	baseDir      string
	selectorFile string
)

var rootCmd = &cobra.Command{
	Use:   "code [base-dir]",
	Short: "Project launcher for development directories",
	Long: `Code is a CLI tool that helps you quickly navigate and open your development projects.
It maintains a most-recently-used (MRU) list and integrates with your preferred editor.`,
	Args: cobra.MaximumNArgs(1),
	RunE: launchProject,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.code.yaml)")
	rootCmd.PersistentFlags().StringVarP(&selectorFile, "selector-file", "s", "", "yaml config file that defines the project selector")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".code")
		viper.SetDefault("base_dir", filepath.Join(home, "Dev"))
		viper.SetDefault("mru_file", filepath.Join(home, ".code_mru"))
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		cfg.BaseDir = os.Args[1]
	}

	if selectorFile != "" {
		cfg.SelectorFile = selectorFile
	}
}

// launchProject handles the project selection and launching process.
func launchProject(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
	defer cancel()

	mruList := core.NewMRUList(cfg.MruFile, cfg.BaseDir)
	defer mruList.Close() // Ensure MRU is saved on exit

	finder := &core.ProjectFinder{}
	allProjects := finder.FindProjects(cfg.BaseDir)

	uniqueProjects := core.RemoveDuplicates(append(mruList.Items(), allProjects...))
	if len(uniqueProjects) == 0 {
		return fmt.Errorf("no projects found in %s", cfg.BaseDir)
	}

	// Load configuration
	appConfig, err := core.LoadConfig(cfg.SelectorFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	selector := core.NewSelector(appConfig)
	selectedProject, err := selector.Select(uniqueProjects)
	if err != nil {
		return fmt.Errorf("project selection failed: %w", err)
	}
	if selectedProject == "" {
		return nil
	}

	fullPath := filepath.Join(cfg.BaseDir, selectedProject)
	if !isDirectory(fullPath) {
		return fmt.Errorf("not a directory: %s", fullPath)
	}

	windowTitle := fmt.Sprintf("nvim ~ %s", filepath.Base(fullPath))

	if err := launchOrFocusWindow(ctx, selector, fullPath, windowTitle); err != nil {
		return fmt.Errorf("failed to launch/focus window: %w", err)
	}

	return mruList.Update(selectedProject)
}

// launchOrFocusWindow either focuses an existing window or launches a new one.
func launchOrFocusWindow(ctx context.Context, selector *core.Selector, projectPath, windowTitle string) error {
	windowManager := &core.WindowManager{}
	windowID, _ := windowManager.FindWindow(windowTitle)

	if windowID == 0 {
		if err := selector.Start(projectPath, windowTitle); err != nil {
			return err
		}
		windowID, _ = waitForWindow(ctx, windowTitle)
	} else {
		if err := windowManager.FocusWindow(windowID); err != nil {
			return err
		}
	}

	return nil
}

// isDirectory checks if the given path is a directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// waitForWindow waits for a window with the given title to appear.
func waitForWindow(ctx context.Context, title string) (int64, error) {
	backoff := initialBackoff
	windowManager := &core.WindowManager{}

	for {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("timeout waiting for window: %s", title)
		default:
			if windowID, _ := windowManager.FindWindow(title); windowID != 0 {
				return windowID, nil
			}
			time.Sleep(backoff)
			backoff *= backoffFactor
		}
	}
}
