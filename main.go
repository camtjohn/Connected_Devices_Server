package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"server_app/internal/devices"
	"server_app/internal/etchsketch"
	"server_app/internal/messaging"
	"server_app/internal/weather"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Runtime configuration
type RuntimeConfig struct {
	DeviceVersion string `json:"deviceVersion"`
}

var (
	runtimeConfig RuntimeConfig
	configMutex   sync.RWMutex
)

// Global etchsketch manager (initialized when MQTT client is ready)
var etchsketchManager *etchsketch.Manager
var etchsketchTopic string

// Load runtime config from config.json
func loadRuntimeConfig() error {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return fmt.Errorf("failed to read config.json: %w", err)
	}

	var config RuntimeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config.json: %w", err)
	}

	configMutex.Lock()
	runtimeConfig = config
	configMutex.Unlock()

	fmt.Printf("Loaded runtime config: deviceVersion=%s\n", config.DeviceVersion)
	return nil
}

// Get current device version from runtime config as uint16
func getDeviceVersion() uint16 {
	configMutex.RLock()
	defer configMutex.RUnlock()

	version, err := strconv.ParseUint(runtimeConfig.DeviceVersion, 10, 16)
	if err != nil {
		fmt.Printf("Warning: invalid version format '%s', using default 1\n", runtimeConfig.DeviceVersion)
		return 1
	}
	return uint16(version)
}

// Periodically reload runtime config
func task_reload_config() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := loadRuntimeConfig(); err != nil {
			fmt.Printf("Warning: failed to reload config: %v\n", err)
		}
	}
}

// Monitor current time set by ntpd at bootup. Only continue when time is updated
func wait_for_current_time() {
	t := time.Now()
	num_tries := 0
	// While current time shows before 2020, wait till ntpd gets current time
	for t.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		fmt.Println("Wait 5 more seconds for ntpd to get time...")
		// Try every 5 seconds for 30 seconds, then wait a minute
		if num_tries < 6 {
			time.Sleep(5 * time.Second)
			num_tries++
		} else {
			time.Sleep(60 * time.Second)
			num_tries = 0
		}
		t = time.Now()
	}
}

// Fetch and store weather data
func fetch_weather(data_type string, zip string) {
	weather_data := weather.FetchWeatherFromAPI(data_type, zip)
	if len(weather_data) > 0 {
		weather.Store_weather(data_type, weather_data, zip)
		fmt.Printf("Fetched and stored %s for %s\n", data_type, zip)
	}
}

// Check if weather data is valid (recently updated)
func is_weather_valid(data_type string, zip string) bool {
	val, exists := weather.GetStoredWeatherData(zip)
	if !exists {
		return false
	}

	// Parse last updated time and set validity period based on data type
	var lastUpdated time.Time
	var validityPeriod time.Duration
	var err error

	if data_type == "current_weather" {
		if val.CurrentWeatherUpdated == "" {
			return false // No valid timestamp, treat as invalid
		}
		lastUpdated, err = time.Parse(time.RFC3339, val.CurrentWeatherUpdated)
		validityPeriod = time.Duration(WeatherValidityPeriod) * time.Minute
	} else if data_type == "forecast_weather" {
		if val.ForecastWeatherUpdated == "" {
			return false // No valid timestamp, treat as invalid
		}
		lastUpdated, err = time.Parse(time.RFC3339, val.ForecastWeatherUpdated)
		validityPeriod = time.Duration(ForecastValidityPeriod) * time.Minute
	} else {
		return false
	}

	if err != nil {
		fmt.Printf("Warning: could not parse weather timestamp: %v\n", err)
		return false
	}

	return time.Since(lastUpdated) <= validityPeriod
}

// Publish weather via MQTT
func publish_weather(data_type string, zip string) {
	if !is_weather_valid(data_type, zip) {
		fmt.Printf("Skipping publish: %s for %s not valid (too old)\n", data_type, zip)
		return
	}

	msg_topic := (TopicWeatherPrefix + "/" + zip)

	if data_type == "current_weather" {
		temp, err := weather.GetCurrentWeatherTemp(zip)
		if err != nil {
			fmt.Printf("Error getting current weather: %v\n", err)
			return
		}
		// Weather updates use QoS 0 per protocol specification
		messaging.PublishQoS0(msg_topic, messaging.EncodeCurrentWeather(temp))
	} else if data_type == "forecast_weather" {
		days, err := weather.GetForecastDays(zip, 3)
		if err != nil {
			fmt.Printf("Error getting forecast: %v\n", err)
			return
		}
		// Convert weather.ForecastDay to messaging.ForecastDay
		msgDays := make([]messaging.ForecastDay, len(days))
		for i, day := range days {
			msgDays[i] = messaging.ForecastDay{
				HighTemp: day.HighTemp,
				Precip:   day.Precip,
				Moon:     day.Moon,
			}
		}
		// Weather updates use QoS 0 per protocol specification
		messaging.PublishQoS0(msg_topic, messaging.EncodeForecast(msgDays))
	}
}

// Publish version notification to device
// Topic: <device_name> (e.g., "dev0" or "debug_dev0")
// Message Type: 0x10 (MSG_TYPE_VERSION)
// QoS: 1 (at-least-once delivery for critical message)
func publish_version_notification(deviceName string) {
	version := getDeviceVersion()
	msg := messaging.EncodeVersion(version)
	topicName := deviceName
	if IsDebugBuild {
		topicName = "debug_" + deviceName
	}
	fmt.Printf("Publishing version %d to topic %s\n", version, topicName)
	messaging.PublishQoS1(topicName, msg)
}

// Parse heartbeat message (binary format: [type][length][name_len][name_data])
// Returns device name or error
func parseHeartbeatMessage(payload []byte) (string, error) {
	if len(payload) < 3 {
		return "", fmt.Errorf("heartbeat message too short (need at least 3 bytes, got %d)", len(payload))
	}

	msgType := payload[0]
	msgLen := payload[1]

	// Check message type
	if msgType != 0x11 {
		return "", fmt.Errorf("invalid heartbeat message type: expected 0x11, got 0x%02X", msgType)
	}

	// Verify payload length matches header
	if len(payload) < 2+int(msgLen) {
		return "", fmt.Errorf("heartbeat payload length mismatch: header says %d, got %d", msgLen, len(payload)-2)
	}

	msgPayload := payload[2 : 2+msgLen]

	// Parse payload: [device_name_len][device_name_data]
	if len(msgPayload) < 1 {
		return "", fmt.Errorf("heartbeat payload missing device name length")
	}

	nameLen := msgPayload[0]
	if len(msgPayload) < 1+int(nameLen) {
		return "", fmt.Errorf("heartbeat device name length mismatch: expected %d bytes, got %d", nameLen, len(msgPayload)-1)
	}

	deviceName := string(msgPayload[1 : 1+nameLen])
	return deviceName, nil
}

// Handle device bootup: register device, fetch/publish weather, send version
func handle_device_bootup(payload []byte) {
	// Extract message payload from binary protocol
	msgType, msgPayload, err := messaging.DecodeMessage(payload)
	if err != nil {
		fmt.Printf("Error decoding message: %v\n", err)
		return
	}

	if msgType != messaging.MSG_DEVICE_CONFIG {
		fmt.Printf("Error: expected MSG_DEVICE_CONFIG (0x03), got 0x%02X\n", msgType)
		return
	}

	// Parse binary device config format using DecodeDeviceConfig
	strs, err := messaging.DecodeDeviceConfig(msgPayload)
	if err != nil {
		fmt.Printf("Error decoding device config: %v\n", err)
		return
	}

	if len(strs) < 2 {
		fmt.Printf("Error: device config requires at least 2 strings, got %d\n", len(strs))
		return
	}

	deviceName := strings.TrimSpace(strs[0])
	zipcode := strings.TrimSpace(strs[1])

	fmt.Printf("Bootup parsed: device=%s, zipcode=%s\n", deviceName, zipcode)
	if deviceName == "" || zipcode == "" {
		fmt.Println("Error: device config has empty device name or zipcode")
		return
	}

	// Register device as active
	devices.RegisterDevice(deviceName, zipcode)

	// Fetch weather only if not already valid
	if !is_weather_valid("current_weather", zipcode) {
		fetch_weather("current_weather", zipcode)
	} else {
		fmt.Printf("Current weather for %s is already valid, skipping fetch\n", zipcode)
	}

	if !is_weather_valid("forecast_weather", zipcode) {
		fetch_weather("forecast_weather", zipcode)
	} else {
		fmt.Printf("Forecast for %s is already valid, skipping fetch\n", zipcode)
	}

	time.Sleep(1 * time.Second)

	// Publish weather to device
	publish_weather("current_weather", zipcode)
	publish_weather("forecast_weather", zipcode)

	// Publish version notification to device (QoS 1 per protocol specification)
	publish_version_notification(deviceName)
}

// Handle etchsketch shared view messages
func handle_etchsketch_message(payload []byte) {
	if len(payload) < 2 {
		fmt.Println("Error: etchsketch message too short")
		return
	}

	msgType := payload[0]
	msgLen := payload[1]

	if len(payload) < 2+int(msgLen) {
		fmt.Printf("Error: etchsketch message length mismatch (expected %d, got %d)\n", msgLen, len(payload)-2)
		return
	}

	msgPayload := payload[2 : 2+msgLen]

	switch msgType {
	case messaging.MSG_TYPE_ETCH_GET_FRAME:
		// Device requesting full canvas state
		fmt.Println("Received etchsketch sync request")
		if err := etchsketchManager.HandleSyncRequest("device"); err != nil {
			fmt.Printf("Error handling sync request: %v\n", err)
		}

	case messaging.MSG_TYPE_ETCH_UPDATE_FRAME:
		// Device publishes updated full frame; server updates local state only
		if len(msgPayload) != 98 {
			fmt.Printf("Invalid etch_update_frame payload length: %d (expected 98)\n", len(msgPayload))
			return
		}
		seq, red, green, blue, err := etchsketch.DecodeFullFrame(msgPayload)
		if err != nil {
			fmt.Printf("Failed to decode full frame: %v\n", err)
			return
		}
		etchsketchManager.HandleFullFrameUpdate(seq, red, green, blue)
		fmt.Printf("Applied etch_update_frame (seq=%d)\n", seq)

	default:
		fmt.Printf("Unknown etchsketch message type: 0x%02X\n", msgType)
	}
}

// Handler responds to mqtt messages for following topics
var msg_handler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	topic := string(msg.Topic())
	payload := msg.Payload()

	if topic == TopicBootup {
		fmt.Printf("Received bootup message on %s (bytes=%d)\n", TopicBootup, len(payload))
		handle_device_bootup(payload)
	}

	// Device heartbeat - keep device marked as active
	if topic == TopicHeartbeat {
		deviceName, err := parseHeartbeatMessage(payload)
		if err != nil {
			fmt.Printf("Error parsing heartbeat message: %v\n", err)
		} else if deviceName != "" {
			devices.Heartbeat(deviceName)
			fmt.Printf("Heartbeat received from %s\n", deviceName)
			// Respond with version notification on every heartbeat
			publish_version_notification(deviceName)
		}
	}

	// Device Last Will Testament - triggered on ungraceful disconnect (network/power loss)
	if topic == TopicOffline {
		deviceName := string(payload)
		if deviceName != "" {
			devices.SetInactive(deviceName)
		}
	}

	// Etchsketch shared view messages
	if topic == etchsketchTopic && etchsketchManager != nil {
		handle_etchsketch_message(payload)
	}
}

// Update weather every x minutes
func task_weather() {
	ticker := time.NewTicker(time.Duration(WeatherUpdateInterval) * time.Minute)
	forecastTicker := time.NewTicker(time.Duration(ForecastUpdateInterval) * time.Minute)
	defer ticker.Stop()
	defer forecastTicker.Stop()

	for {
		select {
		case <-ticker.C:
			// Fetch current weather for all active device zipcodes
			activeZipcodes := devices.GetActiveZipcodes()
			if len(activeZipcodes) == 0 {
				fmt.Println("No active devices, skipping weather fetch")
			} else {
				fmt.Printf("Fetching current weather for %d zipcode(s)\n", len(activeZipcodes))
				for _, zip := range activeZipcodes {
					fetch_weather("current_weather", zip)
					// Publish immediately so devices receive refreshed data without waiting for reboot
					publish_weather("current_weather", zip)
					time.Sleep(1 * time.Second)
				}
			}

		case <-forecastTicker.C:
			// Fetch forecast for all active device zipcodes
			activeZipcodes := devices.GetActiveZipcodes()
			if len(activeZipcodes) == 0 {
				fmt.Println("No active devices, skipping forecast fetch")
			} else {
				fmt.Printf("Fetching forecast for %d zipcode(s)\n", len(activeZipcodes))
				for _, zip := range activeZipcodes {
					fetch_weather("forecast_weather", zip)
					publish_weather("forecast_weather", zip)
					time.Sleep(1 * time.Second)
				}
			}
		}
	}
}

// Ping healthcheck.io: monitor will email if it does not receive ping in x minutes
func task_healthcheck(url string) {
	client := &http.Client{Timeout: 10 * time.Second}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		err := pingHealthcheck(client, url)
		if err != nil {
			// Ping failed, retry a few times before next scheduled check
			backoff := time.Second * 30
			for i := 0; i < 5; i++ {
				time.Sleep(backoff)
				if err = pingHealthcheck(client, url); err == nil {
					// Ping successful
					break
				}
				backoff *= 2 // exponential backoff
			}
		}
		<-ticker.C
	}
}

func pingHealthcheck(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func start_mqtt_process() {
	messaging.Create_client(msg_handler, []string{TopicBootup, TopicTest}, IsDebugBuild)

	// Initialize etchsketch manager on configured topic
	etchsketchTopic = TopicEtchSketch
	etchsketchManager = etchsketch.NewManager(messaging.GetClient(), etchsketchTopic)

	// Clear retained shared view frames so devices don't receive unsolicited frames on boot
	messaging.PublishRetained(etchsketchTopic, []byte{})

	// Subscribe to device offline topic (Last Will Testament from devices)
	messaging.Subscribe(TopicOffline, msg_handler)
	// Subscribe to heartbeat topic for device keepalives
	messaging.Subscribe(TopicHeartbeat, msg_handler)
	// Subscribe to etchsketch shared view topic
	messaging.Subscribe(etchsketchTopic, msg_handler)
}

func main() {
	if IsDebugBuild {
		fmt.Println("Starting up... [DEBUG BUILD]")
	} else {
		fmt.Println("Starting up... [PRODUCTION BUILD]")
	}

	// Initialize persistent device storage (separate files for debug/prod)
	var deviceStoragePath string
	var weatherStoragePath string
	if IsDebugBuild {
		deviceStoragePath = "./data/devices_debug.json"
		weatherStoragePath = "./data/weather_debug.json"
	} else {
		deviceStoragePath = "./data/devices.json"
		weatherStoragePath = "./data/weather.json"
	}

	if err := devices.InitStorage(deviceStoragePath); err != nil {
		fmt.Printf("Warning: failed to initialize device storage: %v\n", err)
	}

	// Initialize weather storage
	if err := weather.InitWeatherStorage(weatherStoragePath); err != nil {
		fmt.Printf("Warning: failed to initialize weather storage: %v\n", err)
	}

	// Load runtime config
	if err := loadRuntimeConfig(); err != nil {
		fmt.Printf("Warning: failed to load runtime config: %v (using defaults)\n", err)
		// Set default version
		configMutex.Lock()
		runtimeConfig.DeviceVersion = "1.0.0"
		configMutex.Unlock()
	}

	wait_for_current_time() // Channel to signal when to stop process
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Post request every x minutes to healthcheck.io
	go task_healthcheck("https://hc-ping.com/5b729be7-9787-405a-b26f-76ad7aad6ca4")

	// Get weather every x minutes
	go task_weather()

	// Reload runtime config every 15 minutes
	go task_reload_config()

	start_mqtt_process()

	fmt.Println("Finished process initializing")

	<-c // Block until signal received

	fmt.Println("Exiting server application")
}
