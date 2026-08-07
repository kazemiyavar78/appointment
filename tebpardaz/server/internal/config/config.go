package config

import (
	"os"
	"strings"
)

// Config holds server runtime settings loaded from env.
type Config struct {
	HTTPAddr string

	// ManagementDSN connects to tp_managment (Clinic / Organization / City — no AutoMigrate).
	ManagementDSN string
	// AppointmentDSN connects to appointment_tapesh (migrated models).
	AppointmentDSN string

	WSAuthSecret   string
	SessionSecret  string
	BaseDomain     string
	SuperAdminUser string
	SuperAdminPass string
}

// Load reads configuration from environment variables.
// Inputs: none (reads process environment).
// Output: Config with defaults filled for missing optional values.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:       envOr("HTTP_ADDR", ":8080"),
		ManagementDSN:  os.Getenv("MANAGEMENT_DSN"),
		AppointmentDSN: os.Getenv("APPOINTMENT_DSN"),
		WSAuthSecret:   os.Getenv("WS_AUTH_SECRET"),
		SessionSecret:  envOr("SESSION_SECRET", "dev-session-secret-change-me"),
		BaseDomain:     envOr("BASE_DOMAIN", "localhost"),
		SuperAdminUser: envOr("SUPERADMIN_USERNAME", "superadmin"),
		SuperAdminPass: envOr("SUPERADMIN_PASSWORD", "SuperAdmin@123"),
	}
	// Backward-compatible single DSN: treat as appointment DB when dedicated vars are empty.
	if cfg.AppointmentDSN == "" {
		cfg.AppointmentDSN = "Server=tcp:192.168.1.100\\Tapsh;Initial Catalog=appointment_tapesh;User ID=sa;Password=TapeshSrv14@4;TrustServerCertificate=True;"
	}
	if cfg.ManagementDSN == "" {
		cfg.ManagementDSN = "Server=tcp:192.168.1.100\\Tapsh;Initial Catalog=tp_managment;User ID=sa;Password=TapeshSrv14@4;TrustServerCertificate=True;"
	}
	return cfg, nil
}

// envOr returns the environment value for key, or fallback when empty.
func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
