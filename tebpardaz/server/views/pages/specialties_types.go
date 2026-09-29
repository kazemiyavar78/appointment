package pages

import "tebpardaz/server/views/components"

// SpecialtyIndexItem یک تخصص دارای پزشک عمومی در فهرست پلتفرم است.
type SpecialtyIndexItem struct {
	Name        string
	URL         string
	DoctorCount int
}

// SpecialtiesView دادهٔ صفحه فهرست تخصص‌ها را نگه می‌دارد.
type SpecialtiesView struct {
	Items []SpecialtyIndexItem
}

// SpecialtyDetailView دادهٔ landing یک تخصص را نگه می‌دارد.
type SpecialtyDetailView struct {
	Name       string
	Slug       string
	Intro      string
	Doctors    []components.DoctorCardView
	Page       int
	TotalPages int
	TotalCount int
	Empty      bool
}
