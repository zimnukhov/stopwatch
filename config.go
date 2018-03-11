package main

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/BurntSushi/toml"
)

// Config is the main configuration type for the whole app
type Config struct {
	Stopwatch *StopwatchConfig `toml:"stopwatch"`
	DB        *DBConfig        `toml:"db"`
	HTTP      *HTTPConfig      `toml:"http"`
}

// StopwatchConfig is part of config related to the app itself
// time settings and logging go here
type StopwatchConfig struct {
	DayStartHour         int    `toml:"day_start_hour"`
	Log                  string `toml:"log"`
	DisplayNotifications bool   `toml:"display_notifications"` // display os x notifcations via osascript
}

// DBConfig is MySQL configuration
type DBConfig struct {
	User     string `toml:"user"`
	Password string `toml:"password"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Database string `toml:"database"`
}

// HTTPConfig is config of HTTP server
type HTTPConfig struct {
	Port       int    `toml:"port"`
	StaticDir  string `toml:"static_dir"`
	HrefPrefix string `toml:"href_prefix"` // prefix of stopwatch urls (e.g. if stopwatch is behind a proxy)
}

// NewConfig creates a new Config instance with default values
func NewConfig() *Config {
	return &Config{
		Stopwatch: &StopwatchConfig{
			DayStartHour:         8,
			Log:                  "/usr/local/stopwatch/error.log",
			DisplayNotifications: false,
		},
		DB: &DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Database: "stopwatch",
		},
		HTTP: &HTTPConfig{
			Port:       8080,
			StaticDir:  "/usr/local/stopwatch/ui",
			HrefPrefix: "/stopwatch",
		},
	}
}

// ParseConfig parses TOML file and creates a Config instance from it
func ParseConfig(filePath string) (*Config, error) {
	cfgData, err := ioutil.ReadFile(filePath)

	if err != nil {
		return nil, err
	}

	cfg := NewConfig()
	_, err = toml.Decode(string(cfgData), cfg)

	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// PrintDefaultConfig prints TOML-encoded default config
func PrintDefaultConfig() error {
	cfg := NewConfig()
	buf := new(bytes.Buffer)

	enc := toml.NewEncoder(buf)
	err := enc.Encode(cfg)

	if err != nil {
		return err
	}

	fmt.Println(buf.String())
	return nil
}
