package components

import "github.com/a-h/templ"

// homeHref returns a safe home URL, defaulting to "/".
func homeHref(url string) templ.SafeURL {
	if url == "" {
		return templ.SafeURL("/")
	}
	return templ.SafeURL(url)
}

// brandInitial returns the first rune of the brand name, or a fallback letter.
func brandInitial(name string) string {
	for _, r := range name {
		return string(r)
	}
	return "ط"
}
