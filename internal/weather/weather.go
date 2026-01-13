package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"server_app/internal/storage"
	"sync"
	"time"
)

var country_code string = "US"

type WeatherData struct {
	Zipcode                string          `json:"zipcode"`
	CurrentWeather         json.RawMessage `json:"current_weather"`
	ForecastWeather        json.RawMessage `json:"forecast_weather"`
	CurrentWeatherUpdated  string          `json:"current_weather_updated"`
	ForecastWeatherUpdated string          `json:"forecast_weather_updated"`
}

var store *storage.Manager
var mu sync.RWMutex

func InitWeatherStorage(dataFilePath string) error {
	var err error
	store, err = storage.New(dataFilePath)
	if err != nil {
		return fmt.Errorf("failed to initialize weather storage: %v", err)
	}
	fmt.Printf("Initialized weather storage\n")
	return nil
}

// Weather Map api (current weather)
var api_key string = "3836f65abd758ae760af5f75471fe0b1"
var weather_url string = "https://api.openweathermap.org/data/2.5/weather?zip="

// Weather Bit api (forecast weather)
var forecast_api_key string = "a7791992885c4e0bac7f5631377da381"
var forecast_url string = "https://api.weatherbit.io/v2.0/forecast/daily?postal_code="

// Helper function to build URLs for a given zipcode
func buildWeatherUrls(zipcode string) (string, string) {
	zip_string := zipcode + "," + country_code
	url_current := weather_url + zip_string + "&units=imperial" + "&appid=" + api_key
	url_forecast := forecast_url + zip_string + "&units=I&key=" + forecast_api_key
	return url_current, url_forecast
}

// FetchWeatherFromAPI retrieves weather data from the API
func FetchWeatherFromAPI(data_type string, zipcode string) []byte {
	url_current, url_forecast := buildWeatherUrls(zipcode)
	var url string
	if data_type == "current_weather" {
		url = url_current
	} else if data_type == "forecast_weather" {
		url = url_forecast
	}

	if url == "" {
		fmt.Println("Get_weather: empty URL for", data_type)
		return nil
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Get_weather: http.Get error:", err)
		return nil
	}
	if resp == nil || resp.Body == nil {
		fmt.Println("Get_weather: nil response or body")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Println("Get_weather: non-2xx status:", resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Get_weather: ReadAll error:", err)
		return nil
	}

	return body
}

// Store weather data using storage manager
func Store_weather(data_type string, weather_data []byte, zipcode string) {
	if len(weather_data) == 0 {
		fmt.Println("Store_weather: no data to store for", data_type)
		return
	}
	if store == nil {
		fmt.Println("Store_weather: storage not initialized")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var data WeatherData
	if val, exists := store.Get(zipcode); exists {
		jsonBytes, _ := json.Marshal(val)
		json.Unmarshal(jsonBytes, &data)
	}

	data.Zipcode = zipcode
	if data_type == "current_weather" {
		data.CurrentWeather = json.RawMessage(weather_data)
		data.CurrentWeatherUpdated = time.Now().Format(time.RFC3339)
	} else if data_type == "forecast_weather" {
		data.ForecastWeather = json.RawMessage(weather_data)
		data.ForecastWeatherUpdated = time.Now().Format(time.RFC3339)
	}

	if err := store.Set(zipcode, data); err != nil {
		fmt.Println("Store_weather: error storing weather:", err)
	}
}

// GetCurrentWeatherTemp retrieves the current temperature as int8
func GetCurrentWeatherTemp(zipcode string) (int8, error) {
	if store == nil {
		return 0, fmt.Errorf("storage not initialized")
	}

	mu.RLock()
	defer mu.RUnlock()

	val, exists := store.Get(zipcode)
	if !exists {
		return 0, fmt.Errorf("no weather data found for zipcode: %s", zipcode)
	}

	var data WeatherData
	jsonBytes, _ := json.Marshal(val)
	json.Unmarshal(jsonBytes, &data)

	if len(data.CurrentWeather) == 0 {
		return 0, fmt.Errorf("no current weather data for zipcode: %s", zipcode)
	}

	var current_data Current_weather
	if err := json.Unmarshal(data.CurrentWeather, &current_data); err != nil {
		return 0, fmt.Errorf("JSON unmarshal error: %v", err)
	}

	temp := int8(math.Round(current_data.Main.Temp))
	return temp, nil
}

// ForecastDay represents a single day forecast for the protocol
type ForecastDay struct {
	HighTemp uint8
	Precip   uint8
	Moon     uint8
}

// GetForecastDays retrieves forecast data as typed values for the protocol
func GetForecastDays(zipcode string, numDays int) ([]ForecastDay, error) {
	if store == nil {
		return nil, fmt.Errorf("storage not initialized")
	}

	mu.RLock()
	defer mu.RUnlock()

	val, exists := store.Get(zipcode)
	if !exists {
		return nil, fmt.Errorf("no weather data found for zipcode: %s", zipcode)
	}

	var data WeatherData
	jsonBytes, _ := json.Marshal(val)
	json.Unmarshal(jsonBytes, &data)

	if len(data.ForecastWeather) == 0 {
		return nil, fmt.Errorf("no forecast data for zipcode: %s", zipcode)
	}

	var forecast_data Forecast_weather
	if err := json.Unmarshal(data.ForecastWeather, &forecast_data); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %v", err)
	}

	if len(forecast_data.Data) < numDays {
		numDays = len(forecast_data.Data)
	}

	days := make([]ForecastDay, numDays)
	for i := 0; i < numDays; i++ {
		forecastDay := forecast_data.Data[i]

		// HighTemp: convert to uint8, taking absolute value
		highTemp := uint8(math.Round(math.Abs(forecastDay.HighTemp)))

		// Precip: already int, just convert to uint8
		precip := uint8(forecastDay.Pop)

		// Moon: convert phase to 0/1/2 (0=<93%, 1=93-99%, 2=100%)
		var moon uint8
		if forecastDay.MoonPhase == 1.0 {
			moon = 2
		} else if forecastDay.MoonPhase > 0.93 {
			moon = 1
		} else {
			moon = 0
		}

		days[i] = ForecastDay{
			HighTemp: highTemp,
			Precip:   precip,
			Moon:     moon,
		}
	}

	return days, nil
}

// GetStoredWeatherData retrieves the full weather data struct for a zipcode from storage
func GetStoredWeatherData(zipcode string) (WeatherData, bool) {
	if store == nil {
		return WeatherData{}, false
	}

	mu.RLock()
	defer mu.RUnlock()

	if val, exists := store.Get(zipcode); exists {
		var data WeatherData
		jsonBytes, _ := json.Marshal(val)
		json.Unmarshal(jsonBytes, &data)
		return data, true
	}

	return WeatherData{}, false
}
