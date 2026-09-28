package booking

import (
	"encoding/json"
	"testing"
)

func TestFormatTrackingCode(t *testing.T) {
	if got := FormatTrackingCode(0); got != "" {
		t.Fatalf("empty id: got %q", got)
	}
	if got := FormatTrackingCode(42); got != "42" {
		t.Fatalf("id 42: got %q", got)
	}
}

func TestNewFinal_serializesFailureForBrowser(t *testing.T) {
	ev := NewFinal(false, "ظرفیت این نوبت تکمیل شده است", "")
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["done"] != true {
		t.Fatalf("done=%v want true", payload["done"])
	}
	if payload["ok"] != false {
		t.Fatalf("ok=%v want false (must not be omitted)", payload["ok"])
	}
	if payload["message"] != "ظرفیت این نوبت تکمیل شده است" {
		t.Fatalf("message=%v", payload["message"])
	}
	if _, ok := payload["tracking_code"]; ok {
		t.Fatalf("failure must not include tracking_code: %s", raw)
	}
	if _, ok := payload["external_id"]; ok {
		t.Fatalf("client payload must not include external_id: %s", raw)
	}
}

func TestNewFinal_includesTrackingCodeOnSuccess(t *testing.T) {
	ev := NewFinal(true, "نوبت ثبت شد", "128")
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["ok"] != true {
		t.Fatalf("ok=%v want true", payload["ok"])
	}
	if payload["tracking_code"] != "128" {
		t.Fatalf("tracking_code=%v", payload["tracking_code"])
	}
}

func TestClinicRejectedForCapacity(t *testing.T) {
	if !clinicRejectedForCapacity("no_capacity", "") {
		t.Fatal("expected no_capacity code")
	}
	if !clinicRejectedForCapacity("", "ظرفیت این نوبت تکمیل شده است") {
		t.Fatal("expected capacity message")
	}
	if clinicRejectedForCapacity("already_booked", "با این کد ملی قبلاً نوبت ثبت شده است") {
		t.Fatal("duplicate booking must not drop the slot")
	}
}
