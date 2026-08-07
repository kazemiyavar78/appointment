package csrf

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// CookieName is the double-submit CSRF cookie for public booking.
	CookieName = "tp_csrf"
	// FormField is the hidden form field that must match the cookie.
	FormField = "csrf_token"
	cookieTTL = 12 * time.Hour
)

// Manager issues and verifies HMAC-bound CSRF tokens for public forms.
type Manager struct {
	secret []byte
}

// NewManager constructs a CSRF Manager.
// Inputs: secret used to sign tokens (typically SESSION_SECRET).
// Output: pointer to Manager.
func NewManager(secret string) *Manager {
	if strings.TrimSpace(secret) == "" {
		secret = "dev-csrf-secret"
	}
	return &Manager{secret: []byte(secret)}
}

// Issue creates a new token, sets the CSRF cookie, and returns the token for the form.
// Inputs: HTTP response writer.
// Output: token string or error when entropy/signing fails.
func (m *Manager) Issue(w http.ResponseWriter) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(raw)
	exp := time.Now().Add(cookieTTL).Unix()
	token := m.sign(nonce, exp)

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(cookieTTL.Seconds()),
	})
	return token, nil
}

// Verify checks that the submitted token matches the cookie and has a valid signature.
// Inputs: request and submitted form/WS token.
// Output: true when both cookie and token are present, equal, and untampered/unexpired.
func (m *Manager) Verify(r *http.Request, submitted string) bool {
	submitted = strings.TrimSpace(submitted)
	if submitted == "" || m == nil {
		return false
	}
	c, err := r.Cookie(CookieName)
	if err != nil || c == nil || strings.TrimSpace(c.Value) == "" {
		return false
	}
	if subtleConstantTimeEq(c.Value, submitted) && m.valid(submitted) {
		return true
	}
	return false
}

func (m *Manager) sign(nonce string, exp int64) string {
	payload := nonce + "." + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "." + sig))
}

func (m *Manager) valid(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 3 {
		return false
	}
	nonce, expStr, sig := parts[0], parts[1], parts[2]
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	payload := nonce + "." + expStr
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

func subtleConstantTimeEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
