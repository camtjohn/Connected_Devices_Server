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

// PixelUpdate represents a single pixel modification
type PixelUpdate struct {
	Row   uint8 // 0-15
	Col   uint8 // 0-15
	Color uint8 // 0=red, 1=green, 2=blue
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

// ApplyUpdates applies a batch of pixel updates to the canvas
// Returns the new sequence number after application
func (c *Canvas) ApplyUpdates(updates []PixelUpdate) uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, update := range updates {
		if update.Row > 15 || update.Col > 15 {
			continue // Skip invalid coordinates
		}

		// Set the bit for this pixel
		bitMask := uint16(1) << update.Col

		switch update.Color {
		case 0: // Red
			c.red[update.Row] |= bitMask
		case 1: // Green
			c.green[update.Row] |= bitMask
		case 2: // Blue
			c.blue[update.Row] |= bitMask
		}
	}

	// Increment sequence number
	c.sequence++
	return c.sequence
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

// EncodeUpdates encodes a batch of pixel updates as an update message
// Returns byte array: [type(0x22)][length][seq][count][pixels...]
func EncodeUpdates(seq uint16, updates []PixelUpdate) []byte {
	if len(updates) > 32 {
		updates = updates[:32] // Truncate to max 32 updates
	}

	payloadLen := 3 + (len(updates) * 3) // 3 bytes seq+count, 3 bytes per pixel
	msg := make([]byte, 2+payloadLen)

	msg[0] = 0x22              // MSG_TYPE_SHARED_VIEW_UPDATES
	msg[1] = uint8(payloadLen) // Payload length
	binary.BigEndian.PutUint16(msg[2:4], seq)
	msg[4] = uint8(len(updates))

	// Encode pixels
	offset := 5
	for _, pixel := range updates {
		msg[offset] = pixel.Row
		msg[offset+1] = pixel.Col
		msg[offset+2] = pixel.Color
		offset += 3
	}

	return msg
}

// DecodeUpdates parses a raw update message and returns the sequence number and pixel updates
func DecodeUpdates(payload []byte) (uint16, []PixelUpdate, error) {
	if len(payload) < 4 {
		return 0, nil, ErrInvalidPayload
	}

	seq := binary.BigEndian.Uint16(payload[0:2])
	count := payload[2]

	if len(payload) < 3+(int(count)*3) {
		return 0, nil, ErrInvalidPayload
	}

	updates := make([]PixelUpdate, count)
	for i := 0; i < int(count); i++ {
		offset := 3 + (i * 3)
		updates[i] = PixelUpdate{
			Row:   payload[offset],
			Col:   payload[offset+1],
			Color: payload[offset+2],
		}
	}

	return seq, updates, nil
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
