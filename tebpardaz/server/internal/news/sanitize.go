package news

import (
	"regexp"
	"strings"
)

var (
	reScript   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reOnEvent  = regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	reJSURL    = regexp.MustCompile(`(?i)(href|src)\s*=\s*("|')\s*javascript:[^"']*("|')`)
	reTagsKeep = regexp.MustCompile(`(?i)</?(?:p|br|div|span|strong|b|em|i|u|h[1-4]|ul|ol|li|img|a|font)(?:\s[^>]*)?>`)
	reStripTag = regexp.MustCompile(`(?i)<[^>]+>`)
)

// SanitizeHTML allows a small rich-text subset (bold, underline, size, color, images, links).
// Inputs: raw HTML from the editor.
// Output: cleaned HTML safe for storage and public render.
func SanitizeHTML(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = reScript.ReplaceAllString(s, "")
	s = reOnEvent.ReplaceAllString(s, "")
	s = reJSURL.ReplaceAllString(s, `$1=$2#$3`)

	var b strings.Builder
	last := 0
	for _, loc := range reStripTag.FindAllStringIndex(s, -1) {
		b.WriteString(s[last:loc[0]])
		tag := s[loc[0]:loc[1]]
		if reTagsKeep.MatchString(tag) {
			b.WriteString(sanitizeAllowedTag(tag))
		}
		last = loc[1]
	}
	b.WriteString(s[last:])
	return strings.TrimSpace(b.String())
}

// PlainTextLength returns approximate visible text length (tags stripped).
// Inputs: html.
// Output: rune count of plain text.
func PlainTextLength(html string) int {
	return len([]rune(reStripTag.ReplaceAllString(html, "")))
}

func sanitizeAllowedTag(tag string) string {
	lower := strings.ToLower(tag)
	if strings.HasPrefix(lower, "<img") {
		return sanitizeImgTag(tag)
	}
	if strings.HasPrefix(lower, "<a") {
		return sanitizeAnchorTag(tag)
	}
	if strings.HasPrefix(lower, "<font") || strings.HasPrefix(lower, "<span") {
		return sanitizeStyleTag(tag)
	}
	// Keep structural tags; drop unknown attributes by rewriting bare names.
	name := tagName(lower)
	if strings.HasPrefix(lower, "</") {
		return "</" + name + ">"
	}
	if strings.HasSuffix(lower, "/>") || name == "br" {
		return "<" + name + ">"
	}
	return "<" + name + ">"
}

func tagName(lowerTag string) string {
	s := strings.TrimPrefix(lowerTag, "</")
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, "/>")
	s = strings.TrimSuffix(s, ">")
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

func sanitizeImgTag(tag string) string {
	src := attrValue(tag, "src")
	if src == "" || strings.HasPrefix(strings.ToLower(src), "javascript:") {
		return ""
	}
	alt := attrValue(tag, "alt")
	// Block layout keeps each image in document order on RTL pages.
	return `<img src="` + htmlEscapeAttr(src) + `" alt="` + htmlEscapeAttr(alt) + `" style="display:block;max-width:100%;height:auto;margin:0.75rem 0;">`
}

func sanitizeAnchorTag(tag string) string {
	href := attrValue(tag, "href")
	if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") {
		return ""
	}
	return `<a href="` + htmlEscapeAttr(href) + `" target="_blank" rel="noopener noreferrer">`
}

func sanitizeStyleTag(tag string) string {
	name := tagName(strings.ToLower(tag))
	if strings.HasPrefix(strings.ToLower(tag), "</") {
		return "</" + name + ">"
	}
	style := attrValue(tag, "style")
	color := attrValue(tag, "color")
	size := attrValue(tag, "size")
	var attrs []string
	if style != "" {
		attrs = append(attrs, `style="`+htmlEscapeAttr(filterStyle(style))+`"`)
	}
	if color != "" && name == "font" {
		attrs = append(attrs, `color="`+htmlEscapeAttr(color)+`"`)
	}
	if size != "" && name == "font" {
		attrs = append(attrs, `size="`+htmlEscapeAttr(size)+`"`)
	}
	if len(attrs) == 0 {
		return "<" + name + ">"
	}
	return "<" + name + " " + strings.Join(attrs, " ") + ">"
}

func filterStyle(style string) string {
	parts := strings.Split(style, ";")
	var keep []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		lp := strings.ToLower(p)
		if strings.HasPrefix(lp, "color:") ||
			strings.HasPrefix(lp, "font-size:") ||
			strings.HasPrefix(lp, "font-weight:") ||
			strings.HasPrefix(lp, "text-decoration:") ||
			strings.HasPrefix(lp, "display:") ||
			strings.HasPrefix(lp, "max-width:") ||
			strings.HasPrefix(lp, "height:") ||
			strings.HasPrefix(lp, "margin:") {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, "; ")
}

func attrValue(tag, name string) string {
	re := regexp.MustCompile(`(?i)\b` + name + `\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
	m := re.FindStringSubmatch(tag)
	if len(m) == 0 {
		return ""
	}
	for i := 2; i < len(m); i++ {
		if m[i] != "" {
			return m[i]
		}
	}
	return ""
}

func htmlEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, `&`, "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, `<`, "&lt;")
	s = strings.ReplaceAll(s, `>`, "&gt;")
	return s
}
