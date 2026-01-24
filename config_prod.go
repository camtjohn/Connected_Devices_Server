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
	// Etch Sketch shared canvas topic
	TopicEtchSketch = "etch_sketch"
	IsDebugBuild    = false

	// Weather timing (in minutes)
	WeatherUpdateInterval  = 30  // Fetch current weather every 30 minutes
	WeatherValidityPeriod  = 35  // Consider weather valid if updated within 35 minutes
	ForecastUpdateInterval = 360 // Fetch forecast every 6 hours (12 * 30min)
	ForecastValidityPeriod = 370 // Consider forecast valid if updated within ~6 hours
)
