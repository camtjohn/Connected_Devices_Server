package weather

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
)

var country_code string = "US"

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

// Helper function to get file paths for a given zipcode
func getWeatherFilePaths(zipcode string) (string, string) {
	json_current := "/home/ubuntu/server_app/internal/weather_data/current_weather_" + zipcode + ".json"
	json_forecast := "/home/ubuntu/server_app/internal/weather_data/forecast_weather_" + zipcode + ".json"
	return json_current, json_forecast
}

// PUBLIC METHODS

func Get_weather(data_type string, zipcode string) []byte {
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Get_weather: ReadAll error:", err)
		return nil
	}

	return body
}

// Store weather data in json file
func Store_weather(data_type string, weather_data []byte, zipcode string) {
	json_current, json_forecast := getWeatherFilePaths(zipcode)
	var json_file string
	if data_type == "current_weather" {
		json_file = json_current
	} else if data_type == "forecast_weather" {
		json_file = json_forecast
	}

	if len(weather_data) == 0 {
		fmt.Println("Store_weather: no data to store for", data_type)
		return
	}

	// Ensure parent directory exists
	dir := filepath.Dir(json_file)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Println("Store_weather: MkdirAll error:", err)
			// continue, try to create file anyway
		}
	}

	// Create json file
	file, err := os.Create(json_file)
	if err != nil {
		fmt.Println("Store_weather: Error creating json file:", err)
		return
	}
	defer file.Close()

	// Write to file
	_, err = file.Write(weather_data)
	if err != nil {
		fmt.Println("Store_weather: Error writing to json file:", err)
	}
}

// Retrieve data from json file
func Read_weather(data_type string, zipcode string) string {
	json_current, json_forecast := getWeatherFilePaths(zipcode)
	var json_file string
	if data_type == "current_weather" {
		json_file = json_current
	} else if data_type == "forecast_weather" {
		json_file = json_forecast
	}

	file, err := os.Open(json_file)
	if err != nil {
		fmt.Println("Read_weather: Error opening json file:", err)
		return ""
	}
	defer file.Close()

	byteValue, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("Read_weather: Error reading json file:", err)
		return ""
	}

	// Assemble string differently for current vs forecast
	var msg_str string

	if data_type == "current_weather" {
		// Assign json data to structure variable
		var current_data Current_weather
		if err := json.Unmarshal(byteValue, &current_data); err != nil {
			fmt.Println("Read_weather: JSON unmarshal error:", err)
			return ""
		}
		temp := math.Abs(current_data.Main.Temp)

		// Convert float temp from struct to string
		msg_str = "0" + fmt.Sprintf("%.0f", temp)

	} else if data_type == "forecast_weather" {
		// Assign json data to structure variable
		var forecast_data Forecast_weather
		if err := json.Unmarshal(byteValue, &forecast_data); err != nil {
			fmt.Println("Read_weather: JSON unmarshal error:", err)
			return ""
		}

		// Convert float values from struct to series of int string
		// Param: data from struct, number of days to report
		msg_str = "1" + assemble_forecast_msg(forecast_data, 3)
	}

	return (msg_str)
}

// PRIVATE METHODS

func assemble_forecast_msg(data Forecast_weather, num_days int) string {
	var forecast_str string
	forecast_str = fmt.Sprintf("%d", num_days)

	// Call assemble_str for each day requesting forecast weather
	for day := 0; day < num_days; day++ {
		forecast_str += assemble_str(data, day)
	}

	return forecast_str
}

// get series of weather values, convert to str, concat
func assemble_str(data Forecast_weather, offset_from_today int) string {
	forecast_data := data.Data[offset_from_today]

	// HighTemp float: translate to 2 digit string
	var high_temp_str string
	high_temp := math.Abs(forecast_data.HighTemp)
	if high_temp < 10.0 {
		high_temp_str = fmt.Sprintf("0%.0f", high_temp)
	} else {
		high_temp_str = fmt.Sprintf("%.0f", high_temp)
	}

	// Snow, Precip int: find max of the two, translate to 2 digit string
	var precip_str string
	precip := forecast_data.Pop
	if precip < 10 {
		precip_str = fmt.Sprintf("0%d", precip)
	} else {
		precip_str = fmt.Sprintf("%d", precip)
	}

	// MoonPhase float: translate to string corresp 100%, 93-99%, below 93%
	var moon_str string
	moon := forecast_data.MoonPhase
	if moon == 1.0 {
		moon_str = "2"
	} else if moon > 0.93 {
		moon_str = "1"
	} else {
		moon_str = "0"
	}

	return (high_temp_str + precip_str + moon_str)
}
