package dashboard

import (
	"net/url"
	"strconv"
	"strings"
)

// PageLabel returns a short Persian name for a tracked public path.
// Inputs: raw visit path (without host).
// Output: human label; unknown paths fall back to the path itself.
func PageLabel(path string) string {
	p := strings.SplitN(strings.TrimSpace(path), "?", 2)[0]
	p = strings.TrimRight(p, "/")
	if p == "" {
		p = "/"
	}
	switch {
	case p == "/":
		return "صفحه اصلی"
	case p == "/doctors" || strings.HasPrefix(p, "/doctors"):
		return "پزشکان"
	case strings.HasPrefix(p, "/booking"):
		return "نوبت‌دهی پزشک"
	case p == "/news" || strings.HasPrefix(p, "/news/"):
		return "اخبار"
	case strings.HasPrefix(p, "/section/") || strings.Contains(p, "/section/"):
		return "بخش درمانی"
	case strings.HasPrefix(p, "/sections") || strings.Contains(p, "/sections"):
		return "فهرست بخش‌ها"
	case strings.Contains(p, "ساعات-کاری") || strings.HasSuffix(p, "/working-hours"):
		return "ساعات کاری"
	case strings.Contains(p, "پیام-به-مراجعین") || strings.HasSuffix(p, "/message-to-visitors"):
		return "پیام به مراجعین"
	case strings.Contains(p, "تجهیزات") || strings.HasSuffix(p, "/equipment"):
		return "تجهیزات"
	case strings.Contains(p, "برنامه-هفتگی") || strings.HasPrefix(p, "/weekly-schedule"):
		return "برنامه هفتگی پزشکان"
	case strings.Contains(p, "/معرفی"):
		return "معرفی بخش"
	case p == "/about":
		return "درباره ما"
	case p == "/contact":
		return "تماس با ما"
	case p == "/terms":
		return "قوانین"
	case strings.HasPrefix(p, "/waiting-queue") || strings.Contains(p, "/waiting-queue"):
		return "وضعیت نوبت"
	case strings.HasPrefix(p, "/test-results"):
		return "جواب آزمایش"
	default:
		return p
	}
}

// DoctorSlugFromPath extracts the doctor slug from a /booking/... visit path.
// Inputs: request path such as /booking/clinic-slug/doctor-slug.
// Output: last non-empty segment, or empty when the path is not a booking page.
func DoctorSlugFromPath(path string) string {
	p := strings.SplitN(strings.TrimSpace(path), "?", 2)[0]
	p = strings.Trim(p, "/")
	if !strings.HasPrefix(p, "booking/") && p != "booking" {
		return ""
	}
	rest := strings.TrimPrefix(p, "booking/")
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return ""
	}
	parts := strings.Split(rest, "/")
	return parts[len(parts)-1]
}

// SpecialtyIDFromURL reads specialty_id from a stored full URL or path+query.
// Inputs: full_url as saved by visit tracking.
// Output: specialty id, or 0 when missing/invalid.
func SpecialtyIDFromURL(fullURL string) uint {
	raw := strings.TrimSpace(fullURL)
	if raw == "" || !strings.Contains(raw, "specialty_id=") {
		return 0
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.RawQuery == "" {
		parsed, err = url.Parse("http://local" + raw)
		if err != nil {
			return 0
		}
	}
	id, err := strconv.ParseUint(parsed.Query().Get("specialty_id"), 10, 64)
	if err != nil {
		return 0
	}
	return uint(id)
}

// RankedNamedCounts sorts named counts descending and keeps the top n.
func RankedNamedCounts(m map[string]NamedCount, n int) []NamedCount {
	out := make([]NamedCount, 0, len(m))
	for _, row := range m {
		if row.Count <= 0 {
			continue
		}
		out = append(out, row)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Count > out[i].Count {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}
