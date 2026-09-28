package slug

import (
	"errors"
	"strings"
	"testing"
)

func TestMakeKeepsDoctorFallback(t *testing.T) {
	if got := Make(""); got != "doctor" {
		t.Fatalf("Make empty = %q", got)
	}
	if got := Make("دندان\u200cپزشکی"); got != "دندانپزشکی" {
		t.Fatalf("legacy zwnj = %q", got)
	}
	if got := Make("داخلي"); got != "داخلي" {
		t.Fatalf("legacy arabic yeh = %q", got)
	}
}

func TestMakePersian(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"رئیس بخش", "رئیس-بخش"},
		{"داخلي", "داخلی"},
		{"داخلی", "داخلی"},
		{"فوق تخصص گوارش", "فوق-تخصص-گوارش"},
		{"زنان و زایمان", "زنان-و-زایمان"},
		{"دندان\u200cپزشکی", "دندان-پزشکی"},
		{"  قلب   و   عروق  ", "قلب-و-عروق"},
		{"قلب:عروق", "قلب-عروق"},
		{"قلب.عروق", "قلب-عروق"},
		{"قلب،عروق", "قلب-عروق"},
		{"قلب,عروق", "قلب-عروق"},
		{"قلب(عروق)", "قلب-عروق"},
		{"قلب/عروق", "قلب-عروق"},
		{`قلب\عروق`, "قلب-عروق"},
		{"قلب_عروق", "قلب-عروق"},
		{"قلب   عروق", "قلب-عروق"},
		{"قلب---عروق", "قلب-عروق"},
		{"-قلب-", "قلب"},
	}
	for _, tc := range tests {
		got, err := MakePersian(tc.in)
		if err != nil {
			t.Fatalf("MakePersian(%q) error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("MakePersian(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") || strings.Contains(got, "--") || strings.ContainsAny(got, "/\\") {
			t.Fatalf("unsafe slug %q", got)
		}
	}
	arabic, err1 := MakePersian("داخلي")
	persian, err2 := MakePersian("داخلی")
	if err1 != nil || err2 != nil || arabic != persian {
		t.Fatalf("equivalent slugs %q %q", arabic, persian)
	}
	if _, err := MakePersian("   "); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty err = %v", err)
	}
	if _, err := MakePersian("..."); !errors.Is(err, ErrEmpty) {
		t.Fatalf("punctuation-only err = %v", err)
	}
}
