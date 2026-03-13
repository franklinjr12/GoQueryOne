package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const CurrentSchemaVersion = 1

type Config struct {
	SchemaVersion int                 `json:"schemaVersion"`
	App           AppConfig           `json:"app"`
	Query         QueryConfig         `json:"query"`
	Connections   []ConnectionProfile `json:"connections"`
}

type AppConfig struct {
	Language string        `json:"language"`
	Theme    string        `json:"theme"`
	Window   WindowConfig  `json:"window"`
	Logging  LoggingConfig `json:"logging"`
}

type WindowConfig struct {
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
	X      float32 `json:"x"`
	Y      float32 `json:"y"`
}

type LoggingConfig struct {
	Level   string `json:"level"`
	File    string `json:"file"`
	MaxMiB  int    `json:"maxMiB"`
	Enabled bool   `json:"enabled"`
}

type QueryConfig struct {
	DefaultTimeoutMs    int  `json:"defaultTimeoutMs"`
	DefaultMaxRows      int  `json:"defaultMaxRows"`
	FetchPageSize       int  `json:"fetchPageSize"`
	StopOnErrorInScript bool `json:"stopOnErrorInScript"`
}

type ConnectionProfile struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	DSN              string            `json:"dsn,omitempty"`
	Driver           string            `json:"driver,omitempty"`
	ConnectionString string            `json:"connectionString,omitempty"`
	FilePath         string            `json:"filePath,omitempty"`
	Username         string            `json:"username,omitempty"`
	CredentialRef    string            `json:"credentialRef,omitempty"`
	SavePassword     bool              `json:"savePassword"`
	Options          ConnectionOptions `json:"options"`
}

type ConnectionOptions struct {
	LoginTimeoutMs int    `json:"loginTimeoutMs"`
	LimitSyntax    string `json:"limitSyntax"`
}

type legacyConfig struct {
	Database struct {
		DSN      string `json:"dsn"`
		Username string `json:"username"`
		Password string `json:"password"`
		Timeout  string `json:"timeout"`
	} `json:"database"`
	App struct {
		LogLevel     string `json:"log_level"`
		QueryTimeout string `json:"query_timeout"`
		MaxRows      int    `json:"max_rows"`
	} `json:"app"`
}

func DefaultConfig() *Config {
	return &Config{
		SchemaVersion: CurrentSchemaVersion,
		App: AppConfig{
			Language: "en-US",
			Theme:    "system",
			Window: WindowConfig{
				Width:  1260,
				Height: 760,
				X:      40,
				Y:      40,
			},
			Logging: LoggingConfig{
				Level:   "info",
				File:    "logs.txt",
				MaxMiB:  20,
				Enabled: true,
			},
		},
		Query: QueryConfig{
			DefaultTimeoutMs:    60000,
			DefaultMaxRows:      10000,
			FetchPageSize:       500,
			StopOnErrorInScript: true,
		},
		Connections: []ConnectionProfile{},
	}
}

func ResolveConfigPath() string {
	if explicit := strings.TrimSpace(os.Getenv("GOQUERYONE_CONFIG")); explicit != "" {
		return explicit
	}
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	exeDir := filepath.Dir(exe)
	portable := filepath.Join(exeDir, "config.json")
	if _, statErr := os.Stat(filepath.Join(exeDir, "portable.flag")); statErr == nil {
		return portable
	}
	if writableDir(exeDir) {
		return portable
	}
	userDir, userErr := os.UserConfigDir()
	if userErr != nil {
		return "config.json"
	}
	targetDir := filepath.Join(userDir, "GoQueryOne")
	_ = os.MkdirAll(targetDir, 0o755)
	return filepath.Join(targetDir, "config.json")
}

func writableDir(dir string) bool {
	if dir == "" {
		return false
	}
	testPath := filepath.Join(dir, ".goqueryone_write_test")
	if err := os.WriteFile(testPath, []byte("ok"), 0o644); err != nil {
		return false
	}
	_ = os.Remove(testPath)
	return true
}

func LoadConfig(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("config path cannot be empty")
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg := DefaultConfig()
		if saveErr := SaveConfig(cfg, path); saveErr != nil {
			return cfg, nil
		}
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	var probe map[string]any
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("invalid config JSON %s: %w", path, err)
	}
	if _, ok := probe["schemaVersion"]; !ok {
		migrated, err := migrateLegacy(path, data)
		if err != nil {
			return nil, err
		}
		return migrated, nil
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config %s: %w", path, err)
	}
	if cfg.SchemaVersion < CurrentSchemaVersion {
		cfg.SchemaVersion = CurrentSchemaVersion
		if err := SaveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to persist migrated config: %w", err)
		}
	}
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func migrateLegacy(path string, data []byte) (*Config, error) {
	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("failed to decode legacy config: %w", err)
	}
	cfg := DefaultConfig()
	if legacy.App.LogLevel != "" {
		cfg.App.Logging.Level = legacy.App.LogLevel
	}
	if legacy.App.QueryTimeout != "" {
		if parsed, parseErr := time.ParseDuration(legacy.App.QueryTimeout); parseErr == nil && parsed > 0 {
			cfg.Query.DefaultTimeoutMs = int(parsed / time.Millisecond)
		}
	}
	if legacy.App.MaxRows > 0 {
		cfg.Query.DefaultMaxRows = legacy.App.MaxRows
	}
	if legacy.Database.DSN != "" || legacy.Database.Username != "" {
		timeout := 30000
		if legacy.Database.Timeout != "" {
			if parsed, parseErr := time.ParseDuration(legacy.Database.Timeout); parseErr == nil && parsed > 0 {
				timeout = int(parsed / time.Millisecond)
			}
		}
		cfg.Connections = append(cfg.Connections, ConnectionProfile{
			ID:           "legacy-default",
			Name:         "Legacy Connection",
			Type:         "dsn",
			DSN:          legacy.Database.DSN,
			Username:     legacy.Database.Username,
			SavePassword: false,
			Options: ConnectionOptions{
				LoginTimeoutMs: timeout,
				LimitSyntax:    "TOP",
			},
		})
	}
	backup := path + ".legacy.bak"
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return nil, fmt.Errorf("failed to create config backup %s: %w", backup, err)
	}
	if err := SaveConfig(cfg, path); err != nil {
		return nil, err
	}
	return cfg, nil
}

func SaveConfig(config *Config, path string) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}
	config.SchemaVersion = CurrentSchemaVersion
	if err := ValidateConfig(config); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to ensure config directory: %w", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config %s: %w", path, err)
	}
	return nil
}

func ValidateConfig(config *Config) error {
	if config == nil {
		return errors.New("config is required")
	}
	if config.Query.DefaultTimeoutMs <= 0 {
		return errors.New("query.defaultTimeoutMs must be > 0")
	}
	if config.Query.DefaultMaxRows <= 0 {
		return errors.New("query.defaultMaxRows must be > 0")
	}
	if config.Query.FetchPageSize <= 0 {
		return errors.New("query.fetchPageSize must be > 0")
	}
	for i := range config.Connections {
		c := config.Connections[i]
		if strings.TrimSpace(c.ID) == "" {
			return fmt.Errorf("connections[%d].id is required", i)
		}
		if strings.TrimSpace(c.Name) == "" {
			return fmt.Errorf("connections[%d].name is required", i)
		}
		if strings.TrimSpace(c.Type) == "" {
			return fmt.Errorf("connections[%d].type is required", i)
		}
	}
	return nil
}

func (c *Config) UpsertConnection(profile ConnectionProfile) {
	for i := range c.Connections {
		if c.Connections[i].ID == profile.ID {
			c.Connections[i] = profile
			return
		}
	}
	c.Connections = append(c.Connections, profile)
}

func (c *Config) RemoveConnection(profileID string) {
	filtered := make([]ConnectionProfile, 0, len(c.Connections))
	for _, existing := range c.Connections {
		if existing.ID != profileID {
			filtered = append(filtered, existing)
		}
	}
	c.Connections = filtered
}

func (c *Config) ConnectionByID(profileID string) (ConnectionProfile, bool) {
	for _, profile := range c.Connections {
		if profile.ID == profileID {
			return profile, true
		}
	}
	return ConnectionProfile{}, false
}
