package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"mzunino.com.uy/go/code/internal/project"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage project cache",
	Long:  `Manage the project discovery cache for faster startup times.`,
}

var clearCacheCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the project cache",
	Long:  `Clear the cached project list, forcing a fresh scan on the next run.`,
	RunE:  clearCache,
}

var infoCacheCmd = &cobra.Command{
	Use:   "info",
	Short: "Show cache information",
	Long:  `Display information about the current project cache.`,
	RunE:  showCacheInfo,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(clearCacheCmd)
	cacheCmd.AddCommand(infoCacheCmd)
}

func clearCache(cmd *cobra.Command, args []string) error {
	if err := project.ClearCache(cfg.BaseDir); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	fmt.Printf("Cache cleared for %s\n", cfg.BaseDir)
	return nil
}

func showCacheInfo(cmd *cobra.Command, args []string) error {
	exists, lastScan, projectCount := project.GetCacheInfo(cfg.BaseDir)

	if !exists {
		fmt.Printf("No cache found for %s\n", cfg.BaseDir)
		return nil
	}

	age := time.Since(lastScan)
	fmt.Printf("Cache Information:\n")
	fmt.Printf("  Base Directory: %s\n", cfg.BaseDir)
	fmt.Printf("  Last Scan: %s (%s ago)\n", lastScan.Format("2006-01-02 15:04:05"), age.Round(time.Second))
	fmt.Printf("  Projects: %d\n", projectCount)
	fmt.Printf("  Cache Age: %s\n", age.Round(time.Second))

	if age > 5*time.Minute {
		fmt.Printf("  Status: Expired (will be refreshed on next run)\n")
	} else {
		fmt.Printf("  Status: Valid\n")
	}

	return nil
}
