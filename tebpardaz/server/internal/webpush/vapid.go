package webpush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wp "github.com/SherClockHolmes/webpush-go"
)

// Keys جفت کلید VAPID و نشانی تماس (subject) برای امضای اعلان است.
type Keys struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

type storedKeys struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	Subject    string `json:"subject"`
}

// LoadKeys کلید را از محیط، بعد از فایل، و در نهایت با ساخت یک‌باره می‌خواند.
// ورودی: کلید عمومی، کلید خصوصی، subject (مثلاً mailto:) و مسیر فایل پشتیبان.
// خروجی: کلید آماده استفاده، یا خطا اگر ساخت و ذخیره ممکن نباشد.
func LoadKeys(publicKey, privateKey, subject, path string) (*Keys, error) {
	subject = normalizeSubject(subject)
	publicKey = strings.TrimSpace(publicKey)
	privateKey = strings.TrimSpace(privateKey)
	if publicKey != "" && privateKey != "" {
		return &Keys{PublicKey: publicKey, PrivateKey: privateKey, Subject: subject}, nil
	}

	path = strings.TrimSpace(path)
	if path == "" {
		path = filepath.Join("data", "vapid.json")
	}
	if loaded, err := readKeyFile(path); err == nil && loaded.PublicKey != "" && loaded.PrivateKey != "" {
		if strings.TrimSpace(loaded.Subject) != "" {
			subject = normalizeSubject(loaded.Subject)
		}
		return &Keys{PublicKey: loaded.PublicKey, PrivateKey: loaded.PrivateKey, Subject: subject}, nil
	}

	priv, pub, err := wp.GenerateVAPIDKeys()
	if err != nil {
		return nil, fmt.Errorf("generate vapid: %w", err)
	}
	keys := &Keys{PublicKey: pub, PrivateKey: priv, Subject: subject}
	if err := writeKeyFile(path, keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func normalizeSubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "mailto:push@tebpardaz.ir"
	}
	if !strings.Contains(subject, ":") {
		return "mailto:" + subject
	}
	return subject
}

func readKeyFile(path string) (storedKeys, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return storedKeys{}, err
	}
	var stored storedKeys
	if err := json.Unmarshal(raw, &stored); err != nil {
		return storedKeys{}, err
	}
	stored.PublicKey = strings.TrimSpace(stored.PublicKey)
	stored.PrivateKey = strings.TrimSpace(stored.PrivateKey)
	return stored, nil
}

func writeKeyFile(path string, keys *Keys) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("vapid dir: %w", err)
	}
	raw, err := json.MarshalIndent(storedKeys{
		PublicKey:  keys.PublicKey,
		PrivateKey: keys.PrivateKey,
		Subject:    keys.Subject,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("vapid file: %w", err)
	}
	return nil
}
