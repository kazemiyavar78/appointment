package seo

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOrganizationSurfaceGraphOmitsMissingFields(t *testing.T) {
	raw := AttachOrganizationSurface("", "https://mehrshafaclinics.ir", "موسسه خدمات درمانی بسیجیان", "", "")
	doc := mustGraph(t, raw)
	org := graphNode(t, doc, "Organization")
	site := graphNode(t, doc, "WebSite")
	if org["@id"] != "https://mehrshafaclinics.ir/#organization" {
		t.Fatalf("organization id = %#v", org["@id"])
	}
	if org["url"] != "https://mehrshafaclinics.ir/" || org["name"] != "موسسه خدمات درمانی بسیجیان" {
		t.Fatalf("organization identity = %#v", org)
	}
	if site["@id"] != "https://mehrshafaclinics.ir/#website" {
		t.Fatalf("website id = %#v", site["@id"])
	}
	publisher, _ := site["publisher"].(map[string]interface{})
	if publisher["@id"] != org["@id"] {
		t.Fatalf("publisher = %#v", site["publisher"])
	}
	for _, key := range []string{"logo", "telephone", "address", "legalName"} {
		if _, ok := org[key]; ok {
			t.Fatalf("missing organization field %s was emitted", key)
		}
	}
	if strings.Contains(raw, "05130001111") || strings.Contains(raw, "MedicalOrganization") || strings.Contains(raw, "MedicalClinic") {
		t.Fatalf("clinic data copied into organization graph:\n%s", raw)
	}
}

func TestOrganizationSurfaceKeepsItsOwnDescriptionAndLogo(t *testing.T) {
	raw := AttachOrganizationSurface("", "https://mehrshafaclinics.ir", "موسسه", "توضیح سازمان", "https://mehrshafaclinics.ir/media/org.png")
	org := graphNode(t, mustGraph(t, raw), "Organization")
	if org["description"] != "توضیح سازمان" || org["logo"] != "https://mehrshafaclinics.ir/media/org.png" {
		t.Fatalf("organization fields = %#v", org)
	}
	if _, ok := org["legalName"]; ok {
		t.Fatal("legalName was copied onto a non-platform organization")
	}
}

func TestAttachOrganizationSurfaceSetsStablePublisher(t *testing.T) {
	page := BuildWebPageSchema("https://mehrshafaclinics.ir/doctors#webpage", "https://mehrshafaclinics.ir/doctors", "پزشکان", "", "")
	raw := AttachOrganizationSurface(BuildGraph(page), "https://mehrshafaclinics.ir", "موسسه", "", "")
	doc := mustGraph(t, raw)
	if strings.Contains(raw, "/doctors#organization") || strings.Contains(raw, "/doctors#website") {
		t.Fatalf("page-local organization id:\n%s", raw)
	}
	if countGraphType(doc, "Organization") != 1 || countGraphType(doc, "WebSite") != 1 {
		t.Fatalf("duplicate identity:\n%s", raw)
	}
	webPage := graphNode(t, doc, "WebPage")
	publisher, _ := webPage["publisher"].(map[string]interface{})
	if publisher["@id"] != "https://mehrshafaclinics.ir/#organization" {
		t.Fatalf("webpage publisher = %#v", webPage["publisher"])
	}
}

func TestOrganizationClinicRefIgnoresPage(t *testing.T) {
	id, pageURL := OrganizationClinicRef("https://mehrshafaclinics.ir", "چمران-مشهد")
	if strings.Contains(id, "%25") || strings.Contains(pageURL, "%25") || strings.Contains(id, "page=") {
		t.Fatalf("clinic ref = %s %s", id, pageURL)
	}
	if id != pageURL+"#clinic" || !strings.HasPrefix(pageURL, "https://mehrshafaclinics.ir/clinics/") {
		t.Fatalf("clinic ref = %s %s", id, pageURL)
	}
	emptyID, emptyURL := OrganizationClinicRef("https://mehrshafaclinics.ir", "")
	if emptyID != "" || emptyURL != "" {
		t.Fatal("empty slug fabricated a clinic url")
	}
}

func mustGraph(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, raw)
	}
	return doc
}

func graphNode(t *testing.T, doc map[string]interface{}, typ string) map[string]interface{} {
	t.Helper()
	raw, _ := doc["@graph"].([]interface{})
	for _, item := range raw {
		node, _ := item.(map[string]interface{})
		if node["@type"] == typ {
			return node
		}
	}
	t.Fatalf("missing %s", typ)
	return nil
}

func countGraphType(doc map[string]interface{}, typ string) int {
	raw, _ := doc["@graph"].([]interface{})
	n := 0
	for _, item := range raw {
		node, _ := item.(map[string]interface{})
		if node["@type"] == typ {
			n++
		}
	}
	return n
}
