package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"server_app/internal/devices"
	"server_app/internal/mqtt_local"
	"server_app/internal/weather"
	"strings"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var zipcodes = []string{"78757", "60607"}
var VERSION_NUM_STRING = "001"

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

// Read/publish weather
func update_weather(data_type string, zip string) {
	msg_topic := (TopicWeatherPrefix + zip)
	// check freshness of json file. get/store new data if old.
	// for now, get forecast at bootup (already got current)
	if data_type == "forecast_weather" {
		forecast_data := weather.Get_weather("forecast_weather", zip)
		weather.Store_weather("forecast_weather", forecast_data, zip)
		time.Sleep(1 * time.Second)
	}
	msg_payload := weather.Read_weather(data_type, zip)

	mqtt_local.Publish(msg_topic, msg_payload)
}

// Handler responds to mqtt messages for following topics
var msg_handler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	topic := string(msg.Topic())
	payload := string(msg.Payload())

	if topic == TopicBootup {
		// Parse payload format: "device_name,zipcode"
		// Example: "dev001,78757"
		var deviceName, zipcode string

		// Parse the payload
		parts := strings.Split(payload, ",")
		if len(parts) < 1 {
			fmt.Println("Error: dev_bootup message format should be 'device_name' with optional zipcode")
			return
		}

		deviceName = strings.TrimSpace(parts[0])
		zipcode = strings.TrimSpace(parts[1])

		if deviceName == "" {
			fmt.Println("Error: dev_bootup message has empty device name or zipcode")
			return
		}

		// Register device as active with zipcode
		devices.RegisterDevice(deviceName, zipcode)

		// Get and publish weather for this device's zipcode
		update_weather("current_weather", zipcode)
		update_weather("forecast_weather", zipcode)

		// Respond to device with current device SW version (informs device if need OTA)
		mqtt_local.Publish(deviceName, VERSION_NUM_STRING)
	}

	// Device heartbeat - keep device marked as active
	if topic == TopicHeartbeat {
		deviceID := payload
		if deviceID != "" {
			devices.Heartbeat(deviceID)
			fmt.Printf("Heartbeat received from %s\n", deviceID)
		}
	}

	// Device Last Will Testament - triggered on ungraceful disconnect (network/power loss)
	if topic == TopicOffline {
		deviceID := payload
		if deviceID != "" {
			devices.SetInactive(deviceID)
		}
	}
}

// Update weather every x minutes
func task_weather() {
	count_send_current := 0

	for {
		// Get zipcodes from active devices
		activeZipcodes := devices.GetActiveZipcodes()

		if len(activeZipcodes) == 0 {
			fmt.Println("No active devices, skipping weather update")
		} else {
			// Send current weather data for all active device zipcodes
			for _, zip := range activeZipcodes {
				weather_data := weather.Get_weather("current_weather", zip)
				weather.Store_weather("current_weather", weather_data, zip)
				time.Sleep(1 * time.Second)
				update_weather("current_weather", zip)
			}
		}

		// Send forecast every 6 hours = 12 times publishing current weather
		count_send_current++
		if count_send_current > 12 {
			activeZipcodes := devices.GetActiveZipcodes()
			if len(activeZipcodes) == 0 {
				fmt.Println("No active devices, skipping forecast update")
			} else {
				for _, zip := range activeZipcodes {
					forecast_data := weather.Get_weather("forecast_weather", zip)
					weather.Store_weather("forecast_weather", forecast_data, zip)
					time.Sleep(1 * time.Second)
					update_weather("forecast_weather", zip)
				}
			}
			count_send_current = 0
		}

		time.Sleep(30 * time.Minute)
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
	mqtt_local.Create_client(msg_handler, []string{TopicBootup, TopicTest})

	// Subscribe to device offline topic (Last Will Testament from devices)
	mqtt_local.Subscribe(TopicOffline, msg_handler)
	// Subscribe to heartbeat topic for device keepalives
	mqtt_local.Subscribe(TopicHeartbeat, msg_handler)
}

func main() {
	if IsDebugBuild {
		fmt.Println("Starting up... [DEBUG BUILD]")
		fmt.Printf("Using debug topics: %s, %s, %s, %s\n", TopicBootup, TopicHeartbeat, TopicOffline, TopicWeatherPrefix)
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
