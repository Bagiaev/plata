package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	ExchangeAPIBaseURL string
	ExchangeAPITimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		ExchangeAPIBaseURL: env("EXCHANGE_API_BASE_URL", "https://api.frankfurter.dev/v2"),
	}

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	apiURL, err := url.Parse(cfg.ExchangeAPIBaseURL)
	if err != nil || apiURL.Hostname() == "" || apiURL.User != nil ||
		apiURL.RawQuery != "" || apiURL.ForceQuery || apiURL.Fragment != "" ||
		(apiURL.Scheme != "http" && apiURL.Scheme != "https") {
		return Config{}, fmt.Errorf("EXCHANGE_API_BASE_URL must be an HTTP URL without credentials, query or fragment")
	}

	cfg.ExchangeAPITimeout, err = time.ParseDuration(env("EXCHANGE_API_TIMEOUT", "5s"))
	if err != nil || cfg.ExchangeAPITimeout <= 0 {
		return Config{}, fmt.Errorf("EXCHANGE_API_TIMEOUT must be a positive duration")
	}

	return cfg, nil
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}

	return fallback
}
