package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/statusnotify"
	adminviews "tebpardaz/server/views/admin"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "tp_admin_session"
	otpPendingCookie  = "tp_admin_otp"
	sessionTTL        = 24 * time.Hour
	otpPendingTTL     = 5 * time.Minute
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
	OTP    *AdminOTPStore
	Status *statusnotify.Notifier
}

// NewAdminAuth constructs AdminAuth.
// Inputs: users repo, session HMAC secret, OTP store, status notifier (may be nil).
// Output: pointer to AdminAuth.
func NewAdminAuth(users *repository.UserRepo, sessionSecret string, otp *AdminOTPStore, status *statusnotify.Notifier) *AdminAuth {
	if otp == nil {
		otp = NewAdminOTPStore()
	}
	return &AdminAuth{
		Users:  users,
		Secret: []byte(sessionSecret),
		OTP:    otp,
		Status: status,
	}
}

// LoginPage renders the login form (HTML).
func (a *AdminAuth) LoginPage(c *gin.Context) {
	if user, ok := a.currentUser(c); ok {
		c.Redirect(http.StatusFound, homePathForRole(user.Role))
		return
	}
	if pending, ok := a.pendingOTPUser(c); ok {
		renderAdminLogin(c, http.StatusOK, adminviews.AdminLoginView{
			OTPRequired: true,
			Username:    pending.Username,
		})
		return
	}
	renderAdminLogin(c, http.StatusOK, adminviews.AdminLoginView{})
}

// Login authenticates username/password then requires OTP, or verifies the OTP step.
func (a *AdminAuth) Login(c *gin.Context) {
	if user, ok := a.currentUser(c); ok {
		c.Redirect(http.StatusFound, homePathForRole(user.Role))
		return
	}
	if strings.TrimSpace(c.PostForm("otp_code")) != "" || a.hasPendingOTP(c) {
		a.completeLoginWithOTP(c)
		return
	}
	a.startLoginOTP(c)
}

// Logout clears the session cookie.
func (a *AdminAuth) Logout(c *gin.Context) {
	a.clearPendingOTP(c)
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

// homePathForRole مسیر صفحه اول پنل را بر اساس نقش برمی‌گرداند.
// ورودی: رشته نقش کاربر. خروجی: مسیر داشبورد یا اخبار برای ویراستار.
func homePathForRole(role string) string {
	if constants.UserRole(role) == constants.UserRoleEditor {
		return "/admin/news"
	}
	return "/admin"
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
	value, err := a.signPayload(payload)
	if err != nil {
		return err
	}
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

// startLoginOTP verifies password and sends a login OTP to status recipients.
func (a *AdminAuth) startLoginOTP(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	if username == "" || password == "" {
		renderAdminLogin(c, http.StatusBadRequest, adminviews.AdminLoginView{ErrorMessage: "نام کاربری و رمز عبور الزامی است"})
		return
	}
	user, err := a.Users.FindByUsername(username)
	if err != nil || user == nil || !user.IsActive {
		renderAdminLogin(c, http.StatusUnauthorized, adminviews.AdminLoginView{ErrorMessage: "نام کاربری یا رمز عبور نادرست است"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		renderAdminLogin(c, http.StatusUnauthorized, adminviews.AdminLoginView{ErrorMessage: "نام کاربری یا رمز عبور نادرست است"})
		return
	}
	code, err := a.OTP.Create(user.ID)
	if err != nil {
		renderAdminLogin(c, http.StatusInternalServerError, adminviews.AdminLoginView{ErrorMessage: "خطا در ایجاد کد تایید"})
		return
	}
	if err := a.sendLoginOTP(c.Request.Context(), user, code, c.ClientIP()); err != nil {
		renderAdminLogin(c, http.StatusBadGateway, adminviews.AdminLoginView{ErrorMessage: err.Error()})
		return
	}
	if err := a.setPendingOTP(c, user); err != nil {
		renderAdminLogin(c, http.StatusInternalServerError, adminviews.AdminLoginView{ErrorMessage: "خطا در ذخیره نشست تایید"})
		return
	}
	renderAdminLogin(c, http.StatusOK, adminviews.AdminLoginView{
		OTPRequired:  true,
		Username:     user.Username,
		ErrorMessage: "",
	})
}

// completeLoginWithOTP verifies the pending OTP and opens the admin session.
func (a *AdminAuth) completeLoginWithOTP(c *gin.Context) {
	user, ok := a.pendingOTPUser(c)
	if !ok {
		renderAdminLogin(c, http.StatusUnauthorized, adminviews.AdminLoginView{ErrorMessage: "نشست تایید منقضی شده؛ دوباره وارد شوید"})
		return
	}
	code := strings.TrimSpace(c.PostForm("otp_code"))
	if code == "" {
		renderAdminLogin(c, http.StatusBadRequest, adminviews.AdminLoginView{
			OTPRequired:  true,
			Username:     user.Username,
			ErrorMessage: "کد تایید را وارد کنید",
		})
		return
	}
	if err := a.OTP.Verify(user.ID, code); err != nil {
		msg := "کد تایید نادرست است"
		switch err {
		case ErrAdminOTPExpired, ErrAdminOTPNotFound:
			a.clearPendingOTP(c)
			renderAdminLogin(c, http.StatusUnauthorized, adminviews.AdminLoginView{ErrorMessage: "کد تایید منقضی شده؛ دوباره وارد شوید"})
			return
		case ErrAdminOTPTooManyAttempts:
			a.clearPendingOTP(c)
			renderAdminLogin(c, http.StatusTooManyRequests, adminviews.AdminLoginView{ErrorMessage: "تعداد تلاش‌ها بیش از حد مجاز است؛ دوباره وارد شوید"})
			return
		}
		renderAdminLogin(c, http.StatusUnauthorized, adminviews.AdminLoginView{
			OTPRequired:  true,
			Username:     user.Username,
			ErrorMessage: msg,
		})
		return
	}
	a.clearPendingOTP(c)
	if err := a.setSession(c, user); err != nil {
		c.String(http.StatusInternalServerError, "session error")
		return
	}
	c.Redirect(http.StatusFound, homePathForRole(user.Role))
}

// sendLoginOTP پیامک کد ورود را برای سوپرادمین‌ها و کاربران تیک‌خورده مرکز می‌فرستد.
func (a *AdminAuth) sendLoginOTP(ctx context.Context, user *models.AppointmentUser, code, clientIP string) error {
	if a.Status == nil {
		return fmt.Errorf("سرویس پیامک پیکربندی نشده است")
	}
	clinicID := uint(0)
	if user.ClinicID != nil {
		clinicID = *user.ClinicID
	}
	msg := fmt.Sprintf("کد ورود به پنل مدیریت: %s\nکاربر: %s", code, user.Username)
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := a.Status.SendToRecipients(sendCtx, clinicID, 0, clientIP, msg); err != nil {
		return fmt.Errorf("ارسال کد تایید ناموفق بود: %w", err)
	}
	return nil
}

func (a *AdminAuth) hasPendingOTP(c *gin.Context) bool {
	_, ok := a.pendingOTPUser(c)
	return ok
}

func (a *AdminAuth) pendingOTPUser(c *gin.Context) (*models.AppointmentUser, bool) {
	cookie, err := c.Cookie(otpPendingCookie)
	if err != nil || cookie == "" {
		return nil, false
	}
	payload, err := a.verifySession(cookie)
	if err != nil || payload.ExpiresAt < time.Now().Unix() {
		return nil, false
	}
	user, err := a.Users.FindByID(payload.UserID)
	if err != nil || user == nil || !user.IsActive {
		return nil, false
	}
	return user, true
}

func (a *AdminAuth) setPendingOTP(c *gin.Context, user *models.AppointmentUser) error {
	payload := sessionPayload{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(otpPendingTTL).Unix(),
	}
	value, err := a.signPayload(payload)
	if err != nil {
		return err
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     otpPendingCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   int(otpPendingTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (a *AdminAuth) clearPendingOTP(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     otpPendingCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *AdminAuth) signPayload(payload sessionPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, a.Secret)
	_, _ = mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

// renderAdminLogin صفحه ورود ادمین را با کامپوننت templ و طراحی Glassmorphism رندر می‌کند.
func renderAdminLogin(c *gin.Context, status int, view adminviews.AdminLoginView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	_ = adminviews.AdminLogin(view).Render(c.Request.Context(), c.Writer)
}
