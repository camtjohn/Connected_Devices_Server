# MQTT Binary Message Protocol

## Overview

All MQTT messages use a binary protocol with a 2-byte header followed by a variable-length payload (0-255 bytes).

## Message Structure

```
[Type: 1 byte][Length: 1 byte][Payload: 0-255 bytes]
```

- **Type**: Message type identifier (see Message Types below)
- **Length**: Payload length in bytes (uint8, 0-255)
- **Payload**: Message-specific data

## Message Types

| Type | Hex  | Description          | Payload Format |
|------|------|----------------------|----------------|
| 0    | 0x00 | Generic              | Topic-dependent |
| 1    | 0x01 | Current Weather      | Temperature data |
| 2    | 0x02 | Forecast Weather     | Multi-day forecast |
| 3    | 0x03 | Device Config        | Two strings with length prefixes |
| 16   | 0x10 | Version              | Version number |

## Message Format Details

### 0x01 - Current Weather

**Total Size**: 3 bytes

```
[0x01][0x01][temp]
```

- **Type**: `0x01`
- **Length**: `0x01` (1 byte payload)
- **Payload**: 
  - `temp` (uint8): Temperature in °F with +50 offset
    - Actual temperature = `temp - 50`
    - Example: `0x46` (70) = 20°F

**Example**:
```
0x01 0x01 0x46
└─┬─┘ └┬┘ └─┬─┘
Type  Len  Temp (70 - 50 = 20°F)
```

### 0x02 - Forecast Weather

**Total Size**: 2 + 1 + (numDays × 3) bytes

```
[0x02][length][numDays][day1_data][day2_data]...
```

- **Type**: `0x02`
- **Length**: Variable (uint8)
- **Payload**:
  - `numDays` (uint8): Number of forecast days (typically 3)
  - For each day (3 bytes):
    - `highTemp` (uint8): High temperature in °F
    - `precip` (uint8): Precipitation probability (0-100%)
    - `moon` (uint8): Moon phase indicator
      - `0` = Less than 93% full
      - `1` = 93-99% full
      - `2` = 100% full (full moon)

**Example** (3-day forecast):
```
0x02 0x0A 0x03 0x4B 0x14 0x01 0x4E 0x28 0x02 0x48 0x0A 0x00
└─┬─┘ └─┬─┘ └─┬─┘ └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
Type  Len=10 3days    Day 1           Day 2           Day 3

Day 1: 75°F, 20% precip, 93-99% moon
Day 2: 78°F, 40% precip, full moon
Day 3: 72°F, 10% precip, <93% moon
```

### 0x10 - Version

**Total Size**: 3 bytes

```
[0x10][0x01][version]
```

- **Type**: `0x10`
- **Length**: `0x01` (1 byte payload)
- **Payload**:
  - `version` (uint8): Firmware version number

**Example**:
```
0x10 0x01 0x01
└─┬─┘ └─┬─┘ └─┬─┘
Type  Len   Version 1
```

### 0x03 - Device Config

**Total Size**: 2 + 1 + (number of strings × 1 length byte) + (sum of string lengths) bytes

```
[0x03][length][numStrings][len1][str1][len2][str2]...[lenN][strN]
```

- **Type**: `0x03`
- **Length**: Variable (uint8)
- **Payload**: 
  - `numStrings` (uint8): Number of strings in the message
  - For each string:
    - `lenN` (uint8): Length of string N
    - `strN` (string): String data (variable length, 0-255 bytes)

**Example** (2 strings: "ESP32_Device", "12345"):
```
0x03 0x1B 0x02 0x0C E S P 3 2 _ D e v i c e 0x05 1 2 3 4 5
└─┬─┘ └─┬─┘ └─┬─┘ └─┬─┘└──────────┬──────────┘ └─┬─┘└────┬────┘
Type  Len=27 Count=2 L1=12    "ESP32_Device"   L2=5  "12345"
```

**Example** (3 strings: "living room", "60601", "WiFi"):
```
0x03 0x20 0x03 0x0B l i v i n g   r o o m 0x05 6 0 6 0 1 0x04 W i F i
└─┬─┘ └─┬─┘ └─┬─┘ └─┬─┘└───────────┬───────┘ └─┬─┘└─────┬────┘ └┬┘└──┬─┘
Type  Len=32 Count=3 L1=11  "living room"       L2=5  "60601"  L3=4 "WiFi"
```

## Decoding Messages

### Basic Decode Steps

1. **Read byte 0**: Message type
2. **Read byte 1**: Payload length (0-255)
3. **Read bytes 2+**: Payload data based on type

### Example Code (Go)

```go
// Decode header
msgType := data[0]
payloadLen := data[1]
payload := data[2 : 2+payloadLen]

switch msgType {
case messaging.MSG_CURRENT_WEATHER:
    temp := int8(payload[0]) - 50
    fmt.Printf("Temperature: %d°F\n", temp)

case messaging.MSG_FORECAST_WEATHER:
    numDays := payload[0]
    for i := 0; i < int(numDays); i++ {
        offset := 1 + (i * 3)
        highTemp := payload[offset]
        precip := payload[offset+1]
        moon := payload[offset+2]
        fmt.Printf("Day %d: %d°F, %d%% precip, moon=%d\n", 
                   i+1, highTemp, precip, moon)
    }

case messaging.MSG_VERSION:
    version := payload[0]
    fmt.Printf("Version: %d\n", version)
}
```

## Topics

Messages are published to specific MQTT topics:

- **Weather**: `weather/<zipcode>` (or `debug_weather/<zipcode>` in debug builds)
  - Can receive MSG_CURRENT_WEATHER or MSG_FORECAST_WEATHER
- **Device Commands**: `<device_name>` (or `debug_<device_name>` in debug builds)
  - Can receive MSG_VERSION or other device-specific messages
- **Device Bootup**: `dev_bootup` (or `debug_dev_bootup` in debug builds)
  - Device sends MSG_DEVICE_CONFIG with device info
- **Heartbeat**: `dev_heartbeat` (or `debug_dev_heartbeat` in debug builds)
  - Device sends periodic heartbeat with device name
- **Offline**: `device_offline` (or `debug_device_offline` in debug builds)
  - Device Last Will Testament triggered on ungraceful disconnect
- **Shared View**: `shared_view` (or `debug_shared_view` in debug builds)
  - Device sends MSG_TYPE_SHARED_VIEW_REQ (0x20) to request full canvas
  - Server responds with MSG_TYPE_SHARED_VIEW_FRAME (0x21) containing full canvas state
  - Device sends MSG_TYPE_SHARED_VIEW_FRAME when it updates the canvas (server receives but doesn't respond)

## Implementation Notes

- Length field is 1 byte (uint8), limiting payloads to 0-255 bytes
- All current messages are under 20 bytes, so this is more than sufficient
- Temperature values use **absolute values** (no negative temperatures)
- Current weather uses **offset encoding** (+50) to handle negative temps as uint8
- Forecast temperatures are stored directly as uint8 (0-255°F range)
- Moon phase uses simplified 3-state indicator for display purposes
- Shared view uses full frame updates only - no incremental updates
