package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"
)

// Config represents the application configuration
type Config struct {
	Database DatabaseConfig `json:"database"`
	App      AppConfig      `json:"app"`
}

// DatabaseConfig represents database connection configuration
type DatabaseConfig struct {
	DSN      string        `json:"dsn"`
	Username string        `json:"username"`
	Password string        `json:"password"`
	Timeout  time.Duration `json:"timeout"`
}

// AppConfig represents application settings
type AppConfig struct {
	LogLevel     string        `json:"log_level"`
	QueryTimeout time.Duration `json:"query_timeout"`
	MaxRows      int           `json:"max_rows"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			DSN:      "",
			Username: "",
			Password: "",
			Timeout:  30 * time.Second,
		},
		App: AppConfig{
			LogLevel:     "info",
			QueryTimeout: 60 * time.Second,
			MaxRows:      1000,
		},
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Error: failed to open config file %s: %v", filename, err)
		return nil, errors.New("failed to open config file " + filename + ": " + err.Error())
	}
	defer file.Close()

	config := DefaultConfig()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		log.Printf("Error: failed to decode config file %s: %v", filename, err)
		return nil, errors.New("failed to decode config file " + filename + ": " + err.Error())
	}

	return config, nil
}

// SaveConfig saves configuration to a JSON file
func SaveConfig(config *Config, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		log.Printf("Error: failed to create config file %s: %v", filename, err)
		return errors.New("failed to create config file " + filename + ": " + err.Error())
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(config)
	if err != nil {
		log.Printf("Error: failed to encode config to file %s: %v", filename, err)
		return errors.New("failed to encode config to file " + filename + ": " + err.Error())
	}

	return nil
}

// ValidateConfig validates the configuration
func ValidateConfig(config *Config) error {
	if config.Database.DSN == "" {
		log.Printf("Error: database DSN is required")
		return errors.New("database DSN is required")
	}

	if config.Database.Timeout <= 0 {
		log.Printf("Error: database timeout must be positive")
		return errors.New("database timeout must be positive")
	}

	if config.App.QueryTimeout <= 0 {
		log.Printf("Error: query timeout must be positive")
		return errors.New("query timeout must be positive")
	}

	if config.App.MaxRows <= 0 {
		log.Printf("Error: max rows must be positive")
		return errors.New("max rows must be positive")
	}

	return nil
}
