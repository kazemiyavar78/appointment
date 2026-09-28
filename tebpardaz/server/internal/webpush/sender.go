package webpush

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tebpardaz/server/internal/cache"

	wp "github.com/SherClockHolmes/webpush-go"
)

// Sender اعلان Web Push را با کلید VAPID به سرویس مرورگر می‌فرستد.
type Sender struct {
	keys   *Keys
	client *http.Client
}

type pushPayload struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Tag     string `json:"tag"`
	URL     string `json:"url"`
	Vibrate []int  `json:"vibrate"`
}

// NewSender سازنده فرستنده است.
// ورودی: کلید VAPID. خروجی: اشاره‌گر Sender. اگر کلید nil باشد Send بدون خطا برمی‌گردد.
func NewSender(keys *Keys) *Sender {
	return &Sender{
		keys:   keys,
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

// SendDecrease اگر تعداد نفرات کم شده باشد یک اعلان می‌فرستد.
// ورودی: PushJob شامل endpoint و تعداد جدید. خروجی: drop=true وقتی اشتراک مرده است (404/410)، و خطای شبکه یا سرویس.
func (s *Sender) SendDecrease(job cache.PushJob) (drop bool, err error) {
	if s == nil || s.keys == nil || strings.TrimSpace(job.Endpoint) == "" {
		return false, nil
	}
	body, vibrate := aheadNotice(job.Ahead, job.Strong)
	openURL := strings.TrimSpace(job.OpenURL)
	if openURL == "" {
		openURL = "/waiting-queue"
	}
	raw, err := json.Marshal(pushPayload{
		Title:   "وضعیت نوبت",
		Body:    body,
		Tag:     fmt.Sprintf("wq-%d-%d", job.ClinicID, job.AdmissionNo),
		URL:     openURL,
		Vibrate: vibrate,
	})
	if err != nil {
		return false, err
	}

	urgency := wp.UrgencyNormal
	if job.Strong {
		urgency = wp.UrgencyHigh
	}
	resp, err := wp.SendNotification(raw, &wp.Subscription{
		Endpoint: job.Endpoint,
		Keys: wp.Keys{
			P256dh: job.P256dh,
			Auth:   job.Auth,
		},
	}, &wp.Options{
		HTTPClient:      s.client,
		Subscriber:      s.keys.Subject,
		Topic:           fmt.Sprintf("wq-%d-%d", job.ClinicID, job.AdmissionNo),
		TTL:             3600,
		Urgency:         urgency,
		VAPIDPublicKey:  s.keys.PublicKey,
		VAPIDPrivateKey: s.keys.PrivateKey,
	})
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return true, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("webpush status %d", resp.StatusCode)
	}
	return false, nil
}

// aheadNotice متن و الگوی لرزش اعلان را از تعداد نفرات جلوتر می‌سازد.
// ورودی: تعداد نفرات و قوی‌بودن هشدار (رسیدن به صفر). خروجی: متن فارسی و vibrate.
func aheadNotice(ahead int, strong bool) (string, []int) {
	if strong || ahead <= 0 {
		return "نوبت شماست! لطفاً به اتاق پزشک بروید.", []int{220, 90, 220, 90, 320}
	}
	return faDigits(ahead) + " نفر جلوتر از شما", []int{180, 80, 180}
}

func faDigits(n int) string {
	digits := []string{"۰", "۱", "۲", "۳", "۴", "۵", "۶", "۷", "۸", "۹"}
	raw := fmt.Sprintf("%d", n)
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteString(digits[r-'0'])
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
