package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manager handles generic JSON file storage with atomic writes
type Manager struct {
	mu       sync.RWMutex
	dataFile string
	data     map[string]interface{}
}

// New creates a new storage manager for a given file
func New(dataFilePath string) (*Manager, error) {
	m := &Manager{
		dataFile: dataFilePath,
		data:     make(map[string]interface{}),
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dataFilePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	// Load existing data
	if err := m.load(); err != nil {
		fmt.Printf("Note: creating new storage file at %s\n", dataFilePath)
	}

	return m, nil
}

// Set stores a key-value pair
func (m *Manager) Set(key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value
	return m.save()
}

// Get retrieves a value by key
func (m *Manager) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, exists := m.data[key]
	return val, exists
}

// GetTyped retrieves and unmarshals a value into a typed struct
func (m *Manager) GetTyped(key string, v interface{}) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, exists := m.data[key]
	if !exists {
		return false, nil
	}

	// Marshal back to JSON and unmarshal into the typed struct
	jsonData, err := json.Marshal(val)
	if err != nil {
		return false, fmt.Errorf("failed to marshal data: %v", err)
	}

	if err := json.Unmarshal(jsonData, v); err != nil {
		return false, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return true, nil
}

// GetAll returns all data
func (m *Manager) GetAll() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	result := make(map[string]interface{})
	for k, v := range m.data {
		result[k] = v
	}
	return result
}

// Delete removes a key
func (m *Manager) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return m.save()
}

// Clear removes all data
func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]interface{})
	return m.save()
}

// Private methods

func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	// Write to temp file first, then rename (atomic operation)
	tmpFile := m.dataFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	if err := os.Rename(tmpFile, m.dataFile); err != nil {
		os.Remove(tmpFile) // cleanup
		return fmt.Errorf("failed to rename file: %v", err)
	}

	return nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file doesn't exist yet, that's ok
		}
		return err
	}

	m.data = make(map[string]interface{})
	if err := json.Unmarshal(data, &m.data); err != nil {
		return fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return nil
}
