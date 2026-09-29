package public

import (
	"math/rand/v2"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/views/components"
)

const homeSpecialtyPreviewCount = 6

// loadApprovedSpecialties تخصص‌های تأییدشده و قابل‌نمایش در نوبت‌دهی را از DB بارگذاری می‌کند.
// ورودی: repo تخصص. خروجی: لیست کارت تخصص با نام، آیکون و توضیحات.
func loadApprovedSpecialties(repo *repository.SpecialtyRepo) []components.SpecialtyCardView {
	if repo == nil {
		return nil
	}
	rows, err := repo.ListApproved()
	if err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]components.SpecialtyCardView, 0, len(rows))
	for _, row := range rows {
		out = append(out, components.SpecialtyCardView{
			ID:               row.ID,
			Name:             row.Name,
			ShortDescription: row.ShortDescription,
			Description:      row.Description,
			Icon:             row.Icon,
			Color:            row.Color,
		})
	}
	return out
}

// randomSpecialtySample n تخصص تصادفی از لیست برمی‌گرداند.
// ورودی: لیست تخصص‌ها و تعداد نمونه. خروجی: زیرمجموعه تصادفی (بدون تغییر ترتیب اصلی).
func randomSpecialtySample(items []components.SpecialtyCardView, n int) []components.SpecialtyCardView {
	if n <= 0 || len(items) == 0 {
		return nil
	}
	if len(items) <= n {
		out := append([]components.SpecialtyCardView(nil), items...)
		rand.Shuffle(len(out), func(i, j int) {
			out[i], out[j] = out[j], out[i]
		})
		return out
	}
	indices := rand.Perm(len(items))[:n]
	out := make([]components.SpecialtyCardView, 0, n)
	for _, idx := range indices {
		out = append(out, items[idx])
	}
	return out
}

// applyPlatformSpecialtyHrefs لینک کارت‌های indexable پلتفرم را به landing عوض می‌کند.
// ورودی: کارت‌ها و شمارش پزشک عمومی. خروجی: Href فقط وقتی SpecialtyIndexable باشد.
func applyPlatformSpecialtyHrefs(items []components.SpecialtyCardView, rows []repository.SpecialtyPublicCount) {
	byID := map[uint]string{}
	for _, row := range rows {
		if seo.SpecialtyIndexable(row.Slug, row.DoctorCount) {
			byID[row.ID] = seo.SpecialtyPath(row.Slug)
		}
	}
	for i := range items {
		if href := byID[items[i].ID]; href != "" {
			items[i].Href = href
		}
	}
}

// applyPlatformClinicHrefs لینک کارت مرکز را به landing همان هاست عوض می‌کند.
// ورودی: کارت‌ها و ردیف‌های مرکز. خروجی: Href فقط وقتی ClinicIndexable باشد.
// روی پلتفرم و دامنهٔ سازمان همان مسیر نسبی /clinics/{slug} است.
func applyPlatformClinicHrefs(items []components.ClinicCardView, rows []models.Clinic) {
	byID := map[uint]string{}
	for i := range rows {
		row := &rows[i]
		if seo.ClinicIndexable(row.IsActiveOnWebsite, row.Slug) {
			byID[row.ID] = seo.ClinicPath(*row.Slug)
		}
	}
	for i := range items {
		if href := byID[items[i].ID]; href != "" {
			items[i].Href = href
		}
	}
}

// platformPublicSpecialtyCounts همان شمارش sitemap را برای لینک خانه می‌خواند.
// ورودی: handler خانه. خروجی: تخصص‌های دارای پزشک عمومی. خطا یعنی لینک فیلتر می‌ماند.
func platformPublicSpecialtyCounts(h *HomeHandler) []repository.SpecialtyPublicCount {
	if h == nil || h.Specialties == nil || h.Clinics == nil {
		return nil
	}
	clinics, err := h.Clinics.ListAll()
	if err != nil {
		return nil
	}
	ids := make([]uint, 0, len(clinics))
	for _, clinic := range clinics {
		ids = append(ids, clinic.ID)
	}
	rows, err := h.Specialties.ListWithPublicDoctors(ids)
	if err != nil {
		return nil
	}
	return rows
}
