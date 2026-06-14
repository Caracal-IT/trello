// Package config loads layered YAML configuration.
//
// Load order:
//  1. config.yaml (base - required)
//  2. config.{APP_ENV}.yaml (env override - optional, defaults to "dev")
//
// Env override values are deep-merged on top of the base, so you only need to
// specify what differs in the env file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration

// Duration wraps time.Duration so it unmarshals from YAML strings like "30s".
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	dur, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", n.Value, err)
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return d.Duration.String(), nil
}

// Config structs

// Config is the root application configuration.
type Config struct {
	App     AppConfig     `yaml:"app"`
	MQTT    MQTTConfig    `yaml:"mqtt"`
	Logging LoggingConfig `yaml:"logging"`
}

// AppConfig holds application-level metadata.
type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"` // overwritten by loader with the active APP_ENV
}

// MQTTConfig holds broker connection settings.
type MQTTConfig struct {
	Broker         string   `yaml:"broker"`
	ClientID       string   `yaml:"clientID"`
	KeepAlive      Duration `yaml:"keepAlive"`
	ConnectTimeout Duration `yaml:"connectTimeout"`
	WriteTimeout   Duration `yaml:"writeTimeout"`
	CleanSession   bool     `yaml:"cleanSession"`
	AutoReconnect  bool     `yaml:"autoReconnect"`
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
}

// LoggingConfig controls the logging pipeline.
type LoggingConfig struct {
	// Level is the global minimum level: debug | info | warn | error | fatal
	Level string `yaml:"level"`

	// Overrides maps source-name prefixes to level strings.
	// The longest matching prefix wins (Serilog-style):
	//   "mqtt"        -> matches "mqtt", "mqtt.client", "mqtt.broker"
	//   "mqtt.client" -> overrides the "mqtt" rule for that specific source
	Overrides map[string]string `yaml:"overrides"`

	Sinks SinksConfig `yaml:"sinks"`
}

// SinksConfig aggregates all sink configurations.
type SinksConfig struct {
	Console       ConsoleSinkConfig       `yaml:"console"`
	Elasticsearch ElasticsearchSinkConfig `yaml:"elasticsearch"`
}

// ConsoleSinkConfig configures the console output sink.
type ConsoleSinkConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Format    string `yaml:"format"`    // "text" | "json"
	Colorized bool   `yaml:"colorized"` // ANSI colours; only meaningful for "text"
}

// ElasticsearchSinkConfig configures the Elasticsearch bulk-indexing sink.
type ElasticsearchSinkConfig struct {
	Enabled       bool     `yaml:"enabled"`
	URL           string   `yaml:"url"`
	Index         string   `yaml:"index"`
	Username      string   `yaml:"username"`
	Password      string   `yaml:"password"`
	BulkSize      int      `yaml:"bulkSize"`
	FlushInterval Duration `yaml:"flushInterval"`
}

// Loader

// Load reads config.yaml from dir and deep-merges config.{APP_ENV}.yaml on top.
// APP_ENV defaults to "dev" when not set.
func Load(dir string) (*Config, error) {
	base, err := readYAML(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("config: base file: %w", err)
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	overridePath := filepath.Join(dir, fmt.Sprintf("config.%s.yaml", env))
	envMap, err := readYAML(overridePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: %s override: %w", env, err)
	}

	merged := deepMerge(base, envMap) // envMap may be nil if file absent

	// Re-encode to YAML so the typed struct's custom unmarshalers fire.
	raw, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("config: re-marshal: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	cfg.App.Env = env
	return &cfg, nil
}

// Helpers

func readYAML(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]any)
	}
	return m, nil
}

// deepMerge returns a new map with src values overlaid on dst.
// Maps are recursed into; all other types use the src value when both exist.
func deepMerge(dst, src map[string]any) map[string]any {
	result := make(map[string]any, len(dst))
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		dstVal, exists := result[k]
		if exists {
			dstMap, dstIsMap := dstVal.(map[string]any)
			srcMap, srcIsMap := v.(map[string]any)
			if dstIsMap && srcIsMap {
				result[k] = deepMerge(dstMap, srcMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}
