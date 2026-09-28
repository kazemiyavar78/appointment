package cache

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// PushSubscriptionTTL عمر نگهداری اشتراک اعلان یک نوبت در حافظه.
	PushSubscriptionTTL = 4 * time.Hour
)

// DeviceSubscription یک مرورگر/دستگاه ثبت‌شده برای اعلان پس‌زمینه است.
type DeviceSubscription struct {
	Endpoint  string
	P256dh    string
	Auth      string
	OpenURL   string
	LastAhead int
	HasAhead  bool
}

// PushJob یک اعلان آماده ارسال است؛ فقط وقتی تعداد نفرات جلوتر کم شده باشد ساخته می‌شود.
type PushJob struct {
	Endpoint    string
	P256dh      string
	Auth        string
	OpenURL     string
	Ahead       int
	Strong      bool
	ClinicID    uint
	AdmissionNo int
	NationalID  string
}

// PatientRef هویت یک بیمارِ دارای اشتراک در یک مرکز است.
type PatientRef struct {
	AdmissionNo int
	NationalID  string
}

type patientPushBag struct {
	Devices []DeviceSubscription
}

// PushSubscriptionCache اشتراک‌های Web Push را در go-cache نگه می‌دارد، جدا از خود صف.
type PushSubscriptionCache struct {
	mu    sync.Mutex
	store *Store
}

// NewPushSubscriptionCache سازنده کش اشتراک اعلان است.
// ورودی: store مشترک حافظه. اگر nil باشد کش خالی و بی‌اثر برمی‌گردد.
// خروجی: اشاره‌گر آماده استفاده.
func NewPushSubscriptionCache(store *Store) *PushSubscriptionCache {
	return &PushSubscriptionCache{store: store}
}

func pushPatientKey(clinicID uint, admission int, nationalID string) string {
	return fmt.Sprintf("wqpush:%d:%d:%s", clinicID, admission, nationalID)
}

func pushIndexKey(clinicID uint) string {
	return fmt.Sprintf("wqpush-idx:%d", clinicID)
}

func pushIndexToken(admission int, nationalID string) string {
	return strconv.Itoa(admission) + "\t" + nationalID
}

func parsePushIndexToken(token string) (PatientRef, bool) {
	admissionRaw, nationalID, ok := strings.Cut(token, "\t")
	if !ok {
		return PatientRef{}, false
	}
	admission, err := strconv.Atoi(admissionRaw)
	if err != nil || admission <= 0 {
		return PatientRef{}, false
	}
	return PatientRef{AdmissionNo: admission, NationalID: nationalID}, true
}

// Add دستگاه را برای یک پذیرش ذخیره می‌کند. اگر همان endpoint دوباره بیاید، کلیدها به‌روز می‌شوند و سابقه تعداد نفرات حفظ می‌شود.
// ورودی: شناسه مرکز، شماره پذیرش، کد ملی، endpoint و کلیدهای push، و آدرسی که با لمس اعلان باز شود.
// خروجی: خطا اگر داده اشتراک نامعتبر باشد.
func (c *PushSubscriptionCache) Add(clinicID uint, admission int, nationalID, endpoint, p256dh, auth, openURL string) error {
	if err := validatePushEndpoint(endpoint, p256dh, auth); err != nil {
		return err
	}
	if c == nil || c.store == nil || clinicID == 0 || admission <= 0 {
		return fmt.Errorf("push subscription unavailable")
	}
	nationalID = strings.TrimSpace(nationalID)

	c.mu.Lock()
	defer c.mu.Unlock()

	key := pushPatientKey(clinicID, admission, nationalID)
	bag := c.loadBag(key)
	replaced := false
	for i := range bag.Devices {
		if bag.Devices[i].Endpoint == endpoint {
			bag.Devices[i].P256dh = p256dh
			bag.Devices[i].Auth = auth
			bag.Devices[i].OpenURL = openURL
			replaced = true
			break
		}
	}
	if !replaced {
		bag.Devices = append(bag.Devices, DeviceSubscription{
			Endpoint: endpoint,
			P256dh:   p256dh,
			Auth:     auth,
			OpenURL:  openURL,
		})
	}
	c.store.SetWithTTL(key, bag, PushSubscriptionTTL)
	c.addIndexToken(clinicID, pushIndexToken(admission, nationalID))
	return nil
}

// HasClinic می‌گوید این مرکز حداقل یک اشتراک زنده دارد تا Refresher فاصله ۵ ثانیه را نگه دارد.
// ورودی: شناسه مرکز. خروجی: true اگر اشتراک منقضی‌نشده وجود داشته باشد.
func (c *PushSubscriptionCache) HasClinic(clinicID uint) bool {
	if c == nil || c.store == nil || clinicID == 0 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.liveTokens(clinicID)) > 0
}

// ListPatients بیماران دارای اشتراک یک مرکز را برمی‌گرداند.
// ورودی: شناسه مرکز. خروجی: فهرست پذیرش و کد ملی.
func (c *PushSubscriptionCache) ListPatients(clinicID uint) []PatientRef {
	if c == nil || c.store == nil || clinicID == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	tokens := c.liveTokens(clinicID)
	out := make([]PatientRef, 0, len(tokens))
	for _, token := range tokens {
		ref, ok := parsePushIndexToken(token)
		if ok {
			out = append(out, ref)
		}
	}
	return out
}

// CollectDecreases دستگاه‌هایی را برمی‌گرداند که باید برای کاهش تعداد نفرات اعلان بگیرند.
// اولین مشاهده فقط خط پایه است و اعلان نمی‌فرستد. اگر بیمار در صف نباشد چیزی عوض نمی‌شود.
// ورودی: مرکز، پذیرش، کد ملی، پیدا شدن در صف، و تعداد فعلی نفرات جلوتر.
// خروجی: کارهای ارسال. آخرین تعدادِ دستگاه‌های در صف ارسال هنوز ذخیره نمی‌شود تا در صورت خطای شبکه دوباره تلاش شود.
func (c *PushSubscriptionCache) CollectDecreases(clinicID uint, admission int, nationalID string, found bool, ahead int) []PushJob {
	if c == nil || c.store == nil || !found || clinicID == 0 || admission <= 0 {
		return nil
	}
	if ahead < 0 {
		ahead = 0
	}
	nationalID = strings.TrimSpace(nationalID)

	c.mu.Lock()
	defer c.mu.Unlock()

	key := pushPatientKey(clinicID, admission, nationalID)
	bag := c.loadBag(key)
	if len(bag.Devices) == 0 {
		return nil
	}
	jobs := make([]PushJob, 0)
	changed := false
	for i := range bag.Devices {
		device := &bag.Devices[i]
		if !device.HasAhead {
			device.HasAhead = true
			device.LastAhead = ahead
			changed = true
			continue
		}
		if ahead < device.LastAhead {
			jobs = append(jobs, PushJob{
				Endpoint:    device.Endpoint,
				P256dh:      device.P256dh,
				Auth:        device.Auth,
				OpenURL:     device.OpenURL,
				Ahead:       ahead,
				Strong:      ahead == 0,
				ClinicID:    clinicID,
				AdmissionNo: admission,
				NationalID:  nationalID,
			})
			continue
		}
		if device.LastAhead != ahead {
			device.LastAhead = ahead
			changed = true
		}
	}
	if changed {
		c.store.ReplaceKeepTTL(key, bag, PushSubscriptionTTL)
	}
	return jobs
}

// MarkNotified بعد از ارسال موفق، تعداد دیده‌شده این دستگاه را روی مقدار جدید می‌گذارد.
// ورودی: همان PushJob ارسال‌شده. خروجی: ندارد.
func (c *PushSubscriptionCache) MarkNotified(job PushJob) {
	if c == nil || c.store == nil || job.ClinicID == 0 || job.Endpoint == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := pushPatientKey(job.ClinicID, job.AdmissionNo, job.NationalID)
	bag := c.loadBag(key)
	updated := false
	for i := range bag.Devices {
		if bag.Devices[i].Endpoint != job.Endpoint {
			continue
		}
		bag.Devices[i].HasAhead = true
		bag.Devices[i].LastAhead = job.Ahead
		updated = true
		break
	}
	if updated {
		c.store.ReplaceKeepTTL(key, bag, PushSubscriptionTTL)
	}
}

// RemoveEndpoint اشتراک منقضی (پاسخ 404 یا 410 سرویس پوش) را حذف می‌کند.
// ورودی: مرکز، پذیرش، کد ملی و endpoint. خروجی: ندارد.
func (c *PushSubscriptionCache) RemoveEndpoint(clinicID uint, admission int, nationalID, endpoint string) {
	if c == nil || c.store == nil || clinicID == 0 || endpoint == "" {
		return
	}
	nationalID = strings.TrimSpace(nationalID)
	c.mu.Lock()
	defer c.mu.Unlock()

	key := pushPatientKey(clinicID, admission, nationalID)
	bag := c.loadBag(key)
	kept := bag.Devices[:0]
	for _, device := range bag.Devices {
		if device.Endpoint != endpoint {
			kept = append(kept, device)
		}
	}
	token := pushIndexToken(admission, nationalID)
	if len(kept) == 0 {
		c.store.Delete(key)
		c.removeIndexToken(clinicID, token)
		return
	}
	bag.Devices = kept
	c.store.ReplaceKeepTTL(key, bag, PushSubscriptionTTL)
}

func (c *PushSubscriptionCache) loadBag(key string) patientPushBag {
	raw, ok := c.store.Get(key)
	if !ok {
		return patientPushBag{}
	}
	bag, ok := raw.(patientPushBag)
	if !ok {
		return patientPushBag{}
	}
	return bag
}

func (c *PushSubscriptionCache) loadIndex(clinicID uint) []string {
	raw, ok := c.store.Get(pushIndexKey(clinicID))
	if !ok {
		return nil
	}
	tokens, ok := raw.([]string)
	if !ok {
		return nil
	}
	out := make([]string, len(tokens))
	copy(out, tokens)
	return out
}

func (c *PushSubscriptionCache) liveTokens(clinicID uint) []string {
	tokens := c.loadIndex(clinicID)
	if len(tokens) == 0 {
		return nil
	}
	live := make([]string, 0, len(tokens))
	for _, token := range tokens {
		ref, ok := parsePushIndexToken(token)
		if !ok {
			continue
		}
		if _, found := c.store.Get(pushPatientKey(clinicID, ref.AdmissionNo, ref.NationalID)); found {
			live = append(live, token)
		}
	}
	if len(live) != len(tokens) {
		if len(live) == 0 {
			c.store.Delete(pushIndexKey(clinicID))
		} else {
			c.store.ReplaceKeepTTL(pushIndexKey(clinicID), live, PushSubscriptionTTL)
		}
	}
	return live
}

func (c *PushSubscriptionCache) addIndexToken(clinicID uint, token string) {
	tokens := c.loadIndex(clinicID)
	for _, existing := range tokens {
		if existing == token {
			c.store.SetWithTTL(pushIndexKey(clinicID), tokens, PushSubscriptionTTL)
			return
		}
	}
	tokens = append(tokens, token)
	c.store.SetWithTTL(pushIndexKey(clinicID), tokens, PushSubscriptionTTL)
}

func (c *PushSubscriptionCache) removeIndexToken(clinicID uint, token string) {
	tokens := c.loadIndex(clinicID)
	kept := tokens[:0]
	for _, existing := range tokens {
		if existing != token {
			kept = append(kept, existing)
		}
	}
	if len(kept) == 0 {
		c.store.Delete(pushIndexKey(clinicID))
		return
	}
	c.store.ReplaceKeepTTL(pushIndexKey(clinicID), kept, PushSubscriptionTTL)
}

// validatePushEndpoint شکل endpoint و وجود کلیدها را بررسی می‌کند.
// ورودی: endpoint و دو کلید اشتراک. خروجی: خطا اگر برای Web Push قابل استفاده نباشد.
func validatePushEndpoint(endpoint, p256dh, auth string) error {
	endpoint = strings.TrimSpace(endpoint)
	p256dh = strings.TrimSpace(p256dh)
	auth = strings.TrimSpace(auth)
	if endpoint == "" || p256dh == "" || auth == "" {
		return fmt.Errorf("push subscription incomplete")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("push endpoint invalid")
	}
	host := strings.ToLower(parsed.Hostname())
	local := host == "localhost" || host == "127.0.0.1" || host == "::1"
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if local {
			return nil
		}
	}
	return fmt.Errorf("push endpoint must be https")
}
