package analytics

import (
	"strings"

	"github.com/mssola/user_agent"
)

// knownBrowserOrder is checked first against the raw UA so Chrome-inside-Edge/Opera
// and Samsung Internet are classified before the generic Chrome token.
var knownBrowserMatchers = []struct {
	needle string
	name   string
}{
	{needle: "samsungbrowser", name: BrowserSamsung},
	{needle: "edg/", name: BrowserEdge},
	{needle: "edgios", name: BrowserEdge},
	{needle: "edga", name: BrowserEdge},
	{needle: "opr/", name: BrowserOpera},
	{needle: "opera", name: BrowserOpera},
	{needle: "fxios", name: BrowserFirefox},
	{needle: "firefox", name: BrowserFirefox},
	{needle: "crios", name: BrowserChrome},
	{needle: "chrome", name: BrowserChrome},
	{needle: "chromium", name: BrowserChrome},
	{needle: "msie", name: BrowserIE},
	{needle: "trident/", name: BrowserIE},
}

var knownBrowserFromLib = map[string]string{
	"chrome":            BrowserChrome,
	"chromium":          BrowserChrome,
	"firefox":           BrowserFirefox,
	"safari":            BrowserSafari,
	"edge":              BrowserEdge,
	"internet explorer": BrowserIE,
	"msie":              BrowserIE,
	"opera":             BrowserOpera,
	"opera mini":        BrowserOpera,
	"samsung browser":   BrowserSamsung,
	"samsung internet":  BrowserSamsung,
}

// MSSOLAUserAgentParser maps User-Agent strings onto the project's closed browser/OS lists.
type MSSOLAUserAgentParser struct{}

// NewUserAgentParser constructs the default User-Agent parser.
// Inputs: none.
// Output: UserAgentParser implementation backed by github.com/mssola/user_agent.
func NewUserAgentParser() UserAgentParser {
	return MSSOLAUserAgentParser{}
}

// Parse extracts a known browser and OS from userAgent.
// Inputs: userAgent (raw User-Agent header; may be empty).
// Output: browser from the allowed list (or Other) and OS from the allowed list (or Unknown).
func (MSSOLAUserAgentParser) Parse(userAgent string) (string, string) {
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		return BrowserOther, OSUnknown
	}
	parsed := user_agent.New(ua)
	libName, _ := parsed.Browser()
	return normalizeBrowser(ua, libName), normalizeOS(ua, parsed)
}

// normalizeBrowser maps a library browser name plus raw UA tokens onto the closed list.
func normalizeBrowser(raw, libName string) string {
	lower := strings.ToLower(raw)
	for _, m := range knownBrowserMatchers {
		if strings.Contains(lower, m.needle) {
			return m.name
		}
	}
	if mapped, ok := knownBrowserFromLib[strings.ToLower(strings.TrimSpace(libName))]; ok {
		return mapped
	}
	if looksLikeSafari(lower) {
		return BrowserSafari
	}
	return BrowserOther
}

// looksLikeSafari reports whether the UA is Safari and not another WebKit browser.
func looksLikeSafari(lowerUA string) bool {
	if !strings.Contains(lowerUA, "safari") {
		return false
	}
	if strings.Contains(lowerUA, "chrome") || strings.Contains(lowerUA, "chromium") ||
		strings.Contains(lowerUA, "android") {
		return false
	}
	return true
}

// normalizeOS maps library OS/platform plus raw UA onto Windows/Android/iOS/macOS/Linux/Unknown.
func normalizeOS(raw string, parsed *user_agent.UserAgent) string {
	lower := strings.ToLower(raw)
	platform := ""
	osName := ""
	if parsed != nil {
		platform = strings.ToLower(parsed.Platform())
		osName = strings.ToLower(parsed.OS())
		info := parsed.OSInfo()
		if info.Name != "" {
			osName = strings.ToLower(info.Name)
		}
	}
	combined := lower + " " + platform + " " + osName

	// Android must be checked before Linux (Android UAs include "Linux").
	if strings.Contains(combined, "android") {
		return OSAndroid
	}
	if strings.Contains(combined, "windows") || strings.Contains(combined, "win32") ||
		strings.Contains(combined, "win64") || strings.Contains(combined, "winnt") {
		return OSWindows
	}
	if strings.Contains(combined, "iphone") || strings.Contains(combined, "ipad") ||
		strings.Contains(combined, "ipod") || strings.Contains(combined, "cpu os") ||
		strings.Contains(combined, "ios") {
		return OSIOS
	}
	if strings.Contains(combined, "macintosh") || strings.Contains(combined, "mac os") ||
		platform == "macintosh" {
		return OSMacOS
	}
	if strings.Contains(combined, "linux") || platform == "linux" || strings.Contains(combined, "x11") {
		return OSLinux
	}
	return OSUnknown
}
