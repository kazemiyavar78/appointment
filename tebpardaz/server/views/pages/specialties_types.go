package pages

import "tebpardaz/server/views/components"

// SpecialtiesView دادهٔ صفحه فهرست تخصص‌ها را نگه می‌دارد.
type SpecialtiesView struct {
	Specialties []components.SpecialtyCardView
	TotalCount  int
}
