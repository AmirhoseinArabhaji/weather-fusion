// Package config loads and validates application configuration from env vars.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName    string
	AppEnv     string
	AppPort    string
	AppVersion string

	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	CacheTTL      time.Duration

	OpenWeatherAPIKey  string
	OpenWeatherBaseURL string
	WeatherAPIKey      string
	WeatherAPIBaseURL  string

	GeminiAPIKey    string
	GeminiModel     string
	GeminiMaxTokens int

	CORSAllowedOrigins string

	LogLevel  string
	LogFormat string
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env")

	cfg := &Config{
		AppName:    getEnv("APP_NAME", "weather-fusion"),
		AppEnv:     getEnv("APP_ENV", "development"),
		AppPort:    getEnv("APP_PORT", "8080"),
		AppVersion: getEnv("APP_VERSION", "v0.1.0"),

		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            getEnv("DB_PORT", "5432"),
		DBUser:            getEnv("DB_USER", "weather"),
		DBPassword:        getEnv("DB_PASSWORD", ""),
		DBName:            getEnv("DB_NAME", "weather_fusion"),
		DBSSLMode:         getEnv("DB_SSL_MODE", "disable"),
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME", 300)) * time.Second,

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),
		CacheTTL:      time.Duration(getEnvInt("CACHE_TTL_SECONDS", 300)) * time.Second,

		OpenWeatherAPIKey:  getEnv("OPENWEATHER_API_KEY", ""),
		OpenWeatherBaseURL: getEnv("OPENWEATHER_BASE_URL", "https://api.openweathermap.org/data/2.5"),
		WeatherAPIKey:      getEnv("WEATHERAPI_KEY", ""),
		WeatherAPIBaseURL:  getEnv("WEATHERAPI_BASE_URL", "https://api.weatherapi.com/v1"),

		GeminiAPIKey:    getEnv("GEMINI_API_KEY", ""),
		GeminiModel:     getEnv("GEMINI_MODEL", "gemini-2.0-flash"),
		GeminiMaxTokens: getEnvInt("GEMINI_MAX_TOKENS", 512),

		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"),

		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func (c *Config) IsProd() bool {
	return c.AppEnv == "production"
}

func (c *Config) validate() error {
	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD must not be empty")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}
