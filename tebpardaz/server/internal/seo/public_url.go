package seo

import (
	"net"
	"net/http"
	"strings"
)

// RequestInfo داده‌های لازم برای ساخت URL عمومی همان درخواست است.
type RequestInfo struct {
	Host           string
	TLS            bool
	RemoteAddr     string
	ForwardedProto string
	ForwardedHost  string
	// TrustPeer یعنی اتصال TCP از پروکسی پیکربندی‌شده آمده و هدرهای Forwarded قابل استفاده‌اند.
	TrustPeer bool
}

// ProxyTrust فهرست CIDR پروکسی‌هایی است که اجازه دارند scheme و host عمومی را اعلام کنند.
type ProxyTrust struct {
	nets []*net.IPNet
}

// NewProxyTrust فهرست IP یا CIDR را به matcher تبدیل می‌کند.
// ورودی: آدرس‌های پروکسی (IP خالی به /32 یا /128 تکمیل می‌شود).
// خروجی: matcher، یا خطا اگر یکی از مقدارها نامعتبر باشد.
func NewProxyTrust(entries []string) (*ProxyTrust, error) {
	nets := make([]*net.IPNet, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, &net.ParseError{Type: "IP address", Text: entry}
			}
			if ip.To4() != nil {
				entry += "/32"
			} else {
				entry += "/128"
			}
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, err
		}
		nets = append(nets, network)
	}
	return &ProxyTrust{nets: nets}, nil
}

// Trusts گزارش می‌دهد peer این اتصال در فهرست پروکسی مورد اعتماد هست یا نه.
// ورودی: RemoteAddr (با پورت یا بدون پورت). خروجی: true فقط برای IP داخل CIDRها.
func (p *ProxyTrust) Trusts(remoteAddr string) bool {
	if p == nil {
		return false
	}
	ip := peerIP(remoteAddr)
	if ip == nil {
		return false
	}
	for _, network := range p.nets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// RequestInfoFromHTTP فیلدهای URL عمومی را از درخواست HTTP می‌خواند.
// ورودی: درخواست و اینکه peer پروکسی مورد اعتماد است یا نه.
// خروجی: RequestInfo. درخواست nil به مقدار صفر تبدیل می‌شود.
func RequestInfoFromHTTP(r *http.Request, trustPeer bool) RequestInfo {
	if r == nil {
		return RequestInfo{TrustPeer: trustPeer}
	}
	host := r.Host
	if host == "" && r.URL != nil {
		host = r.URL.Host
	}
	proto := ""
	forwardedHost := ""
	if r.Header != nil {
		proto = r.Header.Get("X-Forwarded-Proto")
		forwardedHost = r.Header.Get("X-Forwarded-Host")
	}
	return RequestInfo{
		Host:           host,
		TLS:            r.TLS != nil,
		RemoteAddr:     r.RemoteAddr,
		ForwardedProto: proto,
		ForwardedHost:  forwardedHost,
		TrustPeer:      trustPeer,
	}
}

// PublicScheme schemeای را برمی‌گرداند که کاربر نهایی سایت را با آن می‌بیند.
// ورودی: RequestInfo. خروجی: "https" یا "http".
//
// ترتیب: TLS مستقیم، سپس X-Forwarded-Proto فقط اگر TrustPeer باشد.
// هاست غیرمحلی در غیر این صورت https است چون سایت عمومی پشت پروکسی TLS-terminating منتشر می‌شود
// و هدر جعل‌شده از کلاینت نامعتبر نباید scheme را عوض کند.
// http فقط برای میزبان محلی (توسعه) یا وقتی پروکسی مورد اعتماد صریحاً http گفته و هاست محلی است.
func PublicScheme(info RequestInfo) string {
	if info.TLS {
		return "https"
	}
	if info.TrustPeer {
		switch forwardedValue(info.ForwardedProto) {
		case "https":
			return "https"
		case "http":
			if isLocalHost(publicHost(info)) {
				return "http"
			}
			return "https"
		}
	}
	if isLocalHost(publicHost(info)) {
		return "http"
	}
	return "https"
}

// PublicBaseURL مبدأ عمومی سایت را بدون اسلش انتهایی برمی‌گرداند.
// ورودی: RequestInfo. خروجی: مثلاً https://tebpardaz.ir یا رشته خالی اگر host نباشد.
func PublicBaseURL(info RequestInfo) string {
	host := publicHost(info)
	if host == "" {
		return ""
	}
	return PublicScheme(info) + "://" + host
}

// AbsoluteURL مسیر را به URL مطلق همان مبدأ تبدیل می‌کند.
// ورودی: base بدون اسلش انتهایی، و path. خروجی: URL مطلق.
// path خالی و "/" هر دو ریشه را با اسلش انتهایی می‌سازند.
func AbsoluteURL(baseURL, path string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		if baseURL == "" {
			return "/"
		}
		return baseURL + "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}

// publicHost هاست عمومی را برمی‌گرداند.
// X-Forwarded-Host فقط از پروکسی مورد اعتماد پذیرفته می‌شود تا کلاینت Host را جعل نکند.
func publicHost(info RequestInfo) string {
	if info.TrustPeer {
		if host := forwardedValue(info.ForwardedHost); host != "" {
			return host
		}
	}
	return strings.TrimSpace(info.Host)
}

// forwardedValue اولین مقدار یک هدر تکراری یا جدا شده با ویرگول را برمی‌گرداند.
func forwardedValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, ","); i >= 0 {
		raw = raw[:i]
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

// isLocalHost میزبان توسعه محلی را تشخیص می‌دهد.
func isLocalHost(host string) bool {
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	name = strings.Trim(strings.ToLower(strings.TrimSpace(name)), "[]")
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}

// peerIP بخش IP را از RemoteAddr جدا می‌کند.
func peerIP(remoteAddr string) net.IP {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = host
	}
	return net.ParseIP(strings.Trim(remoteAddr, "[]"))
}
