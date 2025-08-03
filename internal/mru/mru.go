package mru

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type MRUList struct {
	filename string
	baseDir  string
	items    []string
	dirty    bool
	lastMod  time.Time
	mu       sync.RWMutex
}

func NewMRUList(filename, baseDir string) *MRUList {
	mru := &MRUList{
		filename: filename,
		baseDir:  baseDir,
	}
	mru.loadWithModTime()
	return mru
}

func (m *MRUList) loadWithModTime() {
	stat, err := os.Stat(m.filename)
	if err != nil {
		m.items = []string{}
		m.lastMod = time.Time{}
		return
	}

	if !stat.ModTime().After(m.lastMod) && m.items != nil {
		return
	}

	data, err := os.ReadFile(m.filename)
	if err != nil {
		m.items = []string{}
		return
	}

	m.items = strings.Fields(string(data))
	m.lastMod = stat.ModTime()
}

func (m *MRUList) save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.dirty {
		return nil
	}

	var builder strings.Builder
	for i, item := range m.items {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(item)
	}

	err := os.WriteFile(m.filename, []byte(builder.String()), 0o644)
	if err == nil {
		m.dirty = false
		m.lastMod = time.Now()
	}
	return err
}

func updateList(project string, list []string) []string {
	newList := make([]string, 0, 10)
	newList = append(newList, project)

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
	m.mu.Lock()
	defer m.mu.Unlock()

	m.loadWithModTime()

	fullPath := filepath.Join(m.baseDir, project)
	absPath, _ := filepath.Abs(fullPath)

	if len(m.items) > 0 && m.items[0] == absPath {
		return nil
	}

	m.items = updateList(absPath, m.items)
	m.dirty = true

	return m.save()
}

func (m *MRUList) Items() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.loadWithModTime()

	if len(m.items) == 0 {
		return []string{}
	}

	relative := make([]string, 0, len(m.items))

	for _, item := range m.items {
		rel, err := filepath.Rel(m.baseDir, item)
		if err == nil {
			relative = append(relative, rel)
		}
	}
	return relative
}

// Flush forces save of dirty data to disk
func (m *MRUList) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.dirty {
		return m.save()
	}
	return nil
}
