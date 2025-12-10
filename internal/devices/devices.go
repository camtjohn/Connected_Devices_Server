package devices

import (
	"encoding/json"
	"fmt"
	"server_app/internal/storage"
	"sync"
	"time"
)

type Device struct {
	ID       string    // Device identifier from bootup message
	Name     string    // Human-readable device name
	Zipcode  string    // Single zipcode this device is associated with
	LastSeen time.Time // Last time we heard from this device
	Active   bool      // Whether device is currently active
}

type DeviceData struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
	Zipcode  string `json:"zipcode"`
	Active   bool   `json:"active"`
	LastSeen string `json:"last_seen"`
}

type DeviceManager struct {
	mu      sync.RWMutex
	devices map[string]*Device
	store   *storage.Manager
}

var manager = &DeviceManager{
	devices: make(map[string]*Device),
}

// InitStorage initializes device storage
func InitStorage(dataFilePath string) error {
	var err error
	manager.store, err = storage.New(dataFilePath)
	if err != nil {
		return err
	}

	// Load devices from persistent storage into memory
	allData := manager.store.GetAll()
	for key, val := range allData {
		var deviceData DeviceData
		if err := reconvertToDeviceData(val, &deviceData); err != nil {
			fmt.Printf("Warning: failed to load device %s: %v\n", key, err)
			continue
		}

		lastSeen, _ := time.Parse(time.RFC3339, deviceData.LastSeen)
		manager.devices[key] = &Device{
			ID:       deviceData.DeviceID,
			Name:     deviceData.Name,
			Zipcode:  deviceData.Zipcode,
			LastSeen: lastSeen,
			Active:   deviceData.Active,
		}
	}

	fmt.Printf("Loaded %d devices from storage\n", len(manager.devices))
	return nil
}

// RegisterDevice sets device as active on bootup message and saves to persistent storage
// If deviceName differs from stored name, updates the stored entry
func RegisterDevice(deviceID string, deviceName string, zipcode string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	var storedZipcode string
	var name string

	// Check if we have stored data for this device
	if storedDevice, exists := manager.devices[deviceID]; exists {
		storedZipcode = storedDevice.Zipcode
		name = deviceName // Use new name from bootup message
		if name != storedDevice.Name {
			fmt.Printf("Device %s name changed from '%s' to '%s'\n", deviceID, storedDevice.Name, name)
		}
		fmt.Printf("Device %s reconnected, using stored zipcode: %s\n", deviceID, storedZipcode)
	} else {
		// First time seeing this device, use provided name and zipcode
		storedZipcode = zipcode
		name = deviceName
		fmt.Printf("Device %s ('%s') registered with zipcode: %s\n", deviceID, name, storedZipcode)
	}

	if device, exists := manager.devices[deviceID]; exists {
		// Device already in memory, update it
		device.Active = true
		device.LastSeen = time.Now()
		device.Zipcode = storedZipcode
		device.Name = name
	} else {
		// New device in memory
		manager.devices[deviceID] = &Device{
			ID:       deviceID,
			Name:     name,
			Zipcode:  storedZipcode,
			LastSeen: time.Now(),
			Active:   true,
		}
	}

	// Update in persistent storage
	saveDeviceToStorage(deviceID)
}

// SetInactive marks device as inactive (e.g., on LWT)
func SetInactive(deviceID string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if device, exists := manager.devices[deviceID]; exists {
		device.Active = false
		fmt.Printf("Device %s set to inactive (LWT triggered)\n", deviceID)
		saveDeviceToStorage(deviceID)
	}
}

// Heartbeat updates last seen time for a device
func Heartbeat(deviceID string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if device, exists := manager.devices[deviceID]; exists {
		device.LastSeen = time.Now()
		// If it was marked inactive and we get a heartbeat, reactivate it
		if !device.Active {
			device.Active = true
			fmt.Printf("Device %s reactivated by heartbeat\n", deviceID)
		}
		saveDeviceToStorage(deviceID)
	}
} // GetActiveDevices returns list of all active devices
func GetActiveDevices() []Device {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	var active []Device
	for _, device := range manager.devices {
		if device.Active {
			active = append(active, *device)
		}
	}
	return active
}

// IsZipcodeActive checks if any active device is associated with a zipcode
func IsZipcodeActive(zipcode string) bool {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	for _, device := range manager.devices {
		if device.Active && device.Zipcode == zipcode {
			return true
		}
	}
	return false
}

// GetDevice returns a specific device's info
func GetDevice(deviceID string) (*Device, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	device, exists := manager.devices[deviceID]
	if exists {
		return &Device{
			ID:       device.ID,
			Name:     device.Name,
			Zipcode:  device.Zipcode,
			LastSeen: device.LastSeen,
			Active:   device.Active,
		}, true
	}
	return nil, false
}

// PrintStatus prints status of all known devices
func PrintStatus() {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	fmt.Println("\n=== Device Status ===")
	if len(manager.devices) == 0 {
		fmt.Println("No devices registered")
		return
	}

	for id, device := range manager.devices {
		status := "ACTIVE"
		if !device.Active {
			status = "INACTIVE"
		}
		fmt.Printf("Device: %s (%s) | Status: %s | Last Seen: %v ago | Zipcode: %s\n",
			id, device.Name, status, time.Since(device.LastSeen).Round(time.Second), device.Zipcode)
	}
	fmt.Println("====================\n")
}

// Private helper functions

func saveDeviceToStorage(deviceID string) {
	if manager.store == nil {
		return
	}

	device := manager.devices[deviceID]
	data := DeviceData{
		DeviceID: device.ID,
		Name:     device.Name,
		Zipcode:  device.Zipcode,
		Active:   device.Active,
		LastSeen: device.LastSeen.Format(time.RFC3339),
	}

	if err := manager.store.Set(deviceID, data); err != nil {
		fmt.Printf("Warning: failed to save device %s to storage: %v\n", deviceID, err)
	}
}

func reconvertToDeviceData(val interface{}, target *DeviceData) error {
	jsonData, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}
