package seo

import (
	"strings"
	"testing"

	"tebpardaz/server/internal/models"
)

func TestApplySupportingSectionPolicyKeepsCanonical(t *testing.T) {
	canonical := "https://tebpardaz.ir/clinics/چمران-مشهد/section/تصویربرداری/ساعات-کاری"
	meta := ApplySupportingSectionPolicy(SectionDetailMeta(SectionHours, "تصویربرداری", "درمانگاه چمران", "", canonical))
	if meta.Robots != RobotsNoindexFollow || meta.Canonical != canonical || strings.Contains(meta.Canonical, "%25") {
		t.Fatalf("supporting meta = %+v", meta)
	}
	primary := SectionDetailMeta(SectionOverview, "تصویربرداری", "درمانگاه چمران", "", "https://tebpardaz.ir/clinics/چمران-مشهد/section/تصویربرداری")
	if primary.Robots != RobotsIndexFollow {
		t.Fatal("primary meta robots changed")
	}
}

func TestSectionDetailIndexableUsesRealContentOnly(t *testing.T) {
	def := models.DefaultSectionBanner(0, "تصویربرداری")
	thin := SectionDetailContent{
		Title:        "تصویربرداری",
		HasBanner:    true,
		Slogan:       def.Slogan,
		Description:  def.Description,
		ServiceLines: nonEmpty(def.Services),
	}
	if SectionDetailIndexable(thin) {
		t.Fatal("default banner was indexable")
	}
	if SectionDetailIndexable(SectionDetailContent{Title: "تصویربرداری"}) {
		t.Fatal("title alone was indexable")
	}
	custom := thin
	custom.Description = "ام‌آرآی و سی‌تی در همین بخش انجام می‌شود."
	if !SectionDetailIndexable(custom) {
		t.Fatal("custom description was not indexable")
	}
	withDoctor := thin
	withDoctor.PublicDoctors = 1
	if !SectionDetailIndexable(withDoctor) {
		t.Fatal("public doctor was not indexable")
	}
	withEquipment := thin
	withEquipment.EquipmentTitles = []string{"سی‌تی اسکن"}
	if !SectionDetailIndexable(withEquipment) {
		t.Fatal("equipment was not indexable")
	}
	withMessage := thin
	withMessage.MessageBodies = []string{"ناشتا مراجعه کنید."}
	if !SectionDetailIndexable(withMessage) {
		t.Fatal("message was not indexable")
	}
	withService := thin
	withService.CatalogServices = 1
	if !SectionDetailIndexable(withService) {
		t.Fatal("catalog service was not indexable")
	}
}

func nonEmpty(raw string) []string {
	return serviceLines(raw)
}

func serviceLines(raw string) []string {
	set := serviceLineSet(raw)
	out := make([]string, 0, len(set))
	for line := range set {
		out = append(out, line)
	}
	return out
}
