package openmeteo

import "testing"

func TestParseForecast(t *testing.T) {
	body := []byte(`{
		"current": {"temperature_2m": 21.5, "apparent_temperature": 20.1,
			"relative_humidity_2m": 55, "wind_speed_10m": 12.3,
			"weather_code": 2, "is_day": 1},
		"daily": {
			"time": ["2024-06-15","2024-06-16"],
			"temperature_2m_min": [12.0, 13.5],
			"temperature_2m_max": [24.0, 26.2],
			"weather_code": [2, 61],
			"precipitation_probability_max": [10, 80]
		}
	}`)
	rep, err := parseForecast(body, 52.52, 13.4, "metric")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Current.Temperature != 21.5 || !rep.Current.IsDay {
		t.Errorf("current: %+v", rep.Current)
	}
	if len(rep.Days) != 2 || rep.Days[1].Precip != 80 {
		t.Errorf("days: %+v", rep.Days)
	}
}

func TestIconAndDescribe(t *testing.T) {
	if Icon(0, true) != "☀" {
		t.Error("clear day icon")
	}
	if Describe(95) != "Thunderstorm" {
		t.Error("thunderstorm describe")
	}
	if Describe(1234) != "Unknown" {
		t.Error("unknown code")
	}
}
