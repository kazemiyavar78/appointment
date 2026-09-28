package models

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultBannerHex       = "#0a2e2e"
	defaultGradientDir     = "to left"
	defaultOverlayLeft     = 85
	defaultOverlayBottom   = 60
	overlayStopLeftPercent = 40
	overlayStopBottomPct   = 50
)

var hexColorPattern = regexp.MustCompile(`(?i)^#[0-9a-f]{6}$`)

var allowedGradientDirs = map[string]string{
	"to left":         "to left",
	"to right":        "to right",
	"to top":          "to top",
	"to bottom":       "to bottom",
	"to top left":     "to top left",
	"to top right":    "to top right",
	"to bottom left":  "to bottom left",
	"to bottom right": "to bottom right",
}

// SanitizeHexColor validates a hex color and returns it in lowercase, or fallback if invalid.
// Input: raw color string and fallback hex. Output: safe #rrggbb color.
func SanitizeHexColor(c, fallback string) string {
	c = strings.TrimSpace(c)
	if hexColorPattern.MatchString(c) {
		return strings.ToLower(c)
	}
	fallback = strings.TrimSpace(fallback)
	if hexColorPattern.MatchString(fallback) {
		return strings.ToLower(fallback)
	}
	return defaultBannerHex
}

// SanitizeGradientDir returns a CSS linear-gradient direction from an allow-list.
// Input: raw direction string. Output: allowed CSS direction, defaulting to "to left".
func SanitizeGradientDir(dir string) string {
	dir = strings.TrimSpace(strings.ToLower(dir))
	if normalized, ok := allowedGradientDirs[dir]; ok {
		return normalized
	}
	return defaultGradientDir
}

// ClampPercent clamps n to 0..100, using fallback when n is outside that range and fallback is valid.
// Input: requested percent and fallback. Output: integer in 0..100.
func ClampPercent(n, fallback int) int {
	if n >= 0 && n <= 100 {
		return n
	}
	if fallback >= 0 && fallback <= 100 {
		return fallback
	}
	return 0
}

// HexToRGBA converts a hex color to an rgba() CSS value with the given alpha (0..1).
// Input: hex color and alpha. Output: rgba(r,g,b,a) string using a sanitized hex.
func HexToRGBA(hex string, alpha float64) string {
	hex = strings.TrimPrefix(SanitizeHexColor(hex, defaultBannerHex), "#")
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.2f)", r, g, b, alpha)
}

// BackgroundCSS builds a sanitized inline CSS background for the section banner.
// Input: receiver SectionBanner. Output: CSS such as background-color or linear-gradient.
func (b SectionBanner) BackgroundCSS() string {
	start := SanitizeHexColor(b.BackgroundColor, defaultBannerHex)
	end := SanitizeHexColor(b.BackgroundColorEnd, start)
	if b.UseBackgroundGradient && b.BackgroundColorEnd != "" {
		dir := SanitizeGradientDir(b.BackgroundGradientDir)
		return fmt.Sprintf("background: linear-gradient(%s, %s, %s);", dir, start, end)
	}
	return "background-color: " + start + ";"
}

// OverlayEnabled reports whether the banner image overlay should be rendered.
// Input: receiver SectionBanner. Output: true for explicit enable or unmigrated zero-value (legacy always-on overlay).
func (b SectionBanner) OverlayEnabled() bool {
	if b.UseOverlayGradient {
		return true
	}
	return strings.TrimSpace(b.OverlayColor) == "" && b.OverlayOpacityLeft == 0 && b.OverlayOpacityBottom == 0
}

// OverlayCSS builds a sanitized dual linear-gradient overlay for the banner image.
// Input: receiver SectionBanner. Output: CSS background value, or empty when overlay is disabled.
func (b SectionBanner) OverlayCSS() string {
	if !b.OverlayEnabled() {
		return ""
	}
	color := SanitizeHexColor(b.OverlayColor, b.BackgroundColor)
	left := b.OverlayOpacityLeft
	bottom := b.OverlayOpacityBottom
	if left == 0 && bottom == 0 {
		left = defaultOverlayLeft
		bottom = defaultOverlayBottom
	}
	left = ClampPercent(left, defaultOverlayLeft)
	bottom = ClampPercent(bottom, defaultOverlayBottom)
	leftRGBA := HexToRGBA(color, float64(left)/100)
	bottomRGBA := HexToRGBA(color, float64(bottom)/100)
	return fmt.Sprintf(
		"background: linear-gradient(to left, %s 0%%, transparent %d%%), linear-gradient(to top, %s 0%%, transparent %d%%);",
		leftRGBA, overlayStopLeftPercent, bottomRGBA, overlayStopBottomPct,
	)
}

// DefaultSectionBanner returns a banner with default slogan, colors, and overlay settings for a section.
// Input: section ID and title used in default copy. Output: SectionBanner ready to persist.
func DefaultSectionBanner(sectionID uint, sectionTitle string) SectionBanner {
	desc := "ارائه خدمات تشخیصی و درمانی با کادر مجرب و پیشرفته‌ترین امکانات."
	if strings.TrimSpace(sectionTitle) != "" {
		desc = fmt.Sprintf("بخش %s با بهره‌گیری از کادر مجرب و پیشرفته‌ترین امکانات تشخیصی و درمانی آماده خدمت‌رسانی به مراجعین گرامی است.", sectionTitle)
	}
	return SectionBanner{
		SectionID:             sectionID,
		Slogan:                "ارائه خدمات تخصصی با بالاترین استانداردهای پزشکی",
		Description:           desc,
		Services:              "پوشش کامل بیمه‌ها\nنوبت‌دهی آنلاین\nکادر تخصصی و مجرب\nپاسخگویی سریع",
		BackgroundColor:       defaultBannerHex,
		BackgroundColorEnd:    "#134e4a",
		UseBackgroundGradient: false,
		BackgroundGradientDir: defaultGradientDir,
		OverlayColor:          defaultBannerHex,
		UseOverlayGradient:    true,
		OverlayOpacityLeft:    defaultOverlayLeft,
		OverlayOpacityBottom:  defaultOverlayBottom,
	}
}
