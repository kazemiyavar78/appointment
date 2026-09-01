package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds clinic client runtime settings (usually from YAML).
type Config struct {
	ClinicID                   string   `yaml:"clinic_id"`
	ClinicName                 string   `yaml:"clinic_name"`
	ServerWSURL                string   `yaml:"server_ws_url"`
	AuthToken                  string   `yaml:"auth_token"`
	LocalDBDSN                 string   `yaml:"local_db_dsn"`
	LISBaseURL                 string   `yaml:"lis_base_url"`
	PDFDir                     string   `yaml:"pdf_dir"`                  // پوشه PDF جواب آزمایش: {admission}-{password}.pdf
	DoctorSyncIntervalSec      int      `yaml:"doctor_sync_interval_sec"` // منسوخ؛ از doctor_sync_times استفاده شود
	DoctorSyncTimes            []string `yaml:"doctor_sync_times"`         // ساعت‌های محلی روزانه مثل "08:00"
	AppointmentSyncIntervalSec   int      `yaml:"appointment_sync_interval_sec"`
	WeeklyReserveSyncIntervalSec int      `yaml:"weekly_reserve_sync_interval_sec"` // پیش‌فرض ۹۰۰ = ۱۵ دقیقه
	MonitoringSyncIntervalSec    int      `yaml:"monitoring_sync_interval_sec"`     // پیش‌فرض ۶۰۰ = ۱۰ دقیقه
	HeartbeatIntervalSec         int      `yaml:"heartbeat_interval_sec"`
}

// defaultDoctorSyncTimes زمان‌های پیش‌فرض سینک پزشکان (۳ بار در روز).
var defaultDoctorSyncTimes = []string{"08:00", "14:00", "20:00"}

// LoadFromFile reads clinic config from a YAML path.
// Inputs: path to YAML file.
// Output: validated Config or error.
func LoadFromFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{
		DoctorSyncTimes:              append([]string(nil), defaultDoctorSyncTimes...),
		AppointmentSyncIntervalSec:   60,
		WeeklyReserveSyncIntervalSec: 900,  // ۱۵ دقیقه
		MonitoringSyncIntervalSec:    600,  // ۱۰ دقیقه
		HeartbeatIntervalSec:         30,
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
// Inputs: none (receiver).
// Output: error when required fields are missing.
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
	if len(c.DoctorSyncTimes) == 0 {
		c.DoctorSyncTimes = append([]string(nil), defaultDoctorSyncTimes...)
	}
	for _, t := range c.DoctorSyncTimes {
		if _, err := time.Parse("15:04", strings.TrimSpace(t)); err != nil {
			return fmt.Errorf("doctor_sync_times entry %q must be HH:MM", t)
		}
	}
	if c.AppointmentSyncIntervalSec <= 0 {
		c.AppointmentSyncIntervalSec = 60
	}
	if c.WeeklyReserveSyncIntervalSec <= 0 {
		c.WeeklyReserveSyncIntervalSec = 900
	}
	if c.MonitoringSyncIntervalSec <= 0 {
		c.MonitoringSyncIntervalSec = 600
	}
	if c.HeartbeatIntervalSec <= 0 {
		c.HeartbeatIntervalSec = 30
	}
	return nil
}

// ParsedDoctorSyncTimes returns validated daily sync clock times.
// Inputs: none (receiver).
// Output: slice of time.Time with only hour/minute meaningful (date is arbitrary).
func (c *Config) ParsedDoctorSyncTimes() []time.Time {
	out := make([]time.Time, 0, len(c.DoctorSyncTimes))
	for _, raw := range c.DoctorSyncTimes {
		t, err := time.Parse("15:04", strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		for _, raw := range defaultDoctorSyncTimes {
			t, _ := time.Parse("15:04", raw)
			out = append(out, t)
		}
	}
	return out
}
