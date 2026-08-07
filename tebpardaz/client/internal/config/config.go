package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds clinic client runtime settings (usually from YAML).
type Config struct {
	ClinicID                   string `yaml:"clinic_id"`
	ClinicName                 string `yaml:"clinic_name"`
	ServerWSURL                string `yaml:"server_ws_url"`
	AuthToken                  string `yaml:"auth_token"`
	LocalDBDSN                 string `yaml:"local_db_dsn"`
	LISBaseURL                 string `yaml:"lis_base_url"`
	DoctorSyncIntervalSec      int    `yaml:"doctor_sync_interval_sec"`
	AppointmentSyncIntervalSec int    `yaml:"appointment_sync_interval_sec"` // ثانیه؛ پیش‌فرض ۶۰ (بروزرسانی خودکار نوبت هر ۱ دقیقه)
	HeartbeatIntervalSec       int    `yaml:"heartbeat_interval_sec"`
}

// LoadFromFile reads clinic config from a YAML path.
// Inputs: path to YAML file.
// Output: validated Config or error.
func LoadFromFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{
		DoctorSyncIntervalSec:      300,
		AppointmentSyncIntervalSec: 60, // پیش‌فرض: بروزرسانی خودکار نوبت‌ها هر ۱ دقیقه
		HeartbeatIntervalSec:       30,
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate checks required fields after YAML load.
func (c *Config) validate() error {
	if strings.TrimSpace(c.ServerWSURL) == "" {
		return fmt.Errorf("server_ws_url is required")
	}
	if strings.TrimSpace(c.AuthToken) == "" {
		return fmt.Errorf("auth_token (clinic_key) is required")
	}
	if strings.TrimSpace(c.LocalDBDSN) == "" {
		return fmt.Errorf("local_db_dsn is required")
	}
	if c.DoctorSyncIntervalSec <= 0 {
		c.DoctorSyncIntervalSec = 300
	}
	// فاصله سینک نوبت‌ها؛ حداقل/پیش‌فرض ۱ دقیقه تا کش سرور تازه بماند
	if c.AppointmentSyncIntervalSec <= 0 {
		c.AppointmentSyncIntervalSec = 60
	}
	if c.HeartbeatIntervalSec <= 0 {
		c.HeartbeatIntervalSec = 30
	}
	return nil
}
