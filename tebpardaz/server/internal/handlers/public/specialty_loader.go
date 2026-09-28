package public

import (
	"math/rand/v2"

	"tebpardaz/server/internal/repository"
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
