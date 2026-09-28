package models

import "testing"

// TestSanitizeHexColor validates hex colors and falls back when input is unsafe.
func TestSanitizeHexColor(t *testing.T) {
	if got := SanitizeHexColor("#0A2E2E", ""); got != "#0a2e2e" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeHexColor("red", "#134e4a"); got != "#134e4a" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeHexColor("linear-gradient(red, blue)", ""); got != defaultBannerHex {
		t.Fatalf("got %q", got)
	}
}

// TestSanitizeGradientDir only allows known CSS directions.
func TestSanitizeGradientDir(t *testing.T) {
	if got := SanitizeGradientDir("to Right"); got != "to right" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeGradientDir("url(javascript:alert(1))"); got != defaultGradientDir {
		t.Fatalf("got %q", got)
	}
}

// TestSectionBannerBackgroundCSS builds solid and gradient backgrounds from stored fields.
func TestSectionBannerBackgroundCSS(t *testing.T) {
	solid := SectionBanner{BackgroundColor: "#0a2e2e"}
	if got := solid.BackgroundCSS(); got != "background-color: #0a2e2e;" {
		t.Fatalf("solid got %q", got)
	}

	grad := SectionBanner{
		BackgroundColor:       "#0a2e2e",
		BackgroundColorEnd:    "#134e4a",
		UseBackgroundGradient: true,
		BackgroundGradientDir: "to left",
	}
	want := "background: linear-gradient(to left, #0a2e2e, #134e4a);"
	if got := grad.BackgroundCSS(); got != want {
		t.Fatalf("gradient got %q want %q", got, want)
	}
}

// TestSectionBannerOverlayCSS matches the previous hardcoded dual overlay when defaults are used.
func TestSectionBannerOverlayCSS(t *testing.T) {
	disabled := SectionBanner{UseOverlayGradient: false, OverlayColor: "#0a2e2e", OverlayOpacityLeft: 85, OverlayOpacityBottom: 60}
	if got := disabled.OverlayCSS(); got != "" {
		t.Fatalf("disabled overlay got %q", got)
	}

	legacy := SectionBanner{}
	if !legacy.OverlayEnabled() {
		t.Fatal("zero-value banner should keep legacy overlay enabled")
	}

	on := SectionBanner{
		UseOverlayGradient:   true,
		OverlayColor:         "#0a2e2e",
		OverlayOpacityLeft:   85,
		OverlayOpacityBottom: 60,
	}
	got := on.OverlayCSS()
	want := "background: linear-gradient(to left, rgba(10,46,46,0.85) 0%, transparent 40%), linear-gradient(to top, rgba(10,46,46,0.60) 0%, transparent 50%);"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
