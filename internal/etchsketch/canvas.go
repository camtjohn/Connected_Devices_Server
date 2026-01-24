package etchsketch

import (
	"encoding/binary"
	"sync"
)

// Canvas represents the shared 16x16 drawing canvas with 3 color channels
type Canvas struct {
	mu       sync.RWMutex
	red      [16]uint16 // Bitmask for each row (16 columns per row)
	green    [16]uint16
	blue     [16]uint16
	sequence uint16 // Monotonically increasing sequence number
}

// NewCanvas creates a new empty canvas
func NewCanvas() *Canvas {
	return &Canvas{
		red:      [16]uint16{},
		green:    [16]uint16{},
		blue:     [16]uint16{},
		sequence: 0,
	}
}

// GetState returns a deep copy of the current canvas state and sequence number
func (c *Canvas) GetState() (red [16]uint16, green [16]uint16, blue [16]uint16, seq uint16) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.red, c.green, c.blue, c.sequence
}

// GetSequence returns the current sequence number
func (c *Canvas) GetSequence() uint16 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sequence
}

// SetState replaces the entire canvas state and sequence number
func (c *Canvas) SetState(seq uint16, red [16]uint16, green [16]uint16, blue [16]uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sequence = seq
	c.red = red
	c.green = green
	c.blue = blue
}

// EncodeFullFrame encodes the full canvas state as a frame message
// Returns byte array: [type(0x21)][length(98)][seq][red[16]][green[16]][blue[16]]
func (c *Canvas) EncodeFullFrame() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	msg := make([]byte, 100) // 2-byte header + 98-byte payload
	msg[0] = 0x21            // MSG_TYPE_SHARED_VIEW_FRAME
	msg[1] = 98              // Payload length

	// Encode sequence number (big-endian)
	binary.BigEndian.PutUint16(msg[2:4], c.sequence)

	// Encode red channel (16 x uint16) using native endianness (little-endian)
	offset := 4
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint16(msg[offset:offset+2], c.red[i])
		offset += 2
	}

	// Encode green channel (16 x uint16) using native endianness (little-endian)
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint16(msg[offset:offset+2], c.green[i])
		offset += 2
	}

	// Encode blue channel (16 x uint16) using native endianness (little-endian)
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint16(msg[offset:offset+2], c.blue[i])
		offset += 2
	}

	return msg
}

// DecodeFullFrame parses a raw frame message and returns the sequence number and canvas state
func DecodeFullFrame(payload []byte) (uint16, [16]uint16, [16]uint16, [16]uint16, error) {
	if len(payload) < 98 {
		return 0, [16]uint16{}, [16]uint16{}, [16]uint16{}, ErrInvalidPayload
	}

	seq := binary.BigEndian.Uint16(payload[0:2])

	var red, green, blue [16]uint16
	offset := 2

	// Decode red channel using native endianness (little-endian)
	for i := 0; i < 16; i++ {
		red[i] = binary.LittleEndian.Uint16(payload[offset : offset+2])
		offset += 2
	}

	// Decode green channel using native endianness (little-endian)
	for i := 0; i < 16; i++ {
		green[i] = binary.LittleEndian.Uint16(payload[offset : offset+2])
		offset += 2
	}

	// Decode blue channel using native endianness (little-endian)
	for i := 0; i < 16; i++ {
		blue[i] = binary.LittleEndian.Uint16(payload[offset : offset+2])
		offset += 2
	}

	return seq, red, green, blue, nil
}
