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

	// Weather timing (in minutes)
	WeatherUpdateInterval  = 30  // Fetch current weather every 30 minutes
	WeatherValidityPeriod  = 35  // Consider weather valid if updated within 35 minutes
	ForecastUpdateInterval = 360 // Fetch forecast every 6 hours (12 * 30min)
	ForecastValidityPeriod = 370 // Consider forecast valid if updated within ~6 hours
)
