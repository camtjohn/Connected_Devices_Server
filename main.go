package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"server_app/internal/devices"
	"server_app/internal/messaging"
	"server_app/internal/weather"
	"strings"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var VERSION_NUM_STRING = 1

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
		lastUpdated, err = time.Parse(time.RFC3339, val.CurrentWeatherUpdated)
		validityPeriod = time.Duration(WeatherValidityPeriod) * time.Minute
	} else if data_type == "forecast_weather" {
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

	msg_topic := (TopicWeatherPrefix + zip)

	if data_type == "current_weather" {
		temp, err := weather.GetCurrentWeatherTemp(zip)
		if err != nil {
			fmt.Printf("Error getting current weather: %v\n", err)
			return
		}
		messaging.Publish(msg_topic, messaging.EncodeCurrentWeather(temp))
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
		messaging.Publish(msg_topic, messaging.EncodeForecast(msgDays))
	}
}

// Handle device bootup: register device, fetch/publish weather, send version
func handle_device_bootup(payload string) {
	// Parse payload format: "device_name,zipcode"
	parts := strings.Split(payload, ",")
	if len(parts) < 2 {
		fmt.Println("Error: dev_bootup format should be 'device_name,zipcode'")
		return
	}

	deviceName := strings.TrimSpace(parts[0])
	zipcode := strings.TrimSpace(parts[1])

	if deviceName == "" || zipcode == "" {
		fmt.Println("Error: dev_bootup has empty device name or zipcode")
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

	// Send version info for OTA check using binary protocol
	messaging.Publish(deviceName, messaging.EncodeVersion(uint8(VERSION_NUM_STRING)))
}

// Handler responds to mqtt messages for following topics
var msg_handler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	topic := string(msg.Topic())
	payload := string(msg.Payload())

	if topic == TopicBootup {
		handle_device_bootup(payload)
	}

	// Device heartbeat - keep device marked as active
	if topic == TopicHeartbeat {
		deviceName := payload
		if deviceName != "" {
			devices.Heartbeat(deviceName)
			fmt.Printf("Heartbeat received from %s\n", deviceName)
		}
	}

	// Device Last Will Testament - triggered on ungraceful disconnect (network/power loss)
	if topic == TopicOffline {
		deviceName := payload
		if deviceName != "" {
			devices.SetInactive(deviceName)
		}
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
	messaging.Create_client(msg_handler, []string{TopicBootup, TopicTest})

	// Subscribe to device offline topic (Last Will Testament from devices)
	messaging.Subscribe(TopicOffline, msg_handler)
	// Subscribe to heartbeat topic for device keepalives
	messaging.Subscribe(TopicHeartbeat, msg_handler)
}

func main() {
	if IsDebugBuild {
		fmt.Println("Starting up... [DEBUG BUILD]")
	} else {
		fmt.Println("Starting up... [PRODUCTION BUILD]")
	}

	// Initialize persistent device storage (single file)
	if err := devices.InitStorage("./data/devices.json"); err != nil {
		fmt.Printf("Warning: failed to initialize device storage: %v\n", err)
	}

	// Initialize weather storage
	if err := weather.InitWeatherStorage("./data/weather.json"); err != nil {
		fmt.Printf("Warning: failed to initialize weather storage: %v\n", err)
	}

	wait_for_current_time() // Channel to signal when to stop process
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Post request every x minutes to healthcheck.io
	go task_healthcheck("https://hc-ping.com/5b729be7-9787-405a-b26f-76ad7aad6ca4")

	// Get weather every x minutes
	go task_weather()

	start_mqtt_process()

	fmt.Println("Finished process initializing")

	<-c // Block until signal received

	fmt.Println("Exiting server application")
}
