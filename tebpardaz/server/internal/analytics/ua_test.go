package analytics

import (
	"strings"
	"testing"
)

func TestUserAgentParser_Browsers(t *testing.T) {
	parser := NewUserAgentParser()
	cases := []struct {
		name    string
		ua      string
		browser string
	}{
		{
			name:    "chrome windows",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			browser: BrowserChrome,
		},
		{
			name:    "firefox windows",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			browser: BrowserFirefox,
		},
		{
			name:    "safari macos",
			ua:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
			browser: BrowserSafari,
		},
		{
			name:    "edge windows",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			browser: BrowserEdge,
		},
		{
			name:    "opera windows",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
			browser: BrowserOpera,
		},
		{
			name:    "samsung internet android",
			ua:      "Mozilla/5.0 (Linux; Android 13; SM-S908B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/23.0 Chrome/110.0.5481.154 Mobile Safari/537.36",
			browser: BrowserSamsung,
		},
		{
			name:    "chrome ios",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1",
			browser: BrowserChrome,
		},
		{
			name:    "unknown curl",
			ua:      "curl/8.0.0",
			browser: BrowserOther,
		},
		{
			name:    "empty",
			ua:      "",
			browser: BrowserOther,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			browser, _ := parser.Parse(tc.ua)
			if browser != tc.browser {
				t.Fatalf("browser = %q, want %q", browser, tc.browser)
			}
			if browser == tc.ua && tc.ua != "" {
				t.Fatal("raw User-Agent must not be stored as browser")
			}
		})
	}
}

func TestUserAgentParser_OS(t *testing.T) {
	parser := NewUserAgentParser()
	cases := []struct {
		name string
		ua   string
		os   string
	}{
		{
			name: "windows 10",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			os:   OSWindows,
		},
		{
			name: "android 13",
			ua:   "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.144 Mobile Safari/537.36",
			os:   OSAndroid,
		},
		{
			name: "ios safari",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			os:   OSIOS,
		},
		{
			name: "macos safari",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
			os:   OSMacOS,
		},
		{
			name: "linux chrome",
			ua:   "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			os:   OSLinux,
		},
		{
			name: "unknown",
			ua:   "SomeCustomClient/1.0",
			os:   OSUnknown,
		},
		{
			name: "empty",
			ua:   "   ",
			os:   OSUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, osName := parser.Parse(tc.ua)
			if osName != tc.os {
				t.Fatalf("os = %q, want %q (ua=%q)", osName, tc.os, tc.ua)
			}
		})
	}
}

func TestUserAgentParser_AndroidNotLinux(t *testing.T) {
	parser := NewUserAgentParser()
	ua := "Mozilla/5.0 (Linux; Android 14; SM-A556E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Mobile Safari/537.36"
	browser, osName := parser.Parse(ua)
	if osName != OSAndroid {
		t.Fatalf("expected Android, got %q", osName)
	}
	if browser != BrowserChrome {
		t.Fatalf("expected Chrome, got %q", browser)
	}
	if strings.Contains(osName, "Linux") && osName != OSLinux {
		t.Fatalf("OS must be the closed-list value, got %q", osName)
	}
}
