package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the ~/.asr-claw/config.yaml structure.
type Config struct {
	Default  DefaultConfig            `yaml:"default"`
	Engines  map[string]EngineConfig  `yaml:"engines"`
	VAD      VADConfig                `yaml:"vad"`
}

// DefaultConfig holds global defaults.
type DefaultConfig struct {
	Engine string `yaml:"engine"`
	Lang   string `yaml:"lang"`
	Format string `yaml:"format"`
}

// EngineConfig holds per-engine settings.
type EngineConfig struct {
	// CLI engines
	Binary    string `yaml:"binary,omitempty"`
	ModelPath string `yaml:"model_path,omitempty"`
	Model     string `yaml:"model,omitempty"`
	Threads   int    `yaml:"threads,omitempty"`

	// Service engines
	Endpoint  string `yaml:"endpoint,omitempty"`
	ModelName string `yaml:"model_name,omitempty"`

	// Cloud engines
	APIKey  string `yaml:"api_key,omitempty"`
	AppID   string `yaml:"app_id,omitempty"`
	Cluster string `yaml:"cluster,omitempty"`
	BaseURL string `yaml:"base_url,omitempty"`
	Tier    string `yaml:"tier,omitempty"`
}

// VADConfig holds VAD parameters.
type VADConfig struct {
	SilenceThreshold  float64 `yaml:"silence_threshold"`
	SilenceDurationMs int     `yaml:"silence_duration_ms"`
	MaxSegmentSec     int     `yaml:"max_segment_sec"`
	MinSegmentMs      int     `yaml:"min_segment_ms"`
}

// BaseDir returns the asr-claw config directory path.
func BaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".asr-claw")
}

// EnsureDirs creates all standard directories under ~/.asr-claw/.
func EnsureDirs() error {
	base := BaseDir()
	for _, dir := range []string{
		filepath.Join(base, "bin"),
		filepath.Join(base, "models"),
		filepath.Join(base, "cache", "segments"),
		filepath.Join(base, "run"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// Load reads config from ~/.asr-claw/config.yaml.
// Returns default config if the file doesn't exist.
func Load() *Config {
	cfg := &Config{
		Default: DefaultConfig{
			Engine: "",
			Lang:   "zh",
			Format: "json",
		},
	}

	path := filepath.Join(BaseDir(), "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	yaml.Unmarshal(data, cfg)

	// Expand environment variables in all string fields
	cfg.expandEnvVars()

	return cfg
}

// Save writes config to ~/.asr-claw/config.yaml.
func Save(cfg *Config) error {
	EnsureDirs()
	path := filepath.Join(BaseDir(), "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetEngine returns config for a specific engine, with defaults applied.
func (c *Config) GetEngine(name string) EngineConfig {
	if c.Engines != nil {
		if ec, ok := c.Engines[name]; ok {
			return ec
		}
	}
	return EngineConfig{}
}

// SetEngine updates config for a specific engine.
func (c *Config) SetEngine(name string, ec EngineConfig) {
	if c.Engines == nil {
		c.Engines = make(map[string]EngineConfig)
	}
	c.Engines[name] = ec
}

// expandEnvVars expands ${ENV_VAR} references in string fields.
func (c *Config) expandEnvVars() {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	expand := func(s string) string {
		return re.ReplaceAllStringFunc(s, func(match string) string {
			varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
			if val := os.Getenv(varName); val != "" {
				return val
			}
			return match
		})
	}

	for name, ec := range c.Engines {
		ec.APIKey = expand(ec.APIKey)
		ec.Binary = expand(ec.Binary)
		ec.ModelPath = expand(ec.ModelPath)
		ec.Endpoint = expand(ec.Endpoint)
		ec.BaseURL = expand(ec.BaseURL)
		c.Engines[name] = ec
	}
}
