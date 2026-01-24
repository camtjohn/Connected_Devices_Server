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

// Removed legacy incremental update handler (pixel-level updates) —
// protocol now uses full-frame publish by devices.

// HandleFullFrameUpdate ingests a full-frame update published by a device
// The server does not republish this frame; it only updates its local state
func (m *Manager) HandleFullFrameUpdate(seq uint16, red [16]uint16, green [16]uint16, blue [16]uint16) {
	m.canvas.SetState(seq, red, green, blue)
	m.lastSeenSeq = seq
	fmt.Printf("EtchSketch: applied full frame (seq=%d)\n", seq)
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
