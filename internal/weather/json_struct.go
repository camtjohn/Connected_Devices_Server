package weather

type Current_weather struct {
	Coord struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coord"`
	Weather []struct {
		ID          int    `json:"id"`
		Main        string `json:"main"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Base string `json:"base"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
		Pressure  int     `json:"pressure"`
		Humidity  int     `json:"humidity"`
		SeaLevel  int     `json:"sea_level"`
		GrndLevel int     `json:"grnd_level"`
	} `json:"main"`
	Visibility int `json:"visibility"`
	Wind       struct {
		Speed float64 `json:"speed"`
		Deg   int     `json:"deg"`
		Gust  float64 `json:"gust"`
	} `json:"wind"`
	Clouds struct {
		All int `json:"all"`
	} `json:"clouds"`
	Dt  int `json:"dt"`
	Sys struct {
		Type    int    `json:"type"`
		ID      int    `json:"id"`
		Country string `json:"country"`
		Sunrise int    `json:"sunrise"`
		Sunset  int    `json:"sunset"`
	} `json:"sys"`
	Timezone int    `json:"timezone"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Cod      int    `json:"cod"`
}

type Forecast_weather struct {
	CityName    string `json:"city_name"`
	CountryCode string `json:"country_code"`
	Data        []struct {
		AppMaxTemp        float64 `json:"app_max_temp"`
		AppMinTemp        float64 `json:"app_min_temp"`
		Clouds            int     `json:"clouds"`
		CloudsHi          int     `json:"clouds_hi"`
		CloudsLow         int     `json:"clouds_low"`
		CloudsMid         int     `json:"clouds_mid"`
		Datetime          string  `json:"datetime"`
		Dewpt             float64 `json:"dewpt"`
		HighTemp          float64 `json:"high_temp"`
		LowTemp           float64 `json:"low_temp"`
		MaxDhi            any     `json:"max_dhi"`
		MaxTemp           float64 `json:"max_temp"`
		MinTemp           float64 `json:"min_temp"`
		MoonPhase         float64 `json:"moon_phase"`
		MoonPhaseLunation float64 `json:"moon_phase_lunation"`
		MoonriseTs        int     `json:"moonrise_ts"`
		MoonsetTs         int     `json:"moonset_ts"`
		Ozone             int     `json:"ozone"`
		Pop               int     `json:"pop"`
		Precip            float64 `json:"precip"`
		Pres              int     `json:"pres"`
		Rh                int     `json:"rh"`
		Slp               int     `json:"slp"`
		Snow              float64 `json:"snow"`
		SnowDepth         float64 `json:"snow_depth"`
		SunriseTs         int     `json:"sunrise_ts"`
		SunsetTs          int     `json:"sunset_ts"`
		Temp              float64 `json:"temp"`
		Ts                int     `json:"ts"`
		Uv                int     `json:"uv"`
		ValidDate         string  `json:"valid_date"`
		Vis               float64 `json:"vis"`
		Weather           struct {
			Code        int    `json:"code"`
			Icon        string `json:"icon"`
			Description string `json:"description"`
		} `json:"weather"`
		WindCdir     string  `json:"wind_cdir"`
		WindCdirFull string  `json:"wind_cdir_full"`
		WindDir      int     `json:"wind_dir"`
		WindGustSpd  float64 `json:"wind_gust_spd"`
		WindSpd      float64 `json:"wind_spd"`
	} `json:"data"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	StateCode string  `json:"state_code"`
	Timezone  string  `json:"timezone"`
}
