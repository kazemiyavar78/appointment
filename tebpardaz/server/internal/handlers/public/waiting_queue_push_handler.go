package public

import (
	"net/http"
	"strings"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// WaitingQueuePushHandler ثبت اشتراک Web Push را جدا از فرم و WebSocket صف انجام می‌دهد.
type WaitingQueuePushHandler struct {
	Queue *WaitingQueueHandler
	Subs  *cache.PushSubscriptionCache
}

type pushSubscribeBody struct {
	CSRFToken string `json:"csrf_token"`
	Endpoint  string `json:"endpoint"`
	Keys      struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// NewWaitingQueuePushHandler سازنده هندلر اشتراک اعلان است.
// ورودی: هندلر صف (برای CSRF، مستأجر و تشخیص مرکز) و کش اشتراک.
// خروجی: اشاره‌گر هندلر.
func NewWaitingQueuePushHandler(queue *WaitingQueueHandler, subs *cache.PushSubscriptionCache) *WaitingQueuePushHandler {
	return &WaitingQueuePushHandler{Queue: queue, Subs: subs}
}

// Subscribe بدنه PushSubscription را می‌گیرد و با کلید پذیرش در حافظه ذخیره می‌کند.
// ورودی: gin context با پارامترهای مسیر و JSON. خروجی: JSON فارسی.
func (h *WaitingQueuePushHandler) Subscribe(c *gin.Context) {
	if h == nil || h.Queue == nil || h.Subs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "message": "سرویس اعلان در دسترس نیست"})
		return
	}
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "نشست صفحه نامعتبر است"})
		return
	}

	var body pushSubscribeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "اطلاعات اعلان نامعتبر است"})
		return
	}
	if h.Queue.CSRF == nil || !h.Queue.CSRF.Verify(c.Request, body.CSRFToken) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": msgWQCSRFInvalid})
		return
	}

	admissionNo, nationalID, clinicID, errMsg := h.Queue.parsePathParams(c, tc)
	if errMsg != "" || admissionNo <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": firstNonEmpty(errMsg, msgWQInvalidForm)})
		return
	}
	clinic, resolveMsg := h.Queue.resolveClinic(tc, clinicID)
	if resolveMsg != "" || clinic == nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": firstNonEmpty(resolveMsg, msgWQClinicInvalid)})
		return
	}

	needClinic := tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform
	openURL := buildWaitingQueuePagePath(admissionNo, nationalID, clinic.ID, needClinic)
	if err := h.Subs.Add(
		clinic.ID,
		admissionNo,
		nationalID,
		strings.TrimSpace(body.Endpoint),
		strings.TrimSpace(body.Keys.P256dh),
		strings.TrimSpace(body.Keys.Auth),
		openURL,
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "اطلاعات اعلان نامعتبر است"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "اعلان پس‌زمینه فعال شد"})
}

// firstNonEmpty اولین رشته غیرخالی را برمی‌گرداند.
// ورودی: چند پیام. خروجی: اولین مقدار پر، یا رشته خالی.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
