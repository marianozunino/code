package project

import (
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/karrick/godirwalk"
)

func FindProjects(devDir string) []string {
	var projects []string

	godirwalk.Walk(devDir, &godirwalk.Options{
		Callback: func(path string, de *godirwalk.Dirent) error {
			if !de.IsDir() {
				return nil
			}
			if _, err := git.PlainOpen(path); err == nil {
				path = strings.TrimPrefix(path, devDir+"/")
				projects = append(projects, path)
				return godirwalk.SkipThis
			}
			return nil
		},
		Unsorted: true,
	})

	return projects
}

func RemoveDuplicates(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
