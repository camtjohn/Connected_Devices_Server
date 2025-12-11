//go:build !debug
// +build !debug

package main

// Production configuration
const (
	TopicBootup        = "dev_bootup"
	TopicHeartbeat     = "dev_heartbeat"
	TopicOffline       = "device_offline"
	TopicTest          = "test_msg"
	TopicWeatherPrefix = "weather"
	IsDebugBuild       = false
)
