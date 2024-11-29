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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	BaseDir      string `mapstructure:"base_dir"`
	MruFile      string `mapstructure:"mru_file"`
	SelectorFile string `mapstructure:"selector_file"`
}

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
	rootCmd.PersistentFlags().StringVarP(&selectorFile, "selector-file", "s", "", "lua script that defines the project selector")
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
