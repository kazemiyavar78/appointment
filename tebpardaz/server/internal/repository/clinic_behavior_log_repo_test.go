package repository

import (
	"testing"
	"time"
)

// TestClinicBehaviorLogRetentionIsTwoDays مدت نگهداری لاگ را روی ۴۸ ساعت قفل می‌کند.
func TestClinicBehaviorLogRetentionIsTwoDays(t *testing.T) {
	if ClinicBehaviorLogRetention != 2*24*time.Hour {
		t.Fatalf("retention=%s want=48h", ClinicBehaviorLogRetention)
	}
}

// TestClinicBehaviorLogCutoffSubtractsTwoDays آستانه حذف را دو روز قبل از now برمی‌گرداند.
func TestClinicBehaviorLogCutoffSubtractsTwoDays(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	got := ClinicBehaviorLogCutoff(now)
	want := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("cutoff=%v want=%v", got, want)
	}
}

// TestDeleteOlderThanNilRepo بدون اتصال دیتابیس هیچ ردیفی حذف نمی‌کند.
func TestDeleteOlderThanNilRepo(t *testing.T) {
	var r *ClinicBehaviorLogRepo
	n, err := r.DeleteOlderThan(time.Now())
	if err != nil || n != 0 {
		t.Fatalf("nil repo: n=%d err=%v", n, err)
	}
	r = &ClinicBehaviorLogRepo{}
	n, err = r.DeleteOlderThan(time.Now())
	if err != nil || n != 0 {
		t.Fatalf("nil db: n=%d err=%v", n, err)
	}
	r = NewClinicBehaviorLogRepo(nil)
	n, err = r.DeleteOlderThan(time.Time{})
	if err != nil || n != 0 {
		t.Fatalf("zero cutoff: n=%d err=%v", n, err)
	}
}

type fakeLogPurger struct {
	before time.Time
	n      int64
	err    error
}

// DeleteOlderThan آستانه حذف را ذخیره می‌کند و نتیجه ساختگی برمی‌گرداند.
func (f *fakeLogPurger) DeleteOlderThan(before time.Time) (int64, error) {
	f.before = before
	return f.n, f.err
}

// TestClinicBehaviorLogCleanerPurgeNowDeletesOlderThanTwoDays پاکسازی را با cutoff دو روزه صدا می‌زند.
func TestClinicBehaviorLogCleanerPurgeNowDeletesOlderThanTwoDays(t *testing.T) {
	now := time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC)
	fake := &fakeLogPurger{n: 3}
	c := &ClinicBehaviorLogCleaner{
		purger: fake,
		now:    func() time.Time { return now },
	}
	n, err := c.PurgeNow()
	if err != nil || n != 3 {
		t.Fatalf("purge: n=%d err=%v", n, err)
	}
	want := time.Date(2026, 9, 15, 21, 0, 0, 0, time.UTC)
	if !fake.before.Equal(want) {
		t.Fatalf("deleted before=%v want=%v", fake.before, want)
	}
}

// TestClinicBehaviorLogCleanerStartStop حلقه پس‌زمینه را بدون بنگ شروع و متوقف می‌کند.
func TestClinicBehaviorLogCleanerStartStop(t *testing.T) {
	fake := &fakeLogPurger{}
	c := &ClinicBehaviorLogCleaner{
		purger:   fake,
		interval: time.Hour,
		now:      time.Now,
		stopCh:   make(chan struct{}),
	}
	c.Start()
	c.Stop()
	if fake.before.IsZero() {
		t.Fatal("expected an immediate purge on start")
	}
}
