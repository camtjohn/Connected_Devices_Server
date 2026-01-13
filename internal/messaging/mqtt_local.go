package messaging

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var client MQTT.Client

func Create_client(handler MQTT.MessageHandler, initialTopics []string, isDebug bool) {
	fmt.Println("Starting create client")
	// Use local broker on the same machine
	broker := "ssl://localhost:8883"
	fmt.Printf("Using MQTT broker: %s\n", broker)
	// include host in clientID to avoid collisions that cause broker to drop connections
	hostname, _ := os.Hostname()

	// Build clientID based on debug build flag
	var clientID string
	if isDebug {
		clientID = "go-server-debug-" + hostname
	} else {
		clientID = "go-server-" + hostname
	}
	fmt.Printf("MQTT client ID: %s\n", clientID)

	caPath := "./certs/ca.crt"
	certPath := "./certs/client_server.crt"
	keyPath := "./certs/client_server.key"

	// Load CA cert
	caCert, err := os.ReadFile(caPath)
	if err != nil {
		log.Fatalf("Failed to read CA cert: %v", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("Failed to append CA cert")
	}

	// Load client cert/key
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Fatalf("Failed to load client certificate/key: %v", err)
	}

	tlsConfig := &tls.Config{
		RootCAs:      caPool,
		Certificates: []tls.Certificate{cert},
		//InsecureSkipVerify: false, // enforce CN/SAN match
		MinVersion: tls.VersionTLS12,
	}

	// set protocol, ip, and port of broker
	opts := MQTT.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	// Use CleanSession=true to avoid queued message backlog on server restart
	opts.SetCleanSession(true)
	// tune keepalive/ping timeouts
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)

	opts.SetTLSConfig(tlsConfig)
	opts.SetDefaultPublishHandler(handler)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectTimeout(5 * time.Second)

	// OnConnect handler — subscribes to topics every time client connects
	opts.OnConnect = func(c MQTT.Client) {
		fmt.Println("Connected to MQTT broker, subscribing to topics...")
		fmt.Printf("Session clean: %v, KeepAlive: %s\n", opts.CleanSession, opts.KeepAlive)

		for _, topic := range initialTopics {
			fmt.Printf("Attempting to subscribe to %s\n", topic)
			if token := c.Subscribe(topic, 1, handler); token.Wait() && token.Error() != nil {
				log.Printf("Failed to subscribe to %s: %v", topic, token.Error())
			} else {
				fmt.Printf("Subscribed to %s\n", topic)
			}
		}
	}

	client = MQTT.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		log.Printf("MQTT connect error: %v\n", token.Error())
		return
	}
}

// PublishQoS0 publishes a message with QoS 0 (fire-and-forget)
// Used for high-frequency messages like weather and shared view updates
func PublishQoS0(topic string, data []byte) {
	// Decode and log message details for debugging
	msgType, payload, err := DecodeMessage(data)
	if err == nil {
		fmt.Printf("Publishing to %s (QoS 0) — Type: 0x%02X, PayloadLen: %d\n", topic, msgType, len(payload))
	} else {
		fmt.Printf("Publishing to %s (QoS 0) — Decode error: %v\n", topic, err)
	}
	if client == nil || !client.IsConnected() {
		log.Printf("MQTT client not connected; skipping publish to %s", topic)
		return
	}
	token := client.Publish(topic, 0, false, data)
	if !token.WaitTimeout(5 * time.Second) {
		log.Printf("Publish timeout to %s (QoS 0)", topic)
	}
	if token.Error() != nil {
		log.Printf("Publish error: %v", token.Error())
	}
}

// PublishQoS1 publishes a message with QoS 1 (at least once delivery)
// Used for critical messages like version updates and device-specific messages
func PublishQoS1(topic string, data []byte) {
	// Decode and log message details for debugging
	msgType, payload, err := DecodeMessage(data)
	if err == nil {
		fmt.Printf("Publishing to %s (QoS 1) — Type: 0x%02X, PayloadLen: %d\n", topic, msgType, len(payload))
	} else {
		fmt.Printf("Publishing to %s (QoS 1) — Decode error: %v\n", topic, err)
	}
	if client == nil || !client.IsConnected() {
		log.Printf("MQTT client not connected; skipping publish to %s", topic)
		return
	}
	token := client.Publish(topic, 1, false, data)
	if !token.WaitTimeout(15 * time.Second) {
		log.Printf("Publish timeout to %s (QoS 1)", topic)
	}
	if token.Error() != nil {
		log.Printf("Publish error: %v", token.Error())
	}
}

// Publish publishes a message with default QoS 1
// Deprecated: use PublishQoS0 or PublishQoS1 instead
func Publish(topic string, data []byte) {
	PublishQoS1(topic, data)
}

// PublishRetained publishes a message with the retained flag set and QoS 1
// Useful for last weather state so ESP32 devices get it immediately on connect
func PublishRetained(topic string, data []byte) {
	fmt.Printf("Publishing retained to %s (QoS 1)\n", topic)
	if client == nil || !client.IsConnected() {
		log.Printf("MQTT client not connected; skipping publish to %s", topic)
		return
	}
	token := client.Publish(topic, 1, true, data)
	token.Wait()
	if token.Error() != nil {
		log.Printf("Publish error: %v", token.Error())
	}
}

// DecodeAndLogMessage decodes binary protocol messages
func DecodeAndLogMessage(data []byte) {
	msgType, payload, err := DecodeMessage(data)
	if err != nil {
		log.Printf("Error decoding message: %v", err)
		return
	}
	fmt.Printf("Decoded message - Type: 0x%02X, Payload length: %d\n", msgType, len(payload))
}

func Subscribe(topic string, handler MQTT.MessageHandler) {
	if client == nil || !client.IsConnected() {
		log.Printf("MQTT client not connected; skipping subscribe to %s", topic)
		return
	}
	fmt.Printf("Attempting to subscribe to %s\n", topic)
	token := client.Subscribe(topic, 1, handler)
	token.Wait()
	if token.Error() != nil {
		log.Printf("Subscribe error to %s: %v", topic, token.Error())
	} else {
		fmt.Printf("Subscribed to %s\n", topic)
	}
}

// GetClient returns the MQTT client instance
func GetClient() MQTT.Client {
	return client
}
