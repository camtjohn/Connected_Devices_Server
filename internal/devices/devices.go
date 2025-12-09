package devices

import (
	"fmt"
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

type DeviceManager struct {
	mu      sync.RWMutex
	devices map[string]*Device
}

var manager = &DeviceManager{
	devices: make(map[string]*Device),
}

// RegisterDevice sets device as active on bootup message and saves to persistent storage
// If device config doesn't exist, creates it with provided zipcode
// If device exists, uses the stored zipcode instead
func RegisterDevice(deviceID string, zipcode string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	var storedZipcode string
	var name string

	// Check if we have stored data for this device
	if storedDevice, exists := storage.devices[deviceID]; exists {
		storedZipcode = storedDevice.Zipcode
		name = storedDevice.Name
		fmt.Printf("Device %s reconnected, using stored zipcode: %s\n", deviceID, storedZipcode)
	} else {
		// First time seeing this device, use provided zipcode and save
		storedZipcode = zipcode
		name = deviceID
		if err := AddOrUpdateDevice(deviceID, name, storedZipcode, true, time.Now().Format(time.RFC3339)); err != nil {
			fmt.Printf("Warning: failed to save device data: %v\n", err)
		}
		fmt.Printf("Device %s registered with zipcode: %s\n", deviceID, storedZipcode)
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

	// Update status in persistent storage
	if err := AddOrUpdateDevice(deviceID, name, storedZipcode, true, time.Now().Format(time.RFC3339)); err != nil {
		fmt.Printf("Warning: failed to update device data: %v\n", err)
	}
}

// SetInactive marks device as inactive (e.g., on LWT)
func SetInactive(deviceID string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if device, exists := manager.devices[deviceID]; exists {
		device.Active = false
		fmt.Printf("Device %s set to inactive (LWT triggered)\n", deviceID)

		// Update in persistent storage
		if err := AddOrUpdateDevice(deviceID, device.Name, device.Zipcode, false, time.Now().Format(time.RFC3339)); err != nil {
			fmt.Printf("Warning: failed to update device data: %v\n", err)
		}
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

		// Update in persistent storage
		if err := AddOrUpdateDevice(deviceID, device.Name, device.Zipcode, true, time.Now().Format(time.RFC3339)); err != nil {
			fmt.Printf("Warning: failed to update device data: %v\n", err)
		}
	}
}

// GetActiveDevices returns list of all active devices
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
