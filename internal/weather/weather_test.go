package weather

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWMOCodeToDescIcon(t *testing.T) {
	tests := []struct {
		code     int
		wantDesc string
		wantIcon string
	}{
		{0, "Clear sky", "☀️"},
		{1, "Mainly clear", "🌤️"},
		{2, "Partly cloudy", "⛅"},
		{3, "Overcast", "☁️"},
		{45, "Foggy", "🌫️"},
		{48, "Foggy", "🌫️"},
		{51, "Light drizzle", "🌦️"},
		{63, "Moderate rain", "🌧️"},
		{75, "Heavy snow", "❄️"},
		{95, "Thunderstorm", "⛈️"},
		{99, "Thunderstorm with hail", "⛈️"},
		{999, "Unknown", "🌡️"},
	}

	for _, tt := range tests {
		desc, icon := WMOCodeToDescIcon(tt.code)
		if desc != tt.wantDesc {
			t.Errorf("WMOCodeToDescIcon(%d) desc = %q, want %q", tt.code, desc, tt.wantDesc)
		}
		if icon != tt.wantIcon {
			t.Errorf("WMOCodeToDescIcon(%d) icon = %q, want %q", tt.code, icon, tt.wantIcon)
		}
	}
}

func TestParseAPIResponse(t *testing.T) {
	payload := `{
		"current": {
			"temperature_2m": 72.5,
			"weather_code": 2
		},
		"daily": {
			"temperature_2m_max": [78.0],
			"temperature_2m_min": [55.0],
			"weather_code": [2]
		}
	}`

	var resp apiResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("failed to parse API response: %v", err)
	}

	if resp.Current.Temperature != 72.5 {
		t.Errorf("current temp = %v, want 72.5", resp.Current.Temperature)
	}
	if resp.Current.WeatherCode != 2 {
		t.Errorf("current weather code = %d, want 2", resp.Current.WeatherCode)
	}
	if len(resp.Daily.TempMax) != 1 || resp.Daily.TempMax[0] != 78.0 {
		t.Errorf("daily temp max = %v, want [78.0]", resp.Daily.TempMax)
	}
	if len(resp.Daily.TempMin) != 1 || resp.Daily.TempMin[0] != 55.0 {
		t.Errorf("daily temp min = %v, want [55.0]", resp.Daily.TempMin)
	}
}

func TestServiceCacheTTL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiResponse{
			Current: struct {
				Temperature float64 `json:"temperature_2m"`
				WeatherCode int     `json:"weather_code"`
			}{Temperature: 70.0, WeatherCode: 0},
			Daily: struct {
				TempMax     []float64 `json:"temperature_2m_max"`
				TempMin     []float64 `json:"temperature_2m_min"`
				WeatherCode []int     `json:"weather_code"`
			}{TempMax: []float64{75.0}, TempMin: []float64{60.0}, WeatherCode: []int{0}},
		})
	}))
	defer server.Close()

	svc := NewService(Config{
		Latitude:        "47.6",
		Longitude:       "-122.3",
		TemperatureUnit: "fahrenheit",
	})

	// Point at a bad URL so fetches fail when cache expires.
	svc.baseURL = "http://127.0.0.1:1"
	svc.mu.Lock()
	svc.cached = WeatherData{
		CurrentTemp: 70.0,
		CurrentCode: 0,
		CurrentDesc: "Clear sky",
		CurrentIcon: "☀️",
		HighTemp:    75.0,
		LowTemp:     60.0,
		Unit:        "F",
		Available:   true,
		Configured:  true,
	}
	svc.lastFetch = time.Now()
	svc.mu.Unlock()

	// Should return cached data (not expired).
	data := svc.GetWeather()
	if !data.Available {
		t.Error("expected weather to be available from cache")
	}
	if data.CurrentTemp != 70.0 {
		t.Errorf("cached temp = %v, want 70.0", data.CurrentTemp)
	}

	// Expire the cache.
	svc.mu.Lock()
	svc.lastFetch = time.Now().Add(-cacheTTL - time.Minute)
	svc.mu.Unlock()

	// Without a valid API endpoint, it should return stale data.
	data = svc.GetWeather()
	if data.CurrentTemp != 70.0 {
		t.Errorf("stale temp = %v, want 70.0 (stale data should be returned on fetch error)", data.CurrentTemp)
	}
}

func TestServiceNotConfigured(t *testing.T) {
	svc := NewService(Config{})

	data := svc.GetWeather()
	if data.Configured {
		t.Error("expected weather to not be configured with empty config")
	}
	if data.Available {
		t.Error("expected weather to not be available with empty config")
	}
}
