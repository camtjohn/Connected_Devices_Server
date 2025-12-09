package devices

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DeviceData stores all information for a device (config + status combined)
type DeviceData struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
	Zipcode  string `json:"zipcode"`
	Active   bool   `json:"active"`
	LastSeen string `json:"last_seen"`
}

type StorageManager struct {
	mu       sync.RWMutex
	dataFile string
	devices  map[string]*DeviceData // key: device_id
}

var storage *StorageManager

// InitStorage initializes the storage system with a single file path
func InitStorage(dataFilePath string) error {
	storage = &StorageManager{
		dataFile: dataFilePath,
		devices:  make(map[string]*DeviceData),
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dataFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Load existing data
	if err := storage.loadData(); err != nil {
		fmt.Printf("Warning: could not load device data: %v\n", err)
		// Don't fail - file may not exist yet
	}

	return nil
}

// AddOrUpdateDevice stores/updates device data
func AddOrUpdateDevice(deviceID, deviceName, zipcode string, active bool, lastSeen string) error {
	if storage == nil {
		return fmt.Errorf("storage not initialized")
	}

	storage.mu.Lock()
	defer storage.mu.Unlock()

	storage.devices[deviceID] = &DeviceData{
		DeviceID: deviceID,
		Name:     deviceName,
		Zipcode:  zipcode,
		Active:   active,
		LastSeen: lastSeen,
	}

	return storage.saveData()
}

// GetStoredDevice retrieves a device's data from storage
func GetStoredDevice(deviceID string) (*DeviceData, bool) {
	if storage == nil {
		return nil, false
	}

	storage.mu.RLock()
	defer storage.mu.RUnlock()

	device, exists := storage.devices[deviceID]
	if exists {
		// Return a copy
		return &DeviceData{
			DeviceID: device.DeviceID,
			Name:     device.Name,
			Zipcode:  device.Zipcode,
			Active:   device.Active,
			LastSeen: device.LastSeen,
		}, true
	}
	return nil, false
}

// GetAllDevices returns all devices
func GetAllDevices() []DeviceData {
	if storage == nil {
		return []DeviceData{}
	}

	storage.mu.RLock()
	defer storage.mu.RUnlock()

	devices := make([]DeviceData, 0, len(storage.devices))
	for _, device := range storage.devices {
		devices = append(devices, *device)
	}
	return devices
}

// GetAllActiveDevices returns all currently active devices
func GetAllActiveDevices() []DeviceData {
	if storage == nil {
		return []DeviceData{}
	}

	storage.mu.RLock()
	defer storage.mu.RUnlock()

	active := make([]DeviceData, 0)
	for _, device := range storage.devices {
		if device.Active {
			active = append(active, *device)
		}
	}
	return active
}

// Private methods

func (s *StorageManager) saveData() error {
	data, err := json.MarshalIndent(s.devices, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal device data: %v", err)
	}

	// Write to temp file first, then rename (atomic operation)
	tmpFile := s.dataFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write data file: %v", err)
	}

	if err := os.Rename(tmpFile, s.dataFile); err != nil {
		os.Remove(tmpFile) // cleanup
		return fmt.Errorf("failed to rename data file: %v", err)
	}

	return nil
}

func (s *StorageManager) loadData() error {
	data, err := os.ReadFile(s.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file doesn't exist yet, that's ok
		}
		return err
	}

	devices := make(map[string]*DeviceData)
	if err := json.Unmarshal(data, &devices); err != nil {
		return fmt.Errorf("failed to unmarshal device data: %v", err)
	}

	s.devices = devices
	fmt.Printf("Loaded %d devices from file\n", len(devices))
	return nil
}
