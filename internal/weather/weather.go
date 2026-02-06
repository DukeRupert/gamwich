package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const cacheTTL = 30 * time.Minute

// Config holds weather service configuration from environment variables.
type Config struct {
	Latitude        string
	Longitude       string
	TemperatureUnit string // "fahrenheit" or "celsius"
}

// WeatherData holds the current and daily weather information.
type WeatherData struct {
	CurrentTemp float64
	CurrentCode int
	CurrentDesc string
	CurrentIcon string
	HighTemp    float64
	LowTemp     float64
	Unit        string // "F" or "C"
	Available   bool
	Configured  bool
}

// Service manages weather data fetching and caching.
type Service struct {
	config    Config
	client    *http.Client
	baseURL   string
	mu        sync.RWMutex
	cached    WeatherData
	lastFetch time.Time
}

// NewService creates a new weather service with the given configuration.
func NewService(cfg Config) *Service {
	if cfg.TemperatureUnit == "" {
		cfg.TemperatureUnit = "fahrenheit"
	}
	unit := "F"
	if cfg.TemperatureUnit == "celsius" {
		unit = "C"
	}
	configured := cfg.Latitude != "" && cfg.Longitude != ""
	return &Service{
		config:  cfg,
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api.open-meteo.com/v1/forecast",
		cached: WeatherData{
			Unit:       unit,
			Configured: configured,
		},
	}
}

// GetWeather returns the current weather data, fetching from the API if the cache is stale.
func (s *Service) GetWeather() WeatherData {
	if !s.cached.Configured {
		return s.cached
	}

	s.mu.RLock()
	if time.Since(s.lastFetch) < cacheTTL && s.cached.Available {
		data := s.cached
		s.mu.RUnlock()
		return data
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock.
	if time.Since(s.lastFetch) < cacheTTL && s.cached.Available {
		return s.cached
	}

	data, err := s.fetch()
	if err != nil {
		// Return stale data on error rather than clearing it.
		return s.cached
	}

	s.cached = data
	s.lastFetch = time.Now()
	return s.cached
}

type apiResponse struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		WeatherCode int     `json:"weather_code"`
	} `json:"current"`
	Daily struct {
		TempMax     []float64 `json:"temperature_2m_max"`
		TempMin     []float64 `json:"temperature_2m_min"`
		WeatherCode []int     `json:"weather_code"`
	} `json:"daily"`
}

func (s *Service) fetch() (WeatherData, error) {
	url := fmt.Sprintf(
		"%s?latitude=%s&longitude=%s&current=temperature_2m,weather_code&daily=temperature_2m_max,temperature_2m_min,weather_code&timezone=auto&forecast_days=1&temperature_unit=%s",
		s.baseURL, s.config.Latitude, s.config.Longitude, s.config.TemperatureUnit,
	)

	resp, err := s.client.Get(url)
	if err != nil {
		return WeatherData{}, fmt.Errorf("weather API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WeatherData{}, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return WeatherData{}, fmt.Errorf("decode weather response: %w", err)
	}

	desc, icon := WMOCodeToDescIcon(apiResp.Current.WeatherCode)

	unit := "F"
	if s.config.TemperatureUnit == "celsius" {
		unit = "C"
	}

	data := WeatherData{
		CurrentTemp: apiResp.Current.Temperature,
		CurrentCode: apiResp.Current.WeatherCode,
		CurrentDesc: desc,
		CurrentIcon: icon,
		Unit:        unit,
		Available:   true,
		Configured:  true,
	}

	if len(apiResp.Daily.TempMax) > 0 {
		data.HighTemp = apiResp.Daily.TempMax[0]
	}
	if len(apiResp.Daily.TempMin) > 0 {
		data.LowTemp = apiResp.Daily.TempMin[0]
	}

	return data, nil
}

// WMOCodeToDescIcon maps a WMO weather code to a human-readable description and emoji icon.
func WMOCodeToDescIcon(code int) (string, string) {
	switch code {
	case 0:
		return "Clear sky", "☀️"
	case 1:
		return "Mainly clear", "🌤️"
	case 2:
		return "Partly cloudy", "⛅"
	case 3:
		return "Overcast", "☁️"
	case 45, 48:
		return "Foggy", "🌫️"
	case 51:
		return "Light drizzle", "🌦️"
	case 53:
		return "Moderate drizzle", "🌦️"
	case 55:
		return "Dense drizzle", "🌧️"
	case 56, 57:
		return "Freezing drizzle", "🌧️"
	case 61:
		return "Slight rain", "🌦️"
	case 63:
		return "Moderate rain", "🌧️"
	case 65:
		return "Heavy rain", "🌧️"
	case 66, 67:
		return "Freezing rain", "🌧️"
	case 71:
		return "Slight snow", "🌨️"
	case 73:
		return "Moderate snow", "🌨️"
	case 75:
		return "Heavy snow", "❄️"
	case 77:
		return "Snow grains", "❄️"
	case 80:
		return "Slight showers", "🌦️"
	case 81:
		return "Moderate showers", "🌧️"
	case 82:
		return "Violent showers", "⛈️"
	case 85:
		return "Slight snow showers", "🌨️"
	case 86:
		return "Heavy snow showers", "❄️"
	case 95:
		return "Thunderstorm", "⛈️"
	case 96, 99:
		return "Thunderstorm with hail", "⛈️"
	default:
		return "Unknown", "🌡️"
	}
}
