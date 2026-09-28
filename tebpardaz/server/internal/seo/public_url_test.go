package seo

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestPublicSchemeAndBaseURL(t *testing.T) {
	directTLS := RequestInfo{Host: "tebpardaz.ir", TLS: true}
	if got := PublicScheme(directTLS); got != "https" {
		t.Fatalf("direct TLS scheme = %q", got)
	}
	if got := PublicBaseURL(directTLS); got != "https://tebpardaz.ir" {
		t.Fatalf("direct TLS base = %q", got)
	}

	proxied := RequestInfo{
		Host:           "chamranclinic.ir",
		ForwardedProto: "https",
		TrustPeer:      true,
	}
	if got := PublicScheme(proxied); got != "https" {
		t.Fatalf("trusted proxy scheme = %q", got)
	}
	if got := PublicBaseURL(proxied); got != "https://chamranclinic.ir" {
		t.Fatalf("trusted proxy base = %q", got)
	}

	platform := RequestInfo{Host: "tebpardaz.ir"}
	if got := PublicBaseURL(platform); got != "https://tebpardaz.ir" {
		t.Fatalf("platform base = %q", got)
	}

	tenant := RequestInfo{Host: "chamranclinic.ir"}
	if got := PublicBaseURL(tenant); got != "https://chamranclinic.ir" {
		t.Fatalf("tenant base = %q", got)
	}
}

func TestAbsoluteURLPaths(t *testing.T) {
	base := "https://tebpardaz.ir"
	if got := AbsoluteURL(base, "/doctors"); got != "https://tebpardaz.ir/doctors" {
		t.Fatalf("path with slash = %q", got)
	}
	if got := AbsoluteURL(base, ""); got != "https://tebpardaz.ir/" {
		t.Fatalf("empty path = %q", got)
	}
	if got := AbsoluteURL(base, "/"); got != "https://tebpardaz.ir/" {
		t.Fatalf("root path = %q", got)
	}

	tenant := "https://chamranclinic.ir"
	if got := AbsoluteURL(tenant, "/doctors"); got != "https://chamranclinic.ir/doctors" {
		t.Fatalf("tenant path = %q", got)
	}
}

func TestPublicSchemeDoesNotTrustUntrustedForwardedProto(t *testing.T) {
	info := RequestInfo{
		Host:           "localhost:8080",
		ForwardedProto: "https",
		TrustPeer:      false,
	}
	if got := PublicScheme(info); got != "http" {
		t.Fatalf("untrusted localhost scheme = %q, want http", got)
	}
}

func TestPublicHostUsesForwardedHostOnlyFromTrustedPeer(t *testing.T) {
	trusted := RequestInfo{
		Host:          "origin.internal",
		ForwardedHost: "chamranclinic.ir",
		TrustPeer:     true,
	}
	if got := PublicBaseURL(trusted); got != "https://chamranclinic.ir" {
		t.Fatalf("trusted forwarded host = %q", got)
	}

	untrusted := RequestInfo{
		Host:          "tebpardaz.ir",
		ForwardedHost: "evil.example",
		TrustPeer:     false,
	}
	if got := PublicBaseURL(untrusted); got != "https://tebpardaz.ir" {
		t.Fatalf("untrusted forwarded host = %q", got)
	}
}

func TestRequestInfoFromHTTPDirectTLS(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://tebpardaz.ir/doctors", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "tebpardaz.ir"
	req.TLS = &tls.ConnectionState{}
	info := RequestInfoFromHTTP(req, false)
	if !info.TLS {
		t.Fatal("expected TLS")
	}
	if got := AbsoluteURL(PublicBaseURL(info), req.URL.Path); got != "https://tebpardaz.ir/doctors" {
		t.Fatalf("absolute = %q", got)
	}
}

func TestProxyTrust(t *testing.T) {
	trust, err := NewProxyTrust([]string{"127.0.0.1", "::1"})
	if err != nil {
		t.Fatal(err)
	}
	if !trust.Trusts("127.0.0.1:443") {
		t.Fatal("loopback should be trusted")
	}
	if trust.Trusts("203.0.113.5:443") {
		t.Fatal("public peer must not be trusted")
	}
	if (*ProxyTrust)(nil).Trusts("127.0.0.1:1") {
		t.Fatal("nil trust must not trust")
	}
}
