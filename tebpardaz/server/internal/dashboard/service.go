package dashboard

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/text"

	ptime "github.com/yaa110/go-persian-calendar"
	"gorm.io/gorm"
)

const topListLimit = 8

// Service loads clinic-home dashboard metrics from live appointment and visit tables.
type Service struct {
	DB    *gorm.DB
	Slots *cache.SlotCache
}

// NewService constructs a dashboard Service.
// Inputs: appointment GORM handle, live slot cache (may be nil).
// Output: pointer to Service.
func NewService(db *gorm.DB, slots *cache.SlotCache) *Service {
	return &Service{DB: db, Slots: slots}
}

type appointmentStats struct {
	Total     int64
	Confirmed int64
	Failed    int64
	Cancelled int64
	Pending   int64
	Doctors   int64
}

type dayCountRow struct {
	Day   time.Time
	Count int64
}

type namedCountRow struct {
	Key   string
	Count int64
}

type hourCountRow struct {
	Hour  int
	Count int64
}

type doctorCatalogRow struct {
	ID            uint
	ClinicID      uint
	Name          string
	FirstName     string
	LastName      string
	Slug          string
	SpecialtyID   uint
	SpecialtyName string
}

type doctorServiceRow struct {
	DoctorID    uint
	ServiceID   uint
	ServiceName string
}

type pathCountRow struct {
	Path    string
	FullURL string
	Count   int64
}

// Load assembles a full dashboard snapshot for the given clinic scope and range.
// Inputs: ctx, scope (allowed clinics), range key, now (clock).
// Output: Snapshot with real metrics, or a database error.
func (s *Service) Load(ctx context.Context, scope Scope, rangeKey RangeKey, now time.Time) (Snapshot, error) {
	period := ResolvePeriod(rangeKey, now)
	snap := Snapshot{
		Range:        period.Key,
		RangeLabel:   period.Label,
		CompareLabel: period.CompareLabel,
		From:         period.From,
		To:           period.To,
		FromDate:     DateKey(period.From),
		ToDate:       DateKey(InclusiveEndDate(period)),
		UpdatedAt:    now,
		UpdatedText:  formatUpdated(now),
		ClinicID:     scope.ClinicID,
		TopPages:     []NamedCount{},
		TopDoctors:   []NamedCount{},
		TopSpecialties: []NamedCount{},
		TopServices:  []NamedCount{},
		PeakHours:    []HourCount{},
		BookingHours: []HourCount{},
		Daily:        []DayPoint{},
		Insights:     []string{},
		Booking: BookingStatus{
			OpenDoctors: []NamedCount{},
		},
	}

	if s == nil || s.DB == nil {
		snap.Insights = []string{"اتصال به پایگاه داده در دسترس نیست."}
		return snap, fmt.Errorf("dashboard db unavailable")
	}
	if !scopeHasClinics(scope) {
		snap.Insights = []string{"هیچ مرکزی برای حساب شما تعریف نشده است."}
		snap.KPIs = BuildKPIs(DefaultMetrics, Values{}, Values{}, period, scope.ClinicID)
		return snap, nil
	}

	curAppt, err := s.loadAppointmentStats(ctx, scope, period.From, period.To)
	if err != nil {
		return snap, err
	}
	prevAppt, err := s.loadAppointmentStats(ctx, scope, period.PrevFrom, period.PrevTo)
	if err != nil {
		return snap, err
	}
	curVisits, err := s.countVisits(ctx, scope, period.From, period.To)
	if err != nil {
		return snap, err
	}
	prevVisits, err := s.countVisits(ctx, scope, period.PrevFrom, period.PrevTo)
	if err != nil {
		return snap, err
	}

	current := Values{
		MetricAppointmentsTotal:     float64(curAppt.Total),
		MetricAppointmentsConfirmed: float64(curAppt.Confirmed),
		MetricAppointmentsFailed:    float64(curAppt.Failed),
		MetricAppointmentsCancelled: float64(curAppt.Cancelled),
		MetricVisits:                float64(curVisits),
		MetricSuccessRate:           RatioPercent(float64(curAppt.Confirmed), float64(curAppt.Total)),
		MetricConversionRate:        RatioPercent(float64(curAppt.Total), float64(curVisits)),
		MetricActiveDoctors:         float64(curAppt.Doctors),
	}
	previous := Values{
		MetricAppointmentsTotal:     float64(prevAppt.Total),
		MetricAppointmentsConfirmed: float64(prevAppt.Confirmed),
		MetricAppointmentsFailed:    float64(prevAppt.Failed),
		MetricAppointmentsCancelled: float64(prevAppt.Cancelled),
		MetricVisits:                float64(prevVisits),
		MetricSuccessRate:           RatioPercent(float64(prevAppt.Confirmed), float64(prevAppt.Total)),
		MetricConversionRate:        RatioPercent(float64(prevAppt.Total), float64(prevVisits)),
		MetricActiveDoctors:         float64(prevAppt.Doctors),
	}
	snap.KPIs = BuildKPIs(DefaultMetrics, current, previous, period, scope.ClinicID)

	daily, err := s.loadDaily(ctx, scope, period)
	if err != nil {
		return snap, err
	}
	snap.Daily = daily

	doctors, err := s.loadDoctorCatalog(ctx, scope)
	if err != nil {
		return snap, err
	}
	bySlug, byID := indexDoctors(doctors)

	pages, bookingPaths, specURLHits, err := s.loadVisitPaths(ctx, scope, period.From, period.To)
	if err != nil {
		return snap, err
	}
	snap.TopPages = pages

	doctorHits := map[string]NamedCount{}
	specHits := map[string]NamedCount{}
	for specID, n := range specURLHits {
		if specID == 0 {
			continue
		}
		label := specialtyNameByID(doctors, specID)
		if label == "" {
			label = "تخصص " + text.ToPersianDigits(fmt.Sprintf("%d", specID))
		}
		key := fmt.Sprintf("spec-%d", specID)
		specHits[key] = NamedCount{Key: key, Label: label, Count: n, Text: text.FormatPersianInt(n)}
	}
	for path, n := range bookingPaths {
		slug := DoctorSlugFromPath(path)
		if slug == "" {
			continue
		}
		doc, ok := bySlug[slug]
		name := slug
		sub := ""
		key := "slug:" + slug
		if ok {
			name = doctorName(doc)
			sub = strings.TrimSpace(doc.SpecialtyName)
			key = fmt.Sprintf("doc-%d", doc.ID)
			if doc.SpecialtyID > 0 {
				sk := fmt.Sprintf("spec-%d", doc.SpecialtyID)
				row := specHits[sk]
				row.Key = sk
				row.Label = doc.SpecialtyName
				if row.Label == "" {
					row.Label = specialtyNameByID(doctors, doc.SpecialtyID)
				}
				row.Count += n
				row.Text = text.FormatPersianInt(row.Count)
				specHits[sk] = row
			}
		}
		row := doctorHits[key]
		row.Key = key
		row.Label = name
		row.Sub = sub
		row.Count += n
		row.Text = text.FormatPersianInt(row.Count)
		if ok {
			row.URL = "/admin/bookings?doctor_hint="
		}
		doctorHits[key] = row
	}
	snap.TopDoctors = RankedNamedCounts(doctorHits, topListLimit)
	snap.TopSpecialties = RankedNamedCounts(specHits, topListLimit)

	services, err := s.rankServices(ctx, doctorHits, byID)
	if err != nil {
		return snap, err
	}
	snap.TopServices = services

	visitHours, err := s.loadHours(ctx, scope, period.From, period.To, true)
	if err != nil {
		return snap, err
	}
	bookHours, err := s.loadHours(ctx, scope, period.From, period.To, false)
	if err != nil {
		return snap, err
	}
	snap.PeakHours = visitHours
	snap.BookingHours = bookHours

	snap.Booking = BookingStatus{
		Confirmed:     curAppt.Confirmed,
		Failed:        curAppt.Failed,
		Cancelled:     curAppt.Cancelled,
		Pending:       curAppt.Pending,
		ConfirmedText: text.FormatPersianInt(curAppt.Confirmed),
		FailedText:    text.FormatPersianInt(curAppt.Failed),
		CancelledText: text.FormatPersianInt(curAppt.Cancelled),
		PendingText:   text.FormatPersianInt(curAppt.Pending),
		OpenDoctors:   s.openCapacityDoctors(scope, byID),
	}
	snap.Insights = BuildInsights(snap)
	return snap, nil
}

// scopeHasClinics reports whether the admin scope can query any clinic or platform visits.
func scopeHasClinics(scope Scope) bool {
	if scope.ClinicID > 0 {
		return true
	}
	return len(scope.ClinicIDs) > 0 || scope.IncludeNullClinicVisits
}

// appointmentQuery starts a clinic-scoped query on patient_appointments.
func (s *Service) appointmentQuery(ctx context.Context, scope Scope) *gorm.DB {
	q := s.DB.WithContext(ctx).Model(&models.PatientAppointment{})
	if scope.ClinicID > 0 {
		return q.Where("clinic_id = ?", scope.ClinicID)
	}
	if len(scope.ClinicIDs) == 0 {
		return q.Where("1 = 0")
	}
	return q.Where("clinic_id IN ?", scope.ClinicIDs)
}

// visitQuery starts a clinic-scoped query on visitor_visit_details.
func (s *Service) visitQuery(ctx context.Context, scope Scope) *gorm.DB {
	q := s.DB.WithContext(ctx).Model(&models.VisitDetail{})
	if scope.ClinicID > 0 {
		return q.Where("clinic_id = ?", scope.ClinicID)
	}
	if scope.IncludeNullClinicVisits && len(scope.ClinicIDs) == 0 {
		return q
	}
	if scope.IncludeNullClinicVisits {
		return q.Where("(clinic_id IN ? OR clinic_id IS NULL)", scope.ClinicIDs)
	}
	if len(scope.ClinicIDs) == 0 {
		return q.Where("1 = 0")
	}
	return q.Where("clinic_id IN ?", scope.ClinicIDs)
}

// loadAppointmentStats counts bookings by status and distinct doctors in [from, to).
func (s *Service) loadAppointmentStats(ctx context.Context, scope Scope, from, to time.Time) (appointmentStats, error) {
	var out appointmentStats
	q := s.appointmentQuery(ctx, scope).Where("created_at >= ? AND created_at < ?", from, to)
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	if err := s.appointmentQuery(ctx, scope).Where("created_at >= ? AND created_at < ?", from, to).
		Select("COUNT(DISTINCT doctor_id)").Scan(&out.Doctors).Error; err != nil {
		return out, err
	}

	type statusRow struct {
		Status string
		Count  int64
	}
	var rows []statusRow
	err := s.appointmentQuery(ctx, scope).
		Select("LOWER(status) as status, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ?", from, to).
		Group("LOWER(status)").
		Scan(&rows).Error
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		switch strings.ToLower(strings.TrimSpace(row.Status)) {
		case "confirmed":
			out.Confirmed = row.Count
		case "failed":
			out.Failed = row.Count
		case "cancelled", "canceled":
			out.Cancelled += row.Count
		case "pending":
			out.Pending = row.Count
		}
	}
	return out, nil
}

// countVisits counts page views in [from, to).
func (s *Service) countVisits(ctx context.Context, scope Scope, from, to time.Time) (int64, error) {
	var n int64
	err := s.visitQuery(ctx, scope).Where("created_at >= ? AND created_at < ?", from, to).Count(&n).Error
	return n, err
}

func (s *Service) loadDaily(ctx context.Context, scope Scope, period Period) ([]DayPoint, error) {
	visitRows, err := s.dayCounts(ctx, true, scope, period.ChartFrom, period.To)
	if err != nil {
		return nil, err
	}
	apptRows, err := s.dayCounts(ctx, false, scope, period.ChartFrom, period.To)
	if err != nil {
		return nil, err
	}
	visits := map[string]int64{}
	appts := map[string]int64{}
	for _, row := range visitRows {
		visits[DateKey(row.Day)] = row.Count
	}
	for _, row := range apptRows {
		appts[DateKey(row.Day)] = row.Count
	}
	out := make([]DayPoint, 0, period.ChartDays)
	for i := 0; i < period.ChartDays; i++ {
		day := period.ChartFrom.AddDate(0, 0, i)
		key := DateKey(day)
		v := visits[key]
		a := appts[key]
		out = append(out, DayPoint{
			Date:         key,
			Label:        text.ToPersianDigits(ptime.New(day).Format("MM/dd")),
			Visits:       v,
			Appointments: a,
			VisitsText:   text.FormatPersianInt(v),
			ApptText:     text.FormatPersianInt(a),
		})
	}
	return out, nil
}

func (s *Service) dayCounts(ctx context.Context, visits bool, scope Scope, from, to time.Time) ([]dayCountRow, error) {
	var rows []dayCountRow
	var q *gorm.DB
	if visits {
		q = s.visitQuery(ctx, scope)
	} else {
		q = s.appointmentQuery(ctx, scope)
	}
	err := q.Select("CAST(created_at AS DATE) as day, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ?", from, to).
		Group("CAST(created_at AS DATE)").
		Order("CAST(created_at AS DATE)").
		Scan(&rows).Error
	return rows, err
}

func (s *Service) loadVisitPaths(ctx context.Context, scope Scope, from, to time.Time) ([]NamedCount, map[string]int64, map[uint]int64, error) {
	var rows []pathCountRow
	err := s.visitQuery(ctx, scope).
		Select("path, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ?", from, to).
		Group("path").
		Order("count desc").
		Limit(50).
		Scan(&rows).Error
	if err != nil {
		return nil, nil, nil, err
	}
	pages := map[string]NamedCount{}
	booking := map[string]int64{}
	for _, row := range rows {
		path := strings.TrimSpace(row.Path)
		if path == "" {
			continue
		}
		label := PageLabel(path)
		item := pages[label]
		item.Key = label
		item.Label = label
		item.Sub = path
		item.Count += row.Count
		item.Text = text.FormatPersianInt(item.Count)
		item.URL = "/admin/visitors/visits?q=" + url.QueryEscape(path)
		pages[label] = item
		if strings.HasPrefix(path, "/booking/") || path == "/booking" {
			booking[path] += row.Count
		}
	}

	var urlRows []pathCountRow
	err = s.visitQuery(ctx, scope).
		Select("full_url, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ? AND full_url LIKE ?", from, to, "%specialty_id=%").
		Group("full_url").
		Scan(&urlRows).Error
	if err != nil {
		return RankedNamedCounts(pages, topListLimit), booking, nil, err
	}
	specHits := map[uint]int64{}
	for _, row := range urlRows {
		id := SpecialtyIDFromURL(row.FullURL)
		if id == 0 {
			continue
		}
		specHits[id] += row.Count
	}
	return RankedNamedCounts(pages, topListLimit), booking, specHits, nil
}

func (s *Service) loadDoctorCatalog(ctx context.Context, scope Scope) ([]doctorCatalogRow, error) {
	q := s.DB.WithContext(ctx).Table("doctors").
		Select("doctors.id, doctors.clinic_id, doctors.name, doctors.first_name, doctors.last_name, doctors.slug, doctors.specialty_id, specialties.name as specialty_name").
		Joins("LEFT JOIN specialties ON specialties.id = doctors.specialty_id AND specialties.deleted_at IS NULL").
		Where("doctors.deleted_at IS NULL AND doctors.is_approved = ?", true)
	if scope.ClinicID > 0 {
		q = q.Where("doctors.clinic_id = ?", scope.ClinicID)
	} else if len(scope.ClinicIDs) > 0 {
		q = q.Where("doctors.clinic_id IN ?", scope.ClinicIDs)
	} else {
		return nil, nil
	}
	var rows []doctorCatalogRow
	err := q.Find(&rows).Error
	return rows, err
}

func (s *Service) rankServices(ctx context.Context, doctorHits map[string]NamedCount, byID map[uint]doctorCatalogRow) ([]NamedCount, error) {
	ids := make([]uint, 0, len(byID))
	hitByDoctor := map[uint]int64{}
	for _, row := range doctorHits {
		var id uint
		if _, err := fmt.Sscanf(row.Key, "doc-%d", &id); err == nil && id > 0 {
			ids = append(ids, id)
			hitByDoctor[id] += row.Count
		}
	}
	if len(ids) == 0 {
		return []NamedCount{}, nil
	}
	var rows []doctorServiceRow
	err := s.DB.WithContext(ctx).Table("doctor_services").
		Select("doctor_services.doctor_id, services.id as service_id, services.name as service_name").
		Joins("JOIN services ON services.id = doctor_services.service_id AND services.deleted_at IS NULL").
		Where("doctor_services.doctor_id IN ?", ids).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	agg := map[string]NamedCount{}
	for _, row := range rows {
		n := hitByDoctor[row.DoctorID]
		if n <= 0 || strings.TrimSpace(row.ServiceName) == "" {
			continue
		}
		key := fmt.Sprintf("svc-%d", row.ServiceID)
		item := agg[key]
		item.Key = key
		item.Label = row.ServiceName
		item.Count += n
		item.Text = text.FormatPersianInt(item.Count)
		agg[key] = item
	}
	return RankedNamedCounts(agg, topListLimit), nil
}

func (s *Service) loadHours(ctx context.Context, scope Scope, from, to time.Time, visits bool) ([]HourCount, error) {
	var q *gorm.DB
	if visits {
		q = s.visitQuery(ctx, scope)
	} else {
		q = s.appointmentQuery(ctx, scope)
	}
	var rows []hourCountRow
	err := q.Select("DATEPART(hour, created_at) as hour, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ?", from, to).
		Group("DATEPART(hour, created_at)").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byHour := map[int]int64{}
	var max int64
	for _, row := range rows {
		if row.Hour < 0 || row.Hour > 23 {
			continue
		}
		byHour[row.Hour] = row.Count
		if row.Count > max {
			max = row.Count
		}
	}
	out := make([]HourCount, 0, 24)
	for h := 0; h < 24; h++ {
		n := byHour[h]
		share := 0
		if max > 0 {
			share = int((n * 100) / max)
		}
		out = append(out, HourCount{
			Hour:  h,
			Label: text.ToPersianDigits(fmt.Sprintf("%02d:00", h)),
			Count: n,
			Text:  text.FormatPersianInt(n),
			Share: share,
		})
	}
	return out, nil
}

func (s *Service) openCapacityDoctors(scope Scope, byID map[uint]doctorCatalogRow) []NamedCount {
	if s.Slots == nil {
		return []NamedCount{}
	}
	clinicIDs := scope.ClinicIDs
	if scope.ClinicID > 0 {
		clinicIDs = []uint{scope.ClinicID}
	}
	now := time.Now()
	agg := map[uint]int64{}
	for _, clinicID := range clinicIDs {
		slots, err := s.Slots.ListByClinic(clinicID)
		if err != nil {
			continue
		}
		for _, slot := range slots {
			if !slot.IsAvailable {
				continue
			}
			if !slot.EndsAt.After(now) {
				continue
			}
			remain := slot.Capacity - slot.BookedCount
			if remain <= 0 {
				continue
			}
			agg[slot.DoctorID] += int64(remain)
		}
	}
	named := map[string]NamedCount{}
	for doctorID, remain := range agg {
		doc, ok := byID[doctorID]
		label := fmt.Sprintf("پزشک %d", doctorID)
		if ok {
			label = doctorName(doc)
		}
		key := fmt.Sprintf("open-%d", doctorID)
		named[key] = NamedCount{
			Key:   key,
			Label: label,
			Sub:   "ظرفیت آزاد",
			Count: remain,
			Text:  text.FormatPersianInt(remain),
			URL:   "/admin/appointments",
		}
	}
	return RankedNamedCounts(named, topListLimit)
}

func indexDoctors(rows []doctorCatalogRow) (map[string]doctorCatalogRow, map[uint]doctorCatalogRow) {
	bySlug := make(map[string]doctorCatalogRow, len(rows))
	byID := make(map[uint]doctorCatalogRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
		if slug := strings.TrimSpace(row.Slug); slug != "" {
			bySlug[slug] = row
		}
	}
	return bySlug, byID
}

func doctorName(row doctorCatalogRow) string {
	if name := strings.TrimSpace(row.Name); name != "" {
		return name
	}
	return strings.TrimSpace(row.FirstName + " " + row.LastName)
}

func specialtyNameByID(rows []doctorCatalogRow, id uint) string {
	for _, row := range rows {
		if row.SpecialtyID == id && strings.TrimSpace(row.SpecialtyName) != "" {
			return row.SpecialtyName
		}
	}
	return ""
}

func formatUpdated(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return text.ToPersianDigits(ptime.New(t).Format("yyyy/MM/dd HH:mm:ss"))
}
