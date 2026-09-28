package config

import (
	"bufio"
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

	// Messaging API (OTP + booking confirmation SMS).
	MessagingBaseURL   string
	MessagingAuthToken string // optional static fallback
	MessagingAESKey    string // base64, 32 bytes decoded
	MessagingHMACKey   string // base64
	MessagingUserCode  string // user_code in auth payload
	MessagingMessenger string // SMS | Bale | BaleANDSMS | BaleORSMS
	MessagingOperator  string // e.g. MCI

	// FileStorageRoot is the shared attachment directory (same value as SmsService FILE_STORAGE_ROOT).
	FileStorageRoot string

	// TrustedProxies is a comma-separated list of proxy IPs or CIDRs Gin may trust.
	TrustedProxies string
	// VAPID keys for Web Push. Empty values are loaded from VAPIDKeysFile or generated once.
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
	VAPIDKeysFile   string
}

// Load reads configuration from environment variables.
// Inputs: none (reads process environment).
// Output: Config with defaults filled for missing optional values.
func Load() (*Config, error) {
	loadLocalEnv(".env")
	cfg := &Config{
		HTTPAddr:           envOr("HTTP_ADDR", ":8080"),
		ManagementDSN:      os.Getenv("MANAGEMENT_DSN"),
		AppointmentDSN:     os.Getenv("APPOINTMENT_DSN"),
		WSAuthSecret:       os.Getenv("WS_AUTH_SECRET"),
		SessionSecret:      envOr("SESSION_SECRET", "dev-session-secret-change-me"),
		BaseDomain:         envOr("BASE_DOMAIN", "tebpardaz.ir"),
		SuperAdminUser:     envOr("SUPERADMIN_USERNAME", "superadmin"),
		SuperAdminPass:     envOr("SUPERADMIN_PASSWORD", "SuperAdmin@123"),
		MessagingBaseURL:   envOr("MESSAGING_BASE_URL", "http://localhost:8030"),
		MessagingAuthToken: os.Getenv("MESSAGING_AUTH_TOKEN"),
		MessagingAESKey:    envOr("MESSAGING_AES_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="),
		MessagingHMACKey:   envOr("MESSAGING_HMAC_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="),
		MessagingUserCode:  envOr("MESSAGING_USER_CODE", "5"),
		MessagingMessenger: envOr("MESSAGING_MESSENGER", "SMS"),
		MessagingOperator:  envOr("MESSAGING_OPERATOR", "MCI"),
		FileStorageRoot:    os.Getenv("FILE_STORAGE_ROOT"),
		TrustedProxies:     envOr("TRUSTED_PROXIES", "127.0.0.1,::1"),
		VAPIDPublicKey:     os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:    os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:       envOr("VAPID_SUBJECT", "mailto:push@tebpardaz.ir"),
		VAPIDKeysFile:      envOr("VAPID_KEYS_FILE", "data/vapid.json"),
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

// loadLocalEnv copies KEY=VALUE lines from path into the process environment.
// Inputs: path to an optional dotenv file. Output: none. Existing variables are left unchanged.
func loadLocalEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

// ProxyList splits TrustedProxies into the list Gin should trust.
// Inputs: none (uses Config.TrustedProxies). Output: at least the loopback addresses.
func (c *Config) ProxyList() []string {
	fallback := []string{"127.0.0.1", "::1"}
	if c == nil {
		return fallback
	}
	parts := strings.Split(c.TrustedProxies, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

// envOr returns the environment value for key, or fallback when empty.
func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
