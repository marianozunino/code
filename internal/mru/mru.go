package mru

import (
	"os"
	"path/filepath"
	"strings"
)

type MRUList struct {
	filename string
	baseDir  string
	items    []string
}

func NewMRUList(filename, baseDir string) *MRUList {
	return &MRUList{
		filename: filename,
		baseDir:  baseDir,
		items:    load(filename),
	}
}

func load(filename string) []string {
	data, err := os.ReadFile(filename)
	if err != nil {
		return []string{}
	}
	return strings.Fields(string(data))
}

func (m *MRUList) save() error {
	content := strings.Join(m.items, "\n")
	return os.WriteFile(m.filename, []byte(content), 0o644)
}

func updateList(project string, list []string) []string {
	newList := []string{project}
	for _, p := range list {
		if p != project && len(newList) < 10 {
			newList = append(newList, p)
		}
	}
	return newList
}

func normalizeProject(project, baseDir string) string {
	if filepath.IsAbs(project) {
		rel, err := filepath.Rel(baseDir, project)
		if err == nil {
			return rel
		}
	}
	return project
}

func (m *MRUList) Update(project string) error {
	fullPath := filepath.Join(m.baseDir, project)
	absPath, _ := filepath.Abs(fullPath)
	m.items = updateList(absPath, m.items)
	return m.save()
}

func (m *MRUList) Items() []string {
	var relative []string
	for _, item := range m.items {
		rel, err := filepath.Rel(m.baseDir, item)
		if err == nil {
			relative = append(relative, rel)
		}
	}
	return relative
}
