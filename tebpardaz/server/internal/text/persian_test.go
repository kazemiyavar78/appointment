package text

import "testing"

func TestNormalizeArabicToPersian(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"محمد", "محمد"},
		{"علي", "علی"},
		{"كاظمي", "کاظمی"},
		{"  علي   كاظمي  ", "علی کاظمی"},
	}
	for _, tc := range tests {
		got := NormalizeArabicToPersian(tc.in)
		if got != tc.want {
			t.Fatalf("NormalizeArabicToPersian(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizePersianText(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"علي", "علی"},
		{"كريم", "کریم"},
		{"داخلى", "داخلی"},
		{"  علي   كاظمي  ", "علی کاظمی"},
		{"دکتر\t\tاحمدی", "دکتر احمدی"},
		{"دکتر\nاحمدی", "دکتر احمدی"},
		{"دکتر\u00a0احمدی", "دکتر احمدی"},
		{"دندان\u200cپزشک", "دندان\u200cپزشک"},
		{"دکتر 123", "دکتر 123"},
		{"رئیس", "رئیس"},
		{"مسئله", "مسئله"},
		{"   ", ""},
	}
	for _, tc := range tests {
		if got := NormalizePersianText(tc.in); got != tc.want {
			t.Fatalf("NormalizePersianText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeArabicToPersianKeepsLegacyDigits(t *testing.T) {
	if got := NormalizeArabicToPersian("دکتر 123"); got != "دکتر ۱۲۳" {
		t.Fatalf("legacy digits = %q", got)
	}
	if got := NormalizeArabicToPersian("علي"); got != "علی" {
		t.Fatalf("legacy letters = %q", got)
	}
	if got := NormalizeArabicToPersian("رئیس"); got != "رییس" {
		t.Fatalf("legacy hamza = %q", got)
	}
	if got := NormalizeArabicToPersian("فاطمة"); got != "فاطمه" {
		t.Fatalf("legacy teh = %q", got)
	}
}

func TestSearchLegacyForms(t *testing.T) {
	forms := SearchLegacyForms("رئیس")
	if len(forms) != 2 || forms[0] != "رئیس" || forms[1] != "رییس" {
		t.Fatalf("forms = %#v", forms)
	}
	if got := SearchLegacyForms("رییس"); len(got) != 1 || got[0] != "رییس" {
		t.Fatalf("stored legacy = %#v", got)
	}
	if got := SearchLegacyForms("دکتر 123"); len(got) != 1 || got[0] != "دکتر 123" {
		t.Fatalf("digits = %#v", got)
	}
	if got := SearchLegacyForms("كيارش"); len(got) != 1 || got[0] != "کیارش" {
		t.Fatalf("yeh = %#v", got)
	}
}

func TestHasArabicChars(t *testing.T) {
	if !HasArabicChars("علي") {
		t.Fatal("expected arabic chars")
	}
	if HasArabicChars("علی") {
		t.Fatal("expected no arabic chars")
	}
	if HasArabicChars("123") {
		t.Fatal("digits are not arabic letters")
	}
}

func TestToPersianDigits(t *testing.T) {
	got := ToPersianDigits("12.5%")
	if got != "۱۲.۵%" {
		t.Fatalf("ToPersianDigits = %q", got)
	}
	if ToPersianDigits("٤٥") != "۴۵" {
		t.Fatalf("arabic-indic digits: %q", ToPersianDigits("٤٥"))
	}
}

func TestFormatPersianInt(t *testing.T) {
	if got := FormatPersianInt(1234); got != "۱٬۲۳۴" {
		t.Fatalf("FormatPersianInt(1234)=%q", got)
	}
	if got := FormatPersianInt(-18); got != "−۱۸" {
		t.Fatalf("FormatPersianInt(-18)=%q", got)
	}
}

func TestFormatPersianPercent(t *testing.T) {
	got := FormatPersianPercent(18.0)
	if got != "۱۸٫۰٪" {
		t.Fatalf("FormatPersianPercent(18)=%q", got)
	}
}
