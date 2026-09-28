package public

import (
	"net/http"
	"sort"
	"strings"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"
	"tebpardaz/shared/protocol"

	"github.com/gin-gonic/gin"
)

// WeeklyScheduleHandler صفحه عمومی «لیست نوبت دهی پزشکان هفتگی» را سرو می‌کند.
// داده از کش سمت سرور خوانده می‌شود که کلاینت هر ۱۵ دقیقه بروزرسانی می‌کند.
type WeeklyScheduleHandler struct {
	Clinics *repository.ClinicRepo
	Cache   *cache.WeeklyReserveCache
}

// NewWeeklyScheduleHandler سازنده WeeklyScheduleHandler است.
// ورودی: ریپوی مراکز، کش نوبت هفتگی.
// خروجی: اشاره‌گر به WeeklyScheduleHandler.
func NewWeeklyScheduleHandler(clinics *repository.ClinicRepo, weekly *cache.WeeklyReserveCache) *WeeklyScheduleHandler {
	return &WeeklyScheduleHandler{Clinics: clinics, Cache: weekly}
}

// Get صفحه لیست نوبت هفتگی را برای لایه فعلی رندر می‌کند.
// ورودی: queryهای clinic_id، q (نام پزشک)، date، shift.
// خروجی: HTML صفحه؛ در لایه خصوصی مرکز از مستأجر گرفته می‌شود.
func (h *WeeklyScheduleHandler) Get(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	if tc.Layout != constants.LayoutOrgan && tc.Layout != constants.LayoutPrivate && tc.Layout != constants.LayoutPlatform {
		NotFound(c)
		return
	}

	clinicOptions, showClinicFilter, err := h.clinicOptionsForTenant(tc)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	selectedClinicID := parseUintQuery(c.Query("clinic_id"))
	if !showClinicFilter {
		// لایه خصوصی: فقط همان مرکز مستأجر
		if tc.ClinicID != nil {
			selectedClinicID = *tc.ClinicID
		}
	}

	query := strings.TrimSpace(c.Query("q"))
	dateFilter := normalizeDigits(strings.TrimSpace(c.Query("date")))
	shiftFilter := strings.TrimSpace(c.Query("shift"))

	view := pages.WeeklyScheduleView{
		Title:            "لیست نوبت دهی پزشکان هفتگی",
		ShowClinicFilter: showClinicFilter,
		Clinics:          clinicOptions,
		ClinicID:         selectedClinicID,
		FormAction:       "/weekly-schedule",
		Query:            query,
		Date:             dateFilter,
		Shift:            shiftFilter,
	}

	if selectedClinicID == 0 {
		if showClinicFilter {
			view.InfoMessage = "اول مرکز درمانی را انتخاب کنید، بعد می‌توانید پزشک یا روز را جستجو کنید."
		} else {
			view.EmptyMessage = "مرکز مشخص نیست."
		}
		h.render(c, tc, view, clinicOptions)
		return
	}

	// اعتبارسنجی clinic_id در محدوده مستأجر
	if !clinicIDAllowed(selectedClinicID, clinicOptions, showClinicFilter, tc) {
		view.ClinicID = 0
		view.InfoMessage = "مرکز انتخاب‌شده معتبر نیست. لطفاً دوباره انتخاب کنید."
		h.render(c, tc, view, clinicOptions)
		return
	}

	bag, ok := h.Cache.Get(selectedClinicID)
	if !ok || len(bag.Reserves) == 0 {
		view.EmptyMessage = "هنوز لیست نوبت این مرکز آماده نشده است. کمی بعد دوباره سر بزنید."
		if !bag.UpdatedAt.IsZero() {
			view.UpdatedAt = bag.UpdatedAt.Format("15:04")
		}
		h.render(c, tc, view, clinicOptions)
		return
	}

	filtered := filterWeeklyReserves(bag.Reserves, query, dateFilter, shiftFilter)
	view.DateOptions = weeklyDateOptions(bag.Reserves)
	view.ShiftOptions = weeklyShiftOptions(bag.Reserves)
	view.Shifts = groupWeeklyReserves(filtered, bag.Shifts)
	view.TotalDoctors, _ = countWeeklyRows(view.Shifts)
	if !bag.UpdatedAt.IsZero() {
		view.UpdatedAt = bag.UpdatedAt.Format("15:04")
	}
	if len(view.Shifts) == 0 {
		if query != "" || dateFilter != "" || shiftFilter != "" {
			view.EmptyMessage = "با این جستجو نوبتی پیدا نشد. فیلترها را عوض کنید یا پاک کنید."
		} else {
			view.EmptyMessage = "نوبتی برای نمایش وجود ندارد."
		}
	}
	h.render(c, tc, view, clinicOptions)
}

// render برنامه هفتگی را با متادیتای همان میزبان رندر می‌کند.
// ورودی: کانتکست، مستأجر، ویو و گزینه‌های مرکز. خروجی: ندارد.
// نام مرکز فقط وقتی از داده واقعی انتخاب شده باشد در عنوان می‌آید. فیلتر query ایندکس نمی‌شود.
func (h *WeeklyScheduleHandler) render(c *gin.Context, tc *tenant.Context, view pages.WeeklyScheduleView, options []pages.WeeklyScheduleClinicOption) {
	kind, siteName := publicSite(tc)
	place := weeklyPlaceName(kind, siteName, view.ClinicID, options)
	filtered := seo.WeeklyFiltered(seo.WeeklyQuery{
		Q:        c.Query("q"),
		Date:     c.Query("date"),
		Shift:    c.Query("shift"),
		ClinicID: c.Query("clinic_id"),
	})
	meta := seo.WeeklyMeta(kind, place, requestCanonical(c), filtered)
	RenderPublicLayoutWithHead(c, tc, pages.WeeklySchedule(view), "weekly-schedule", headFromMeta(meta))
}

// weeklyPlaceName نام مرکز انتخاب‌شده یا برند مستأجر را برای عنوان برمی‌گرداند.
// ورودی: نوع سایت، نام برند، شناسه مرکز انتخاب‌شده، گزینه‌ها. خروجی: نام، یا خالی برای پلتفرم بدون انتخاب مرکز.
func weeklyPlaceName(kind seo.SiteKind, siteName string, clinicID uint, options []pages.WeeklyScheduleClinicOption) string {
	if clinicID != 0 {
		for _, opt := range options {
			if opt.ID == clinicID && strings.TrimSpace(opt.Name) != "" {
				return opt.Name
			}
		}
	}
	if kind == seo.SitePlatform {
		return ""
	}
	return siteName
}

// clinicOptionsForTenant گزینه‌های انتخاب مرکز را برای لایه فعلی می‌سازد.
// ورودی: مستأجر فعلی.
// خروجی: گزینه‌ها، نمایش فیلتر مرکز، خطا.
func (h *WeeklyScheduleHandler) clinicOptionsForTenant(tc *tenant.Context) ([]pages.WeeklyScheduleClinicOption, bool, error) {
	clinicIDs, showClinicFilter, err := resolveTenantClinicIDs(tc, h.Clinics)
	if err != nil {
		return nil, showClinicFilter, err
	}
	if len(clinicIDs) == 0 {
		return nil, showClinicFilter, nil
	}

	var rows []models.Clinic
	switch {
	case showClinicFilter && tc.Layout == constants.LayoutPlatform:
		rows, err = h.Clinics.ListAll()
	case showClinicFilter && tc.Layout == constants.LayoutOrgan && tc.OrganizationID != nil:
		rows, err = h.Clinics.ListByOrganizationID(*tc.OrganizationID)
	default:
		// خصوصی یا تک‌مرکز: فقط نام‌ها را از GetByID می‌گیریم
		options := make([]pages.WeeklyScheduleClinicOption, 0, len(clinicIDs))
		for _, id := range clinicIDs {
			clinic, getErr := h.Clinics.GetByID(id)
			if getErr != nil || clinic == nil {
				continue
			}
			options = append(options, pages.WeeklyScheduleClinicOption{ID: clinic.ID, Name: clinic.Name})
		}
		return options, showClinicFilter, nil
	}
	if err != nil {
		return nil, showClinicFilter, err
	}
	options := make([]pages.WeeklyScheduleClinicOption, 0, len(rows))
	for _, clinic := range rows {
		options = append(options, pages.WeeklyScheduleClinicOption{ID: clinic.ID, Name: clinic.Name})
	}
	return options, showClinicFilter, nil
}

// clinicIDAllowed بررسی می‌کند clinic_id در محدوده مستأجر باشد.
func clinicIDAllowed(clinicID uint, options []pages.WeeklyScheduleClinicOption, showFilter bool, tc *tenant.Context) bool {
	if clinicID == 0 {
		return false
	}
	if !showFilter {
		return tc.ClinicID != nil && *tc.ClinicID == clinicID
	}
	for _, opt := range options {
		if opt.ID == clinicID {
			return true
		}
	}
	return false
}

// filterWeeklyReserves ردیف‌ها را بر اساس نام پزشک، تاریخ و شیفت فیلتر می‌کند.
// ورودی: همه رزروها و فیلترهای متنی.
// خروجی: زیر‌مجموعه فیلترشده.
func filterWeeklyReserves(reserves []protocol.WeeklyReserveDTO, query, date, shift string) []protocol.WeeklyReserveDTO {
	query = strings.ToLower(strings.TrimSpace(query))
	date = strings.TrimSpace(date)
	shift = strings.TrimSpace(shift)
	if query == "" && date == "" && shift == "" {
		return reserves
	}
	out := make([]protocol.WeeklyReserveDTO, 0, len(reserves))
	for _, r := range reserves {
		if date != "" && strings.TrimSpace(r.ReserveDate) != date {
			continue
		}
		if shift != "" && !shiftMatches(r.ShiftName, shift) {
			continue
		}
		if query != "" {
			name := strings.ToLower(strings.TrimSpace(r.DoctorFullName))
			spec := strings.ToLower(strings.TrimSpace(r.Speciality))
			if !strings.Contains(name, query) && !strings.Contains(spec, query) {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// shiftMatches تطبیق شیفت انتخاب‌شده با نام شیفت ردیف (کامل یا بخشی مثل «صبح»).
func shiftMatches(rowShift, filter string) bool {
	rowShift = strings.TrimSpace(rowShift)
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	if rowShift == filter {
		return true
	}
	return strings.Contains(rowShift, filter)
}

// weeklyDateOptions گزینه‌های یکتای تاریخ را از داده می‌سازد.
func weeklyDateOptions(reserves []protocol.WeeklyReserveDTO) []pages.WeeklyScheduleFilterOption {
	seen := map[string]struct{}{}
	dates := make([]string, 0)
	for _, r := range reserves {
		d := strings.TrimSpace(r.ReserveDate)
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		dates = append(dates, d)
	}
	sort.Strings(dates)
	out := make([]pages.WeeklyScheduleFilterOption, 0, len(dates))
	for _, d := range dates {
		out = append(out, pages.WeeklyScheduleFilterOption{Value: d, Label: d})
	}
	return out
}

// weeklyShiftOptions گزینه‌های یکتای شیفت را از داده می‌سازد.
func weeklyShiftOptions(reserves []protocol.WeeklyReserveDTO) []pages.WeeklyScheduleFilterOption {
	seen := map[string]struct{}{}
	names := make([]string, 0)
	for _, r := range reserves {
		s := strings.TrimSpace(r.ShiftName)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		names = append(names, s)
	}
	sort.Strings(names)
	out := make([]pages.WeeklyScheduleFilterOption, 0, len(names)+2)
	// میان‌برهای ساده برای کاربر کم‌سواد
	out = append(out,
		pages.WeeklyScheduleFilterOption{Value: "صبح", Label: "فقط صبح"},
		pages.WeeklyScheduleFilterOption{Value: "عصر", Label: "فقط عصر"},
	)
	for _, s := range names {
		out = append(out, pages.WeeklyScheduleFilterOption{Value: s, Label: s})
	}
	return out
}

// countWeeklyRows تعداد کارت‌های پزشک و تعداد سکشن‌ها را می‌شمارد.
func countWeeklyRows(sections []pages.WeeklyScheduleShiftSection) (doctors, sectionsCount int) {
	sectionsCount = len(sections)
	for _, s := range sections {
		doctors += len(s.Rows)
	}
	return doctors, sectionsCount
}

// groupWeeklyReserves ردیف‌ها را بر اساس تاریخ+نام شیفت گروه‌بندی و مرتب می‌کند.
// ورودی: لیست رزروها و نقشه شیفت‌ها از کش.
// خروجی: سکشن‌های نمایش صفحه (بدون ادغام دو تاریخ هم‌نام هفته).
func groupWeeklyReserves(reserves []protocol.WeeklyReserveDTO, shifts map[string][]string) []pages.WeeklyScheduleShiftSection {
	_ = shifts // نقشه کلاینت برای سازگاری نگه داشته می‌شود؛ گروه‌بندی از خود ردیف‌هاست

	type shiftMeta struct {
		key  string
		name string
		date string
	}
	byKey := make(map[string][]pages.WeeklyScheduleRow)
	metas := make([]shiftMeta, 0)

	for _, r := range reserves {
		shiftName := strings.TrimSpace(r.ShiftName)
		date := strings.TrimSpace(r.ReserveDate)
		if shiftName == "" {
			// ساعت نامعتبر/خارج از بازه؛ در UI نشان داده نمی‌شود
			continue
		}
		key := date + "|" + shiftName
		if _, exists := byKey[key]; !exists {
			metas = append(metas, shiftMeta{key: key, name: shiftName, date: date})
		}
		byKey[key] = append(byKey[key], pages.WeeklyScheduleRow{
			DoctorFullName:  r.DoctorFullName,
			Speciality:      r.Speciality,
			ReserveDate:     r.ReserveDate,
			ReserveTime:     r.ReserveTime,
			OutTime:         r.OutTime,
			ReserveCount:    r.ReserveCount,
			ReserveMaxCount: r.ReserveMaxCount,
		})
	}

	sort.Slice(metas, func(i, j int) bool {
		if metas[i].date != metas[j].date {
			return metas[i].date < metas[j].date
		}
		return metas[i].name < metas[j].name
	})

	out := make([]pages.WeeklyScheduleShiftSection, 0, len(metas))
	for _, m := range metas {
		rows := byKey[m.key]
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].ReserveTime != rows[j].ReserveTime {
				return rows[i].ReserveTime < rows[j].ReserveTime
			}
			return rows[i].Speciality < rows[j].Speciality
		})
		out = append(out, pages.WeeklyScheduleShiftSection{
			ShiftName: m.name,
			Date:      m.date,
			Rows:      rows,
		})
	}
	return out
}
