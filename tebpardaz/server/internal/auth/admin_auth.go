package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "tp_admin_session"
	sessionTTL        = 24 * time.Hour
	// ContextUserKey is the gin.Context key for *models.AppointmentUser.
	ContextUserKey = "admin_user"
)

// sessionPayload is the signed cookie body.
type sessionPayload struct {
	UserID    uint  `json:"uid"`
	ExpiresAt int64 `json:"exp"`
}

// AdminAuth validates admin session cookies for back-office routes.
type AdminAuth struct {
	Users  *repository.UserRepo
	Secret []byte
}

// NewAdminAuth constructs AdminAuth.
// Inputs: users repo, session HMAC secret.
// Output: pointer to AdminAuth.
func NewAdminAuth(users *repository.UserRepo, sessionSecret string) *AdminAuth {
	return &AdminAuth{
		Users:  users,
		Secret: []byte(sessionSecret),
	}
}

// LoginPage renders the login form (HTML).
func (a *AdminAuth) LoginPage(c *gin.Context) {
	if _, ok := a.currentUser(c); ok {
		c.Redirect(http.StatusFound, "/admin/approvals")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, loginHTML(""))
}

// Login authenticates username/password and sets the session cookie.
func (a *AdminAuth) Login(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	if username == "" || password == "" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, loginHTML("نام کاربری و رمز عبور الزامی است"))
		return
	}
	user, err := a.Users.FindByUsername(username)
	if err != nil || !user.IsActive {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusUnauthorized, loginHTML("نام کاربری یا رمز عبور نادرست است"))
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusUnauthorized, loginHTML("نام کاربری یا رمز عبور نادرست است"))
		return
	}
	if err := a.setSession(c, user); err != nil {
		c.String(http.StatusInternalServerError, "session error")
		return
	}
	c.Redirect(http.StatusFound, "/admin/approvals")
}

// Logout clears the session cookie.
func (a *AdminAuth) Logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, "/admin/login")
}

// Middleware requires an authenticated admin user.
func (a *AdminAuth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := a.currentUser(c)
		if !ok {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}
		c.Set(ContextUserKey, user)
		c.Next()
	}
}

// RequireRoles rejects users whose role is not in the allowed set.
func (a *AdminAuth) RequireRoles(roles ...constants.UserRole) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[string(r)] = struct{}{}
	}
	return func(c *gin.Context) {
		user, ok := UserFromGin(c)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if _, ok := allowed[user.Role]; !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

// UserFromGin returns the authenticated AppointmentUser from gin context.
func UserFromGin(c *gin.Context) (*models.AppointmentUser, bool) {
	v, ok := c.Get(ContextUserKey)
	if !ok {
		return nil, false
	}
	u, ok := v.(*models.AppointmentUser)
	return u, ok
}

// currentUser reads and validates the session cookie.
func (a *AdminAuth) currentUser(c *gin.Context) (*models.AppointmentUser, bool) {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil || cookie == "" {
		return nil, false
	}
	payload, err := a.verifySession(cookie)
	if err != nil || payload.ExpiresAt < time.Now().Unix() {
		return nil, false
	}
	user, err := a.Users.FindByID(payload.UserID)
	if err != nil || !user.IsActive {
		return nil, false
	}
	return user, true
}

// setSession writes a signed session cookie for the given user.
func (a *AdminAuth) setSession(c *gin.Context, user *models.AppointmentUser) error {
	payload := sessionPayload{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(sessionTTL).Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, a.Secret)
	_, _ = mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	value := body + "." + sig
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// verifySession parses and HMAC-verifies a session cookie value.
func (a *AdminAuth) verifySession(value string) (*sessionPayload, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return nil, strconv.ErrSyntax
	}
	mac := hmac.New(sha256.New, a.Secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, got) {
		return nil, strconv.ErrSyntax
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var payload sessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func loginHTML(errMsg string) string {
	errBlock := ""
	if errMsg != "" {
		errBlock = `<p class="mb-3 text-sm text-red-600">` + errMsg + `</p>`
	}
	return `<!doctype html>
<html lang="fa" dir="rtl">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<title>ورود ادمین | طب‌پرداز</title>
<link rel="stylesheet" href="/static/css/output.css"/>
</head>
<body class="bg-surface-soft text-ink font-sans">
<main class="mx-auto flex min-h-screen max-w-md items-center p-4">
<form method="post" action="/admin/login" class="w-full space-y-3 rounded border border-gray-200 bg-white p-6">
<h1 class="mb-2 text-2xl text-brand">ورود به پنل</h1>
` + errBlock + `
<label class="block text-sm text-ink-muted">نام کاربری</label>
<input class="w-full rounded border border-gray-300 p-2" type="text" name="username" required autocomplete="username"/>
<label class="block text-sm text-ink-muted">رمز عبور</label>
<input class="w-full rounded border border-gray-300 p-2" type="password" name="password" required autocomplete="current-password"/>
<button class="w-full rounded bg-brand px-4 py-2 text-white" type="submit">ورود</button>
</form>
</main>
</body>
</html>`
}
