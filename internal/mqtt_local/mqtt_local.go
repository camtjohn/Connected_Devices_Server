package mqtt_local

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var client MQTT.Client

func Create_client(handler MQTT.MessageHandler) {
	fmt.Println("Starting create client")

	broker := "ssl://localhost:8883"
	// include host in clientID to avoid collisions that cause broker to drop connections
	hostname, _ := os.Hostname()
	clientID := "go-server-" + hostname

	caPath := "/home/ubuntu/server_app/certs/ca.crt"
	certPath := "/home/ubuntu/server_app/certs/client_server.crt"
	keyPath := "/home/ubuntu/server_app/certs/client_server.key"

	// Load CA cert
	caCert, err := ioutil.ReadFile(caPath)
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
		RootCAs:            caPool,
		Certificates:       []tls.Certificate{cert},
		//InsecureSkipVerify: false, // enforce CN/SAN match
		MinVersion:         tls.VersionTLS12,
	}

	// set protocol, ip, and port of broker
	opts := MQTT.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	// Keep session persistent so broker won't drop subscriptions on reconnect
	opts.SetCleanSession(false)
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

		topics := []string{"test1", "dev_bootup"}
		for _, topic := range topics {
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

func Publish(topic string, msg string) {
	fmt.Printf("Publishing to topic from mqtt: %s\n", topic)
	if client == nil || !client.IsConnected() {
		log.Printf("MQTT client not connected; skipping publish to %s", topic)
		return
	}
	token := client.Publish(topic, 1, false, msg)
	token.Wait()
	if token.Error() != nil {
		log.Printf("Publish error: %v", token.Error())
	}
}

func Subscribe(topic string, handler MQTT.MessageHandler) {
	if client == nil || !client.IsConnected() {
		log.Printf("MQTT client not connected; skipping subscribe to %s", topic)
		return
	}
	token := client.Subscribe(topic, 1, handler)
	token.Wait()
	if token.Error() != nil {
		log.Printf("Subscribe error: %v", token.Error())
	} else {
		fmt.Printf("Subscribed to %s\n", topic)
	}
}
