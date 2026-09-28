package analytics

import "testing"

func TestReferrerAnalyzer_IsFromGoogle(t *testing.T) {
	a := NewReferrerAnalyzer()
	cases := []struct {
		name     string
		referrer string
		fullURL  string
		want     bool
	}{
		{
			name:     "google search",
			referrer: "https://www.google.com/search?q=%D9%86%D9%88%D8%A8%D8%AA",
			fullURL:  "/",
			want:     true,
		},
		{
			name:     "google.ir",
			referrer: "https://www.google.ir/",
			fullURL:  "/doctors",
			want:     true,
		},
		{
			name:     "google.co.uk",
			referrer: "https://www.google.co.uk/url?sa=t",
			fullURL:  "/",
			want:     true,
		},
		{
			name:     "utm_source google",
			referrer: "",
			fullURL:  "/news?utm_source=google&utm_medium=cpc",
			want:     true,
		},
		{
			name:     "utm_source Google mixed case",
			referrer: "https://example.com/",
			fullURL:  "/booking/x?utm_source=Google",
			want:     true,
		},
		{
			name:     "facebook referrer",
			referrer: "https://www.facebook.com/",
			fullURL:  "/",
			want:     false,
		},
		{
			name:     "empty",
			referrer: "",
			fullURL:  "/",
			want:     false,
		},
		{
			name:     "mygoogle.com is not google",
			referrer: "https://mygoogle.com/landing",
			fullURL:  "/",
			want:     false,
		},
		{
			name:     "utm_source bing",
			referrer: "",
			fullURL:  "/?utm_source=bing",
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := a.IsFromGoogle(tc.referrer, tc.fullURL)
			if got != tc.want {
				t.Fatalf("IsFromGoogle(%q, %q) = %v, want %v", tc.referrer, tc.fullURL, got, tc.want)
			}
		})
	}
}
