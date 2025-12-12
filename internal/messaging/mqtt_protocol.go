package messaging

import (
	"fmt"
)

// Message Types
const (
	MSG_GENERIC          = 0x00
	MSG_CURRENT_WEATHER  = 0x01
	MSG_FORECAST_WEATHER = 0x02
	MSG_VERSION          = 0x10
)

// Protocol constraints for ESP32 compatibility
const (
	MAX_PAYLOAD_SIZE = 255 // Maximum payload size (1-byte length field: 0-255)
)

// ForecastDay represents a single day forecast with weather data
type ForecastDay struct {
	HighTemp uint8
	Precip   uint8
	Moon     uint8
}

// EncodeCurrentWeather creates a message with type and 1 byte temp (offset +50)
func EncodeCurrentWeather(temp int8) []byte {
	msg := make([]byte, 3)
	msg[0] = MSG_CURRENT_WEATHER
	msg[1] = 1 // payload length
	msg[2] = uint8(temp + 50)
	return msg
}

// EncodeForecast creates message: [type][len][numDays][day1][day2]...
// Each day: [highTemp uint8][precip uint8][moon uint8]
func EncodeForecast(days []ForecastDay) []byte {
	payloadLen := 1 + (len(days) * 3) // 1 for numDays, 3 per day
	msg := make([]byte, 2+payloadLen)
	msg[0] = MSG_FORECAST_WEATHER
	msg[1] = uint8(payloadLen)
	msg[2] = uint8(len(days))

	offset := 3
	for _, day := range days {
		msg[offset] = day.HighTemp
		msg[offset+1] = day.Precip
		msg[offset+2] = day.Moon
		offset += 3
	}
	return msg
}

// EncodeVersion creates a version message with proper header
func EncodeVersion(version uint8) []byte {
	msg := make([]byte, 3)
	msg[0] = MSG_VERSION
	msg[1] = 1 // payload length
	msg[2] = version
	return msg
}

// EncodeGeneric creates a generic message for topic-specific data
func EncodeGeneric(payload []byte) []byte {
	msg := make([]byte, 2+len(payload))
	msg[0] = MSG_GENERIC
	msg[1] = uint8(len(payload))
	copy(msg[2:], payload)
	return msg
}

// DecodeMessage parses header and returns type, payload with bounds checking
func DecodeMessage(data []byte) (msgType uint8, payload []byte, err error) {
	if len(data) < 2 {
		return 0, nil, fmt.Errorf("message too short: got %d bytes, need at least 2", len(data))
	}

	msgType = data[0]
	length := uint16(data[1])

	// Validate length against actual data size
	if int(length) > len(data)-2 {
		return 0, nil, fmt.Errorf("invalid length field: claims %d bytes but only %d available", length, len(data)-2)
	}

	// Validate against maximum payload size for ESP32 safety
	if length > MAX_PAYLOAD_SIZE {
		return 0, nil, fmt.Errorf("payload too large: %d bytes exceeds maximum of %d", length, MAX_PAYLOAD_SIZE)
	}

	payload = data[2 : 2+length]
	return
}
