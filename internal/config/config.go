// Package config loads and validates the YAML configuration. The schema was
// cleaned up relative to the Python original: unused/typo keys were removed.
// Parsing is tolerant — unknown or legacy keys do not fail the load, but a
// single deprecation warning listing them is emitted at startup.
package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"bodsch.me/mariadb-backup/internal/logging"
	"gopkg.in/yaml.v3"
)

// Compression modes.
const (
	CompressionNone = "none"
	CompressionGzip = "gzip"
)

// Default values, matching the Python read_configuration defaults.
const (
	defaultRotationDaily  = 3
	defaultRotationWeekly = 2
	defaultSMTPPort       = 587
	defaultMySQLPort      = 3306
)

// Config is the top-level configuration.
type Config struct {
	Connection   Connection   `yaml:"connection"`
	Storage      Storage      `yaml:"storage"`
	Notification Notification `yaml:"notification"`
	Excludes     Excludes     `yaml:"excludes"`
}

// Connection holds the database connection parameters.
type Connection struct {
	Username string  `yaml:"username"`
	Password string  `yaml:"password"`
	Host     string  `yaml:"host"`
	Port     flexInt `yaml:"port"`
	Socket   string  `yaml:"socket"`
}

// Storage configures where backups are written and how they are rotated.
type Storage struct {
	Destination string   `yaml:"destination"`
	Compression string   `yaml:"compression"`
	Rotation    Rotation `yaml:"rotation"`
}

// Rotation configures backup retention. dailySet/weeklySet record whether the
// key was present in the YAML so an explicit 0 is preserved instead of being
// treated as "unset" and overwritten with the default.
type Rotation struct {
	Daily     int `yaml:"daily"`
	Weekly    int `yaml:"weekly"`
	dailySet  bool
	weeklySet bool
}

// UnmarshalYAML decodes daily/weekly while recording which keys were present,
// so applyDefaults can distinguish an omitted key from a deliberate 0.
func (r *Rotation) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Daily  *int `yaml:"daily"`
		Weekly *int `yaml:"weekly"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	if raw.Daily != nil {
		r.Daily = *raw.Daily
		r.dailySet = true
	}
	if raw.Weekly != nil {
		r.Weekly = *raw.Weekly
		r.weeklySet = true
	}
	return nil
}

// Notification configures the optional log email.
type Notification struct {
	Enabled   bool   `yaml:"enabled"`
	SMTP      SMTP   `yaml:"smtp"`
	Sender    string `yaml:"sender"`
	Recipient string `yaml:"recipient"`
}

// SMTP configures the mail server.
type SMTP struct {
	ServerName string   `yaml:"server_name"`
	Port       flexInt  `yaml:"port"`
	TLS        bool     `yaml:"tls"`
	Auth       SMTPAuth `yaml:"auth"`
}

// SMTPAuth holds optional SMTP credentials.
type SMTPAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Excludes lists databases and tables to skip.
type Excludes struct {
	Databases []string `yaml:"databases"`
	Tables    []string `yaml:"tables"`
}

// Default returns a Config populated with the same defaults the Python script
// applied before reading the file.
func Default() *Config {
	c := &Config{}
	c.applyDefaults()
	return c
}

func (c *Config) applyDefaults() {
	if !c.Storage.Rotation.dailySet {
		c.Storage.Rotation.Daily = defaultRotationDaily
	}
	if !c.Storage.Rotation.weeklySet {
		c.Storage.Rotation.Weekly = defaultRotationWeekly
	}
	if c.Storage.Compression == "" {
		c.Storage.Compression = CompressionNone
	}
	if c.Notification.SMTP.Port == 0 {
		c.Notification.SMTP.Port = defaultSMTPPort
	}
}

// Compress reports whether dumps should be gzip-compressed according to the
// configuration. A CLI flag may OR this on at the call site.
func (c *Config) Compress() bool {
	return c.Storage.Compression == CompressionGzip
}

// Load reads, parses and validates the configuration file. A missing or empty
// path returns the defaults (matching Python). Unknown/legacy keys are tolerated
// but reported once as a deprecation warning.
func Load(path string, log logging.Logger) (*Config, error) {
	if path == "" {
		log.Debug("no config file given, using defaults")
		return Default(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug("config file %s not found, using defaults", path)
			return Default(), nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if legacy := detectLegacyKeys(data); len(legacy) > 0 {
		log.Warn("%sdeprecated config keys ignored: %s%s",
			logging.ColorWarn, joinKeys(legacy), logging.ColorReset)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	switch c.Storage.Compression {
	case CompressionNone, CompressionGzip:
	default:
		return fmt.Errorf("invalid storage.compression %q (want %q or %q)",
			c.Storage.Compression, CompressionNone, CompressionGzip)
	}
	return nil
}

func joinKeys(keys []string) string {
	sort.Strings(keys)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
}

// flexInt is an int that tolerates being expressed in YAML as an int, a numeric
// string, or an empty string (legacy config used `port: ”`). An empty/invalid
// value decodes to 0, meaning "unset".
type flexInt int

func (f *flexInt) UnmarshalYAML(value *yaml.Node) error {
	s := value.Value
	if s == "" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		// Tolerate junk by treating it as unset rather than failing the load.
		*f = 0
		return nil
	}
	*f = flexInt(n)
	return nil
}
