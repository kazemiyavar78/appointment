package booking

import (
	"strings"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/shared/constants"
)

// SlotSource نزدیک‌ترین اسلات قابل‌رزرو پزشکان را بارگذاری می‌کند.
// پیاده‌سازی فعلی: کش در حافظه (۴ ساعت)؛ قبلاً دیتابیس بود.
type SlotSource interface {
	NearestAvailableByDoctors(clinicIDs, doctorIDs []uint, from, to time.Time) (map[uint]models.DoctorSlot, error)
}

// ListFilter پارامترهای فیلتر لیست عمومی رزرو را نگه می‌دارد.
type ListFilter struct {
	ClinicIDs   []uint
	ClinicID    uint // محدودسازی اختیاری به یک مرکز (زیر‌مرکز سازمان)
	SpecialtyID uint
	Query       string
	Date        time.Time // صفر = از الان به بعد؛ مقداردار = فقط همان روز تقویمی
	Layout      constants.LayoutKind
}

// DoctorCard یک ردیف پزشک برای لیست عمومی رزرو است.
type DoctorCard struct {
	ID              uint
	Name            string
	SpecialtyName   string
	PhotoURL        string
	ClinicID        uint
	ClinicName      string
	ClinicSlug      string
	DoctorSlug      string
	ShowClinicBadge bool
	HasSlot         bool
	NearestStartsAt time.Time
	BookingURL      string
}

// SpecialtyOption گزینه دراپ‌داون تخصص است.
type SpecialtyOption struct {
	ID   uint
	Name string
}

// ClinicOption گزینه دراپ‌داون زیر‌مرکز است (مستأجران سازمانی).
type ClinicOption struct {
	ID   uint
	Name string
}

// ListResult payload آماده‌شده لیست رزرو برای هندلر/ویو است.
type ListResult struct {
	Doctors          []DoctorCard
	Specialties      []SpecialtyOption
	Clinics          []ClinicOption
	ShowClinicFilter bool
	ShowClinicBadge  bool
}

// ListingService لیست عمومی رزرو پزشکان را می‌سازد.
type ListingService struct {
	Doctors     *repository.DoctorRepo
	Specialties *repository.SpecialtyRepo
	Clinics     *repository.ClinicRepo
	Slots       SlotSource // منبع اسلات: کش (نه دیتابیس)
}

// NewListingService سازنده ListingService است.
// ورودی: ریپوی پزشک/تخصص/مرکز و SlotSource (کش اسلات‌ها).
// خروجی: اشاره‌گر به ListingService.
func NewListingService(
	doctors *repository.DoctorRepo,
	specialties *repository.SpecialtyRepo,
	clinics *repository.ClinicRepo,
	slots SlotSource,
) *ListingService {
	return &ListingService{
		Doctors:     doctors,
		Specialties: specialties,
		Clinics:     clinics,
		Slots:       slots,
	}
}

// ListDoctors پزشکان فیلتر‌شده را همراه نزدیک‌ترین نوبت از کش برمی‌گرداند.
// ورودی: filter (محدوده مراکز + جستجو + layout برای URL)، showClinicBadge (چیدمان سازمانی).
// خروجی: ListResult یا خطا.
func (s *ListingService) ListDoctors(filter ListFilter, showClinicBadge bool) (*ListResult, error) {
	result := &ListResult{
		ShowClinicBadge:  showClinicBadge,
		ShowClinicFilter: showClinicBadge,
	}

	clinicIDs := filter.ClinicIDs
	if filter.ClinicID > 0 && containsUint(clinicIDs, filter.ClinicID) {
		clinicIDs = []uint{filter.ClinicID}
	}
	if len(clinicIDs) == 0 {
		return result, nil
	}

	if err := s.fillFilterOptions(result, filter.ClinicIDs); err != nil {
		return nil, err
	}

	doctors, err := s.Doctors.ListPublic(repository.DoctorPublicFilter{
		ClinicIDs:   clinicIDs,
		SpecialtyID: filter.SpecialtyID,
		Query:       filter.Query,
	})
	if err != nil {
		return nil, err
	}
	if len(doctors) == 0 {
		return result, nil
	}

	from, to := slotWindow(filter.Date)
	doctorIDs := make([]uint, 0, len(doctors))
	for i := range doctors {
		if err := s.Doctors.EnsureSlug(&doctors[i]); err != nil {
			return nil, err
		}
		doctorIDs = append(doctorIDs, doctors[i].ID)
	}

	// خواندن نزدیک‌ترین اسلات از کش (سینک کلاینت هر ۱ دقیقه / درخواست سرور)
	nearest := map[uint]models.DoctorSlot{}
	if s.Slots != nil {
		nearest, err = s.Slots.NearestAvailableByDoctors(clinicIDs, doctorIDs, from, to)
		if err != nil {
			return nil, err
		}
	}

	metaByClinic := s.clinicMeta(clinicIDs)
	cards := make([]DoctorCard, 0, len(doctors))
	for _, d := range doctors {
		meta := metaByClinic[d.ClinicID]
		card := DoctorCard{
			ID:              d.ID,
			Name:            doctorDisplayName(d),
			SpecialtyName:   d.Specialty.Name,
			PhotoURL:        d.PhotoURL,
			ClinicID:        d.ClinicID,
			ClinicName:      meta.Name,
			ClinicSlug:      meta.Slug,
			DoctorSlug:      d.Slug,
			ShowClinicBadge: showClinicBadge,
			BookingURL:      BuildBookingURL(filter.Layout, meta.Slug, d.Slug),
		}
		if slot, ok := nearest[d.ID]; ok {
			card.HasSlot = true
			card.NearestStartsAt = slot.StartsAt
		}
		cards = append(cards, card)
	}
	result.Doctors = cards
	return result, nil
}

func (s *ListingService) fillFilterOptions(result *ListResult, orgClinicIDs []uint) error {
	if s.Specialties != nil {
		rows, err := s.Specialties.ListApproved()
		if err != nil {
			return err
		}
		for _, row := range rows {
			result.Specialties = append(result.Specialties, SpecialtyOption{ID: row.ID, Name: row.Name})
		}
	}
	if result.ShowClinicFilter && s.Clinics != nil {
		for _, id := range orgClinicIDs {
			c, err := s.Clinics.GetByID(id)
			if err != nil || c == nil {
				continue
			}
			result.Clinics = append(result.Clinics, ClinicOption{ID: c.ID, Name: c.Name})
		}
	}
	return nil
}

type clinicListMeta struct {
	Name string
	Slug string
}

func (s *ListingService) clinicMeta(clinicIDs []uint) map[uint]clinicListMeta {
	out := make(map[uint]clinicListMeta, len(clinicIDs))
	if s.Clinics == nil {
		return out
	}
	for _, id := range clinicIDs {
		c, err := s.Clinics.GetByID(id)
		if err != nil || c == nil {
			continue
		}
		meta := clinicListMeta{Name: c.Name}
		if c.Slug != nil {
			meta.Slug = strings.TrimSpace(*c.Slug)
		}
		out[id] = meta
	}
	return out
}

// slotWindow تاریخ اختیاری تقویمی را به بازه [from, to) برای جستجوی اسلات تبدیل می‌کند.
func slotWindow(date time.Time) (from, to time.Time) {
	now := time.Now()
	if date.IsZero() {
		return now, time.Time{}
	}
	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	from = day
	if from.Before(now) {
		from = now
	}
	to = day.Add(24 * time.Hour)
	return from, to
}

func doctorDisplayName(d models.Doctor) string {
	if name := strings.TrimSpace(d.Name); name != "" {
		return name
	}
	return strings.TrimSpace(d.FirstName + " " + d.LastName)
}

func containsUint(ids []uint, id uint) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
