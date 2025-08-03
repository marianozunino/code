package mru

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxMRUItems    = 20
	tempFileSuffix = ".tmp"
	bufferSize     = 4096
)

type MRUList struct {
	filename    string
	baseDir     string
	items       []string       // Ordered list for MRU behavior
	itemSet     map[string]int // O(1) lookup: path -> index
	dirty       bool
	lastMod     time.Time
	mu          sync.RWMutex
	initialized bool
}

// NewMRUList creates a new MRU list with optimized defaults
func NewMRUList(filename, baseDir string) *MRUList {
	mru := &MRUList{
		filename: filename,
		baseDir:  baseDir,
		items:    make([]string, 0, maxMRUItems),
		itemSet:  make(map[string]int, maxMRUItems),
	}
	return mru
}

// ensureInitialized performs lazy initialization
func (m *MRUList) ensureInitialized() {
	if m.initialized {
		return
	}

	m.loadWithModTime()
	m.initialized = true
}

// loadWithModTime loads the MRU list only if the file has been modified
func (m *MRUList) loadWithModTime() {
	stat, err := os.Stat(m.filename)
	if err != nil {
		// File doesn't exist or can't be accessed
		m.items = m.items[:0]
		m.itemSet = make(map[string]int, maxMRUItems)
		m.lastMod = time.Time{}
		return
	}

	// Skip loading if file hasn't been modified since last load
	if !stat.ModTime().After(m.lastMod) && len(m.items) > 0 {
		return
	}

	if err := m.loadFromFile(); err != nil {
		// Reset to empty state on error
		m.items = m.items[:0]
		m.itemSet = make(map[string]int, maxMRUItems)
		return
	}

	m.lastMod = stat.ModTime()
}

// loadFromFile loads the MRU list from file with buffered I/O and cleanup
func (m *MRUList) loadFromFile() error {
	file, err := os.Open(m.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Clear existing data
	m.items = m.items[:0]
	m.itemSet = make(map[string]int, maxMRUItems)

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, bufferSize), bufferSize*2)

	validItems := make([]string, 0, maxMRUItems)
	seenItems := make(map[string]bool, maxMRUItems)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip duplicates
		if seenItems[line] {
			continue
		}
		seenItems[line] = true

		// Automatic cleanup: verify project still exists
		if m.projectExists(line) {
			validItems = append(validItems, line)
			if len(validItems) >= maxMRUItems {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading MRU file: %w", err)
	}

	// Update internal structures
	m.items = append(m.items, validItems...)
	m.rebuildIndex()

	// Mark as dirty if we removed invalid items
	if len(validItems) != len(seenItems) {
		m.dirty = true
	}

	return nil
}

// projectExists checks if a project path still exists
func (m *MRUList) projectExists(project string) bool {
	var fullPath string

	if filepath.IsAbs(project) {
		fullPath = project
	} else {
		fullPath = filepath.Join(m.baseDir, project)
	}

	stat, err := os.Stat(fullPath)
	return err == nil && stat.IsDir()
}

// rebuildIndex rebuilds the itemSet index for O(1) lookups
func (m *MRUList) rebuildIndex() {
	m.itemSet = make(map[string]int, len(m.items))
	for i, item := range m.items {
		m.itemSet[item] = i
	}
}

// saveAtomic performs atomic file writes to prevent corruption
func (m *MRUList) saveAtomic() error {
	if !m.dirty || len(m.items) == 0 {
		return nil
	}

	// Create temporary file in the same directory
	tempFile := m.filename + tempFileSuffix
	file, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	writer := bufio.NewWriterSize(file, bufferSize)

	// Write all items with buffered I/O
	for i, item := range m.items {
		if i > 0 {
			if _, err := writer.WriteString("\n"); err != nil {
				file.Close()
				os.Remove(tempFile)
				return fmt.Errorf("write error: %w", err)
			}
		}
		if _, err := writer.WriteString(item); err != nil {
			file.Close()
			os.Remove(tempFile)
			return fmt.Errorf("write error: %w", err)
		}
	}

	// Ensure all data is written
	if err := writer.Flush(); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("flush error: %w", err)
	}

	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("sync error: %w", err)
	}

	file.Close()

	// Atomic rename
	if err := os.Rename(tempFile, m.filename); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	m.dirty = false
	m.lastMod = time.Now()
	return nil
}

// Update adds or moves a project to the front of the MRU list with O(1) lookup
func (m *MRUList) Update(project string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureInitialized()

	// Normalize the project path
	normalizedProject := m.normalizeProject(project)

	// O(1) lookup to check if item already exists
	if existingIndex, exists := m.itemSet[normalizedProject]; exists {
		// Move existing item to front if it's not already there
		if existingIndex == 0 {
			return nil // Already at front
		}

		// Remove from current position
		copy(m.items[existingIndex:], m.items[existingIndex+1:])
		m.items = m.items[:len(m.items)-1]

		// Add to front
		m.items = append([]string{normalizedProject}, m.items...)
		m.rebuildIndex()
	} else {
		// Add new item to front
		if len(m.items) >= maxMRUItems {
			// Remove oldest item
			oldestItem := m.items[len(m.items)-1]
			delete(m.itemSet, oldestItem)
			m.items = m.items[:len(m.items)-1]
		}

		// Add to front
		m.items = append([]string{normalizedProject}, m.items...)
		m.rebuildIndex()
	}

	m.dirty = true
	return m.saveAtomic()
}

// Items returns a copy of the MRU items as relative paths
func (m *MRUList) Items() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.ensureInitialized()

	if len(m.items) == 0 {
		return []string{}
	}

	// Pre-allocate result slice
	result := make([]string, 0, len(m.items))

	for _, item := range m.items {
		if relPath := m.toRelativePath(item); relPath != "" {
			result = append(result, relPath)
		}
	}

	return result
}

// normalizeProject converts project path to absolute path for consistent storage
func (m *MRUList) normalizeProject(project string) string {
	var fullPath string

	if filepath.IsAbs(project) {
		fullPath = project
	} else {
		fullPath = filepath.Join(m.baseDir, project)
	}

	// Clean the path to remove any redundant elements
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fullPath
	}

	return absPath
}

// toRelativePath converts absolute path back to relative path for display
func (m *MRUList) toRelativePath(absPath string) string {
	relPath, err := filepath.Rel(m.baseDir, absPath)
	if err != nil {
		return ""
	}

	// Don't return paths that go outside the base directory
	if strings.HasPrefix(relPath, "..") {
		return ""
	}

	return relPath
}

// Flush forces save of dirty data to disk
func (m *MRUList) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.saveAtomic()
}

// Contains checks if a project exists in the MRU list (O(1) operation)
func (m *MRUList) Contains(project string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.ensureInitialized()

	normalizedProject := m.normalizeProject(project)
	_, exists := m.itemSet[normalizedProject]
	return exists
}

// Remove removes a project from the MRU list
func (m *MRUList) Remove(project string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureInitialized()

	normalizedProject := m.normalizeProject(project)
	index, exists := m.itemSet[normalizedProject]
	if !exists {
		return nil // Not in list
	}

	// Remove from slice
	copy(m.items[index:], m.items[index+1:])
	m.items = m.items[:len(m.items)-1]

	// Remove from index
	delete(m.itemSet, normalizedProject)
	m.rebuildIndex() // Rebuild index as positions have changed

	m.dirty = true
	return m.saveAtomic()
}

// Clear removes all items from the MRU list
func (m *MRUList) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = m.items[:0]
	m.itemSet = make(map[string]int, maxMRUItems)
	m.dirty = true

	return m.saveAtomic()
}

// Size returns the number of items in the MRU list
func (m *MRUList) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.ensureInitialized()
	return len(m.items)
}

// Cleanup removes non-existent projects from the MRU list
func (m *MRUList) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureInitialized()

	validItems := make([]string, 0, len(m.items))
	for _, item := range m.items {
		if m.projectExists(item) {
			validItems = append(validItems, item)
		}
	}

	if len(validItems) != len(m.items) {
		m.items = validItems
		m.rebuildIndex()
		m.dirty = true
		return m.saveAtomic()
	}

	return nil
}
