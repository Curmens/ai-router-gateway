package config

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Providers ProvidersConfig `mapstructure:"providers"`
	Routing   RoutingConfig   `mapstructure:"routing"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
}

type ServerConfig struct {
	Port    int            `mapstructure:"port"`
	APIKeys []APIKeyConfig `mapstructure:"api_keys"`
}

type APIKeyConfig struct {
	Key       string `mapstructure:"key"`
	Role      string `mapstructure:"role"`
	RateLimit int    `mapstructure:"rate_limit"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type ProvidersConfig struct {
	OpenAI       OpenAIConfig `mapstructure:"openai"`
	Gemini       GeminiConfig `mapstructure:"gemini"`
	Ollama       OllamaConfig `mapstructure:"ollama"`
	Subscription ClaudeConfig `mapstructure:"subscription"`
	Agy          AgyConfig    `mapstructure:"agy"`
}

type LiteLLMConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	APIKey    string        `mapstructure:"api_key"`
	BaseURL   string        `mapstructure:"base_url"`
	Models    []ModelConfig `mapstructure:"models"`
}

type AgyConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	BinaryPath string        `mapstructure:"binary_path"`
	Models     []ModelConfig `mapstructure:"models"`
}

type ClaudeConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	BinaryPath string        `mapstructure:"binary_path"`
	Models     []ModelConfig `mapstructure:"models"`
}

type OpenAIConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	APIKey    string        `mapstructure:"api_key"`
	BaseURL   string        `mapstructure:"base_url"`
	Models    []ModelConfig `mapstructure:"models"`
}

type GeminiConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	APIKey    string        `mapstructure:"api_key"`
	BaseURL   string        `mapstructure:"base_url"`
	Models    []ModelConfig `mapstructure:"models"`
}

type OllamaConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	BaseURL         string        `mapstructure:"base_url"`
	ClassifierModel string        `mapstructure:"classifier_model"`
	Models          []ModelConfig `mapstructure:"models"`
}

type ModelConfig struct {
	Name                string  `mapstructure:"name"`
	CostPer1kPrompt     float64 `mapstructure:"cost_per_1k_prompt"`
	CostPer1kCompletion float64 `mapstructure:"cost_per_1k_completion"`
}

type RoutingConfig struct {
	DefaultModel string         `mapstructure:"default_model"`
	AutoRouting  AutoRouting    `mapstructure:"auto_routing"`
	Failover     FailoverConfig `mapstructure:"failover"`
	GraphPath    string         `mapstructure:"graph_path"`
}

type AutoRouting struct {
	Enabled             bool    `mapstructure:"enabled"`
	ClassifierThreshold float64 `mapstructure:"classifier_threshold"`
}

type FailoverConfig struct {
	MaxRetries     int                  `mapstructure:"max_retries"`
	RetryBackoffMs int                  `mapstructure:"retry_backoff_ms"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
	Chains         map[string][]string  `mapstructure:"chains"`
}

type CircuitBreakerConfig struct {
	MaxFailures    int `mapstructure:"max_failures"`
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
}

type TelemetryConfig struct {
	Prometheus    PrometheusConfig    `mapstructure:"prometheus"`
	OpenTelemetry OpenTelemetryConfig `mapstructure:"opentelemetry"`
}

type PrometheusConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type OpenTelemetryConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	CollectorURL string `mapstructure:"collector_url"`
}

var ActiveConfig *Config

func LoadConfig(configPath string) (*Config, error) {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	ActiveConfig = &cfg

	viper.OnConfigChange(func(e fsnotify.Event) {
		var newCfg Config
		if err := viper.Unmarshal(&newCfg); err == nil {
			ActiveConfig = &newCfg
		}
	})
	viper.WatchConfig()

	return &cfg, nil
}
