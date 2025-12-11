//go:build debug
// +build debug

package main

// Debug configuration - prefixes topics to avoid interfering with production
const (
	TopicBootup        = "debug_dev_bootup"
	TopicHeartbeat     = "debug_dev_heartbeat"
	TopicOffline       = "debug_device_offline"
	TopicTest          = "debug_test_msg"
	TopicWeatherPrefix = "debug_weather"
	IsDebugBuild       = true
)
