package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"server_app/internal/mqtt_local"
	"server_app/internal/weather"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

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
	msg_topic := ("weather" + zip)
	// check freshness of json file. get/store new data if old.
	// for now, get forecast at bootup (already got current)
	if data_type == "forecast_weather" {
		forecast_data := weather.Get_weather("forecast_weather")
		weather.Store_weather("forecast_weather", forecast_data)
		time.Sleep(1 * time.Second)
	}
	msg_payload := weather.Read_weather(data_type)

	mqtt_local.Publish(msg_topic, msg_payload)
}

// Handler responds to mqtt messages for following topics
var msg_handler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	topic := string(msg.Topic())
	payload := string(msg.Payload())

	if topic == "test1" {
		fmt.Println("recvd msg for test1", payload)
	}

	if topic == "dev_bootup" {
		version_str := "01"
		// At some point, Will get zip from payload and update weather for that zip
		update_weather("current_weather", "49085")
		update_weather("forecast_weather", "49085")
		mqtt_local.Publish(payload, version_str)
	}
}

// Update weather every x minutes
func task_weather() {
	count_send_current := 0

	for {
		// Send current weather data to devices
		weather_data := weather.Get_weather("current_weather")
		weather.Store_weather("current_weather", weather_data)
		time.Sleep(1 * time.Second)
		update_weather("current_weather", "49085")

		// Send forecast every 6 hours = 12 times publishing current weather
		count_send_current++
		if count_send_current > 12 {
			forecast_data := weather.Get_weather("forecast_weather")
			weather.Store_weather("forecast_weather", forecast_data)
			time.Sleep(1 * time.Second)
			update_weather("forecast_weather", "49085")
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

func main() {
	fmt.Println("Starter up...")
	wait_for_current_time()

	// Channel to signal when to stop process
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Post request every x minutes to healthcheck.io
	go task_healthcheck("https://hc-ping.com/5b729be7-9787-405a-b26f-76ad7aad6ca4")

	// Get weather every x minutes
	go task_weather()

	// Start mqtt process
	mqtt_local.Create_client(msg_handler)
	fmt.Println("Finished process initializing")

	<-c // Block until signal received

	fmt.Println("Exiting server application")
}
