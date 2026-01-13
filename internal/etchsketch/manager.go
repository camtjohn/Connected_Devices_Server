package etchsketch

import (
	"fmt"
	"sync"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Manager handles incoming etchsketch messages and broadcasts updates
type Manager struct {
	mu          sync.RWMutex
	canvas      *Canvas
	client      MQTT.Client
	topic       string
	lastSeenSeq uint16
	deviceIDs   map[string]bool // Track connected devices
}

// NewManager creates a new etchsketch manager
func NewManager(client MQTT.Client, topic string) *Manager {
	return &Manager{
		canvas:      NewCanvas(),
		client:      client,
		topic:       topic,
		lastSeenSeq: 0,
		deviceIDs:   make(map[string]bool),
	}
}

// HandleSyncRequest handles a device requesting the full canvas state
// Publishes the current retained frame with QoS 0 per protocol specification
func (m *Manager) HandleSyncRequest(deviceID string) error {
	frame := m.canvas.EncodeFullFrame()

	// Shared view frames use QoS 0 per protocol specification, but should be retained
	token := m.client.Publish(m.topic, 0, true, frame)
	if !token.WaitTimeout(5000) {
		return fmt.Errorf("publish timeout for sync request from device %s", deviceID)
	}
	if token.Error() != nil {
		return fmt.Errorf("failed to publish sync frame to device %s: %w", deviceID, token.Error())
	}

	fmt.Printf("Published full frame to %s (seq=%d)\n", deviceID, m.canvas.GetSequence())
	return nil
}

// HandleDeviceUpdates applies device pixel updates to the canvas and broadcasts them
// Returns the new sequence number
func (m *Manager) HandleDeviceUpdates(deviceID string, payload []byte) (uint16, error) {
	// Parse the update batch
	deviceSeq, updates, err := DecodeUpdates(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to decode updates from device %s: %w", deviceID, err)
	}

	// Apply updates to the canonical canvas
	newSeq := m.canvas.ApplyUpdates(updates)

	// Republish full frame as retained message with new sequence (QoS 0 per protocol)
	frame := m.canvas.EncodeFullFrame()
	fmt.Printf("Publishing updated frame to %s (seq=%d, %d bytes, QoS 0, retained)\n", m.topic, newSeq, len(frame))
	token := m.client.Publish(m.topic, 0, true, frame)
	if !token.WaitTimeout(5000) {
		return newSeq, fmt.Errorf("timeout publishing updated frame from device %s", deviceID)
	}
	if token.Error() != nil {
		return newSeq, fmt.Errorf("failed to publish frame after update from device %s: %w", deviceID, token.Error())
	}

	// Broadcast pixel updates to all other devices with the new sequence number (QoS 0 per protocol)
	updateMsg := EncodeUpdates(newSeq, updates)
	token = m.client.Publish(m.topic, 0, false, updateMsg)
	if !token.WaitTimeout(5000) {
		return newSeq, fmt.Errorf("timeout broadcasting updates from device %s", deviceID)
	}
	if token.Error() != nil {
		return newSeq, fmt.Errorf("failed to broadcast updates from device %s: %w", deviceID, token.Error())
	}

	fmt.Printf("Device %s: %d pixels applied (device_seq=%d, new_seq=%d)\n", deviceID, len(updates), deviceSeq, newSeq)
	return newSeq, nil
}

// RegisterDevice tracks a device as connected to the etchsketch view
func (m *Manager) RegisterDevice(deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deviceIDs[deviceID] = true
	fmt.Printf("Registered device %s for etchsketch\n", deviceID)
}

// UnregisterDevice removes a device from the etchsketch view
func (m *Manager) UnregisterDevice(deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.deviceIDs, deviceID)
	fmt.Printf("Unregistered device %s from etchsketch\n", deviceID)
}

// GetConnectedDevices returns the list of devices connected to etchsketch
func (m *Manager) GetConnectedDevices() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]string, 0, len(m.deviceIDs))
	for id := range m.deviceIDs {
		devices = append(devices, id)
	}
	return devices
}

// GetCanvasState returns a snapshot of the current canvas
func (m *Manager) GetCanvasState() (red [16]uint16, green [16]uint16, blue [16]uint16, seq uint16) {
	return m.canvas.GetState()
}
