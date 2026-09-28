package analytics

import (
	"net/url"
	"strings"
)

// DefaultReferrerAnalyzer detects Google as a traffic source from Referer and UTM params.
type DefaultReferrerAnalyzer struct{}

// NewReferrerAnalyzer constructs the default Google-referrer detector.
// Inputs: none.
// Output: ReferrerAnalyzer implementation.
func NewReferrerAnalyzer() ReferrerAnalyzer {
	return DefaultReferrerAnalyzer{}
}

// IsFromGoogle reports whether the visit came from Google search or a Google-tagged campaign.
// Inputs: referrer (Referer header), fullURL (path + query of the visited page).
// Output: true when the referrer host contains "google." or utm_source=google.
func (DefaultReferrerAnalyzer) IsFromGoogle(referrer, fullURL string) bool {
	if referrerHasGoogleDomain(referrer) {
		return true
	}
	return queryHasGoogleUTM(fullURL)
}

// referrerHasGoogleDomain reports whether the referrer host is a Google property.
func referrerHasGoogleDomain(referrer string) bool {
	ref := strings.TrimSpace(referrer)
	if ref == "" {
		return false
	}
	host := hostFromReferrer(ref)
	if host == "" {
		return false
	}
	return hostLooksLikeGoogle(host)
}

// hostFromReferrer extracts the hostname from a Referer value that may omit a scheme.
func hostFromReferrer(ref string) string {
	parsed, err := url.Parse(ref)
	if err == nil {
		if h := parsed.Hostname(); h != "" {
			return h
		}
		if parsed.Scheme == "" && parsed.Host == "" && parsed.Path != "" {
			if retry, err := url.Parse("https://" + ref); err == nil {
				return retry.Hostname()
			}
		}
	}
	return ""
}

// hostLooksLikeGoogle reports whether host is a Google property (google.com, google.ir, www.google.co.uk, ...).
func hostLooksLikeGoogle(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimSuffix(h, ".")
	if h == "" {
		return false
	}
	if h == "google.com" || strings.HasPrefix(h, "google.") {
		return true
	}
	return strings.Contains(h, ".google.") || strings.HasSuffix(h, ".google.com")
}

// queryHasGoogleUTM reports whether fullURL has utm_source=google (case-insensitive).
func queryHasGoogleUTM(fullURL string) bool {
	raw := strings.TrimSpace(fullURL)
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return utmSourceIsGoogle(raw)
	}
	if parsed.RawQuery == "" && !strings.Contains(raw, "?") {
		if strings.HasPrefix(raw, "?") {
			return utmSourceFromQuery(raw[1:])
		}
		return false
	}
	return utmSourceFromQuery(parsed.RawQuery)
}

// utmSourceFromQuery inspects a raw query string for utm_source=google.
func utmSourceFromQuery(rawQuery string) bool {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return utmSourceIsGoogle(rawQuery)
	}
	return strings.EqualFold(strings.TrimSpace(values.Get("utm_source")), "google")
}

// utmSourceIsGoogle is a fallback substring check when URL parsing fails.
func utmSourceIsGoogle(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "utm_source=google")
}
