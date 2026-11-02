// Package openmeteo fetches weather from the Open-Meteo API (no API key)
// with geocoding by place name and 15-minute response caching.
package openmeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/cmerk2021/panely/internal/httpx"
)

// CacheTTL is how long weather responses are cached.
const CacheTTL = 15 * time.Minute

// Current conditions.
type Current struct {
	Temperature float64
	FeelsLike   float64
	Humidity    int
	WindSpeed   float64
	Code        int // WMO weather code
	IsDay       bool
}

// Day is one forecast day.
type Day struct {
	Date   time.Time
	Min    float64
	Max    float64
	Code   int
	Precip float64 // probability max, percent
}

// Report is a full weather report for a location.
type Report struct {
	Place   string
	Lat     float64
	Lon     float64
	Units   string // metric | imperial
	Current Current
	Days    []Day
}

// Geocode resolves a place name to coordinates using Open-Meteo geocoding.
func Geocode(ctx context.Context, client *httpx.Client, place string) (lat, lon float64, name string, err error) {
	u := "https://geocoding-api.open-meteo.com/v1/search?count=1&name=" + url.QueryEscape(place)
	body, err := client.GetCached(ctx, u, 24*time.Hour)
	if err != nil {
		return 0, 0, "", err
	}
	var res struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Country   string  `json:"country_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return 0, 0, "", fmt.Errorf("geocode: %w", err)
	}
	if len(res.Results) == 0 {
		return 0, 0, "", fmt.Errorf("location %q not found", place)
	}
	r := res.Results[0]
	return r.Latitude, r.Longitude, r.Name, nil
}

// Fetch retrieves current conditions and a 5-day forecast.
func Fetch(ctx context.Context, client *httpx.Client, lat, lon float64, units string) (*Report, error) {
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.4f", lat))
	q.Set("longitude", fmt.Sprintf("%.4f", lon))
	q.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,weather_code,is_day")
	q.Set("daily", "temperature_2m_min,temperature_2m_max,weather_code,precipitation_probability_max")
	q.Set("forecast_days", "5")
	q.Set("timezone", "auto")
	if units == "imperial" {
		q.Set("temperature_unit", "fahrenheit")
		q.Set("wind_speed_unit", "mph")
	}
	u := "https://api.open-meteo.com/v1/forecast?" + q.Encode()
	body, err := client.GetCached(ctx, u, CacheTTL)
	if err != nil {
		return nil, err
	}
	return parseForecast(body, lat, lon, units)
}

func parseForecast(body []byte, lat, lon float64, units string) (*Report, error) {
	var res struct {
		Current struct {
			Temp     float64 `json:"temperature_2m"`
			Feels    float64 `json:"apparent_temperature"`
			Humidity int     `json:"relative_humidity_2m"`
			Wind     float64 `json:"wind_speed_10m"`
			Code     int     `json:"weather_code"`
			IsDay    int     `json:"is_day"`
		} `json:"current"`
		Daily struct {
			Time   []string  `json:"time"`
			Min    []float64 `json:"temperature_2m_min"`
			Max    []float64 `json:"temperature_2m_max"`
			Code   []int     `json:"weather_code"`
			Precip []float64 `json:"precipitation_probability_max"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("weather: %w", err)
	}
	rep := &Report{
		Lat: lat, Lon: lon, Units: units,
		Current: Current{
			Temperature: res.Current.Temp,
			FeelsLike:   res.Current.Feels,
			Humidity:    res.Current.Humidity,
			WindSpeed:   res.Current.Wind,
			Code:        res.Current.Code,
			IsDay:       res.Current.IsDay == 1,
		},
	}
	for i := range res.Daily.Time {
		d, err := time.Parse("2006-01-02", res.Daily.Time[i])
		if err != nil {
			continue
		}
		day := Day{Date: d}
		if i < len(res.Daily.Min) {
			day.Min = res.Daily.Min[i]
		}
		if i < len(res.Daily.Max) {
			day.Max = res.Daily.Max[i]
		}
		if i < len(res.Daily.Code) {
			day.Code = res.Daily.Code[i]
		}
		if i < len(res.Daily.Precip) {
			day.Precip = res.Daily.Precip[i]
		}
		rep.Days = append(rep.Days, day)
	}
	return rep, nil
}

// Icon maps a WMO weather code to a unicode icon.
func Icon(code int, isDay bool) string {
	switch {
	case code == 0:
		if isDay {
			return "☀"
		}
		return "🌙"
	case code <= 2:
		return "⛅"
	case code == 3:
		return "☁"
	case code == 45 || code == 48:
		return "🌫"
	case code >= 51 && code <= 57:
		return "🌦"
	case code >= 61 && code <= 67:
		return "🌧"
	case code >= 71 && code <= 77:
		return "❄"
	case code >= 80 && code <= 82:
		return "🌧"
	case code == 85 || code == 86:
		return "🌨"
	case code >= 95:
		return "⛈"
	default:
		return "·"
	}
}

// Describe maps a WMO weather code to a short description.
func Describe(code int) string {
	switch {
	case code == 0:
		return "Clear"
	case code == 1:
		return "Mostly clear"
	case code == 2:
		return "Partly cloudy"
	case code == 3:
		return "Overcast"
	case code == 45 || code == 48:
		return "Fog"
	case code >= 51 && code <= 57:
		return "Drizzle"
	case code >= 61 && code <= 67:
		return "Rain"
	case code >= 71 && code <= 77:
		return "Snow"
	case code >= 80 && code <= 82:
		return "Showers"
	case code == 85 || code == 86:
		return "Snow showers"
	case code == 95:
		return "Thunderstorm"
	case code == 96 || code == 99:
		return "Storm + hail"
	default:
		return "Unknown"
	}
}
