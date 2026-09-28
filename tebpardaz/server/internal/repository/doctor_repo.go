package repository

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/slug"
	"tebpardaz/server/internal/text"
	"tebpardaz/shared/constants"
	"tebpardaz/shared/protocol"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DoctorRepo provides persistence for doctors, specialties, and approval requests.
type DoctorRepo struct {
	DB *gorm.DB
}

// NewDoctorRepo constructs a DoctorRepo.
func NewDoctorRepo(db *gorm.DB) *DoctorRepo {
	return &DoctorRepo{DB: db}
}

// UpsertResult is one doctor sync outcome returned to the clinic client.
type UpsertResult struct {
	LocalCode  int
	ExternalID string
	DoctorID   uint
	Pending    bool
}

// SyncFromClinic updates approved doctors by clinic_id + national_id and reports which DTOs remain pending.
// Unapproved doctors are never written to the database; callers must store them in cache.
// Inputs: clinicID, doctors from the clinic push.
// Output: results for ack, pending DTOs (not in DB as approved), and error.
func (r *DoctorRepo) SyncFromClinic(clinicID uint, doctors []protocol.DoctorDTO) (results []UpsertResult, pending []protocol.DoctorDTO, err error) {
	results = make([]UpsertResult, 0, len(doctors))
	pending = make([]protocol.DoctorDTO, 0)
	for _, dto := range doctors {
		res, isPending, syncErr := r.syncOne(clinicID, dto)
		if syncErr != nil {
			return results, pending, syncErr
		}
		if isPending {
			pending = append(pending, dto)
			results = append(results, UpsertResult{
				LocalCode:  dto.LocalCode,
				ExternalID: "",
				DoctorID:   0,
				Pending:    true,
			})
			continue
		}
		results = append(results, res)
	}
	return results, pending, nil
}

// syncOne updates an approved doctor matched by national_id (+ clinic) or returns pending=true.
// Inputs: clinicID, dto.
// Output: UpsertResult when updated, pending flag when not in DB as approved, error on failure.
func (r *DoctorRepo) syncOne(clinicID uint, dto protocol.DoctorDTO) (UpsertResult, bool, error) {
	doctor, found, err := r.findApproved(clinicID, dto)
	if err != nil {
		return UpsertResult{}, false, err
	}
	if !found {
		// پزشک تأییدنشده نباید در DB بماند؛ ردیف‌های قدیمی حذف می‌شوند
		_ = r.deleteUnapprovedMatch(clinicID, dto)
		return UpsertResult{}, true, nil
	}

	r.applyHISFields(&doctor, dto)
	if err := r.DB.Save(&doctor).Error; err != nil {
		return UpsertResult{}, false, err
	}
	return UpsertResult{
		LocalCode:  dto.LocalCode,
		ExternalID: doctor.ExternalID,
		DoctorID:   doctor.ID,
		Pending:    false,
	}, false, nil
}

// deleteUnapprovedMatch removes leftover unapproved DB rows matching national_id or local_code.
// Inputs: clinicID, dto.
// Output: error from delete (ignored by callers for sync continuity).
func (r *DoctorRepo) deleteUnapprovedMatch(clinicID uint, dto protocol.DoctorDTO) error {
	q := r.DB.Where("clinic_id = ? AND is_approved = ?", clinicID, false)
	nid := strings.TrimSpace(dto.NationalID)
	switch {
	case nid != "":
		q = q.Where("national_id = ?", nid)
	case dto.LocalCode > 0:
		q = q.Where("local_code = ?", dto.LocalCode)
	default:
		return nil
	}
	return q.Delete(&models.Doctor{}).Error
}

// findApproved locates an approved doctor by clinic_id + national_id (primary), then external_id.
// Inputs: clinicID, dto.
// Output: doctor, found flag, error.
func (r *DoctorRepo) findApproved(clinicID uint, dto protocol.DoctorDTO) (models.Doctor, bool, error) {
	var doctor models.Doctor
	nid := strings.TrimSpace(dto.NationalID)
	if nid != "" {
		tx := r.DB.Where(
			"clinic_id = ? AND national_id = ? AND is_approved = ? AND external_id <> ''",
			clinicID, nid, true,
		).Limit(1).Find(&doctor)
		if tx.Error != nil {
			return doctor, false, tx.Error
		}
		if tx.RowsAffected > 0 {
			return doctor, true, nil
		}
	}
	if dto.ExternalID != "" {
		tx := r.DB.Where(
			"clinic_id = ? AND external_id = ? AND is_approved = ?",
			clinicID, dto.ExternalID, true,
		).Limit(1).Find(&doctor)
		if tx.Error != nil {
			return doctor, false, tx.Error
		}
		if tx.RowsAffected > 0 {
			return doctor, true, nil
		}
	}
	return doctor, false, nil
}

// FindExistingPublic locates an approved doctor by national_id or external_id within a clinic.
// Inputs: clinicID, dto.
// Output: doctor, found flag, error.
func (r *DoctorRepo) FindExistingPublic(clinicID uint, dto protocol.DoctorDTO) (models.Doctor, bool, error) {
	return r.findApproved(clinicID, dto)
}

// applyHISFields copies HIS-sourced fields onto an approved doctor without overwriting admin specialty/photo.
// Inputs: doctor pointer, dto from clinic.
// Output: none (mutates doctor).
func (r *DoctorRepo) applyHISFields(doctor *models.Doctor, dto protocol.DoctorDTO) {
	if name := normalizeDoctorName(dto.Name); name != "" {
		doctor.Name = name
	}
	if firstName := normalizeDoctorName(dto.FirstName); firstName != "" {
		doctor.FirstName = firstName
	}
	if lastName := normalizeDoctorName(dto.LastName); lastName != "" {
		doctor.LastName = lastName
	}
	if dto.Mobile != "" {
		doctor.Mobile = dto.Mobile
	}
	if dto.NationalID != "" {
		doctor.NationalID = dto.NationalID
	}
	if dto.DoctorSystemID > 0 {
		doctor.DoctorSystemID = dto.DoctorSystemID
	}
	if dto.SpecialtyCode != "" {
		doctor.SpecialtyCode = dto.SpecialtyCode
	}
	if dto.LocalCode > 0 {
		doctor.LocalCode = dto.LocalCode
	}
	// IsActive is owned by admin after approval (website visibility); do not overwrite from HIS.
}

// ApproveInput holds admin-assigned fields required to persist a doctor for the first time.
type ApproveInput struct {
	ClinicID       uint
	LocalCode      int
	NationalID     string
	FirstName      string
	LastName       string
	Name           string
	Mobile         string
	DoctorSystemID int
	SpecialtyCode  string
	SpecialtyID    uint
	PhotoURL       string
	Photo300       string
	Photo600       string
	Photo900       string
	Photo1200      string
	UseClinicLogo  bool
	IsActive       bool
	ReviewerID     uint
	Note           string
}

// DoctorPhotosUpdate مقادیر تصاویر در ابعاد مختلف را نگهداری می‌کند.
type DoctorPhotosUpdate struct {
	PhotoURL  string
	Photo300  string
	Photo600  string
	Photo900  string
	Photo1200 string
	// UseClinicLogo اگر غیر nil باشد پرچم استفاده از لوگوی مرکز را تنظیم می‌کند.
	UseClinicLogo *bool
	// LogoURL آدرس لوگوی مرکز است و فقط وقتی UseClinicLogo روشن است نوشته می‌شود.
	LogoURL string
	// ReplacePhotos همهٔ سایزها را بازنویسی می‌کند، حتی اگر خالی باشند.
	ReplacePhotos bool
}

// CreateApproved persists a newly approved doctor (photo + specialty required) and an audit approval row.
// Inputs: ApproveInput with SpecialtyID and PhotoURL set.
// Output: created DoctorApprovalRequest (with ExternalID) or error.
func (r *DoctorRepo) CreateApproved(in ApproveInput) (*models.DoctorApprovalRequest, error) {
	photoURL := strings.TrimSpace(in.PhotoURL)
	photo1200 := strings.TrimSpace(in.Photo1200)
	if photoURL == "" && photo1200 != "" {
		photoURL = photo1200
	}
	if photo1200 == "" && photoURL != "" {
		photo1200 = photoURL
	}
	if photoURL == "" {
		if in.UseClinicLogo {
			return nil, fmt.Errorf("clinic logo required")
		}
		return nil, fmt.Errorf("photo_url required")
	}
	if in.SpecialtyID == 0 {
		return nil, fmt.Errorf("specialty_id required")
	}
	nid := strings.TrimSpace(in.NationalID)
	if nid == "" {
		return nil, fmt.Errorf("national_id required")
	}

	// جلوگیری از تکرار پزشک تأییدشده با همان کد ملی در مرکز
	var existing models.Doctor
	tx := r.DB.Where("clinic_id = ? AND national_id = ? AND is_approved = ?", in.ClinicID, nid, true).
		Limit(1).Find(&existing)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected > 0 {
		return nil, fmt.Errorf("doctor already approved")
	}

	externalID := uuid.NewString()
	name, firstName, lastName, err := resolveApprovedDoctorNames(in.Name, in.FirstName, in.LastName)
	if err != nil {
		return nil, err
	}
	doctor := models.Doctor{
		ClinicID:       in.ClinicID,
		SpecialtyID:    in.SpecialtyID,
		Name:           name,
		FirstName:      firstName,
		LastName:       lastName,
		Mobile:         in.Mobile,
		NationalID:     nid,
		DoctorSystemID: in.DoctorSystemID,
		SpecialtyCode:  in.SpecialtyCode,
		PhotoURL:       photoURL,
		Photo300:       strings.TrimSpace(in.Photo300),
		Photo600:       strings.TrimSpace(in.Photo600),
		Photo900:       strings.TrimSpace(in.Photo900),
		Photo1200:      photo1200,
		UseClinicLogo:  in.UseClinicLogo,
		ExternalID:     externalID,
		LocalCode:      in.LocalCode,
		IsApproved:     true,
		IsActive:       in.IsActive,
	}
	if err := r.DB.Create(&doctor).Error; err != nil {
		return nil, err
	}
	if err := r.EnsureSlug(&doctor); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(in)
	now := time.Now()
	req := models.DoctorApprovalRequest{
		ClinicID:      in.ClinicID,
		DoctorID:      &doctor.ID,
		ExternalID:    externalID,
		RequestedName: name,
		Status:        string(constants.ApprovalApproved),
		PayloadJSON:   string(payload),
		ReviewedBy:    &in.ReviewerID,
		ReviewedAt:    &now,
		Note:          in.Note,
	}
	if err := r.DB.Create(&req).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

// UpdateApprovedProfile عکس‌ها (۴ سایز)، تخصص و توضیحات پزشک تأییدشده را بروزرسانی می‌کند.
// ورودی: doctorID شناسه پزشک، specialtyID شناسه تخصص (۰=بدون تغییر)، photos تصاویر در ابعاد مختلف، shortDesc توضیح کوتاه، longDesc توضیح بلند، setDesc اعمال توضیحات.
// خروجی: رکورد بروزرسانی‌شده پزشک یا خطا در صورت عدم وجود یا اشکال پایگاه‌داده.
func (r *DoctorRepo) UpdateApprovedProfile(doctorID, specialtyID uint, photos DoctorPhotosUpdate, shortDesc, longDesc string, setDesc bool) (*models.Doctor, error) {
	doc, err := r.GetByID(doctorID)
	if err != nil {
		return nil, err
	}
	if doc == nil || !doc.IsApproved {
		return nil, fmt.Errorf("doctor not approved")
	}
	updates := map[string]any{}
	if specialtyID > 0 {
		updates["specialty_id"] = specialtyID
	}
	if photos.UseClinicLogo != nil && *photos.UseClinicLogo {
		logo := strings.TrimSpace(photos.LogoURL)
		if logo == "" {
			return nil, fmt.Errorf("clinic logo required")
		}
		updates["use_clinic_logo"] = true
		updates["photo_url"] = logo
		updates["photo_300"] = logo
		updates["photo_600"] = logo
		updates["photo_900"] = logo
		updates["photo_1200"] = logo
	} else {
		if photos.UseClinicLogo != nil {
			updates["use_clinic_logo"] = false
		}
		if photos.ReplacePhotos {
			url := strings.TrimSpace(photos.PhotoURL)
			photo1200 := strings.TrimSpace(photos.Photo1200)
			if photo1200 == "" {
				photo1200 = url
			}
			if url == "" {
				url = photo1200
			}
			updates["photo_url"] = url
			updates["photo_300"] = strings.TrimSpace(photos.Photo300)
			updates["photo_600"] = strings.TrimSpace(photos.Photo600)
			updates["photo_900"] = strings.TrimSpace(photos.Photo900)
			updates["photo_1200"] = photo1200
		} else {
			if p := strings.TrimSpace(photos.PhotoURL); p != "" {
				updates["photo_url"] = p
			}
			if p := strings.TrimSpace(photos.Photo300); p != "" {
				updates["photo_300"] = p
			}
			if p := strings.TrimSpace(photos.Photo600); p != "" {
				updates["photo_600"] = p
			}
			if p := strings.TrimSpace(photos.Photo900); p != "" {
				updates["photo_900"] = p
			}
			if p := strings.TrimSpace(photos.Photo1200); p != "" {
				updates["photo_1200"] = p
				updates["photo_url"] = p
			} else if p := strings.TrimSpace(photos.PhotoURL); p != "" {
				updates["photo_url"] = p
			}
		}
	}
	if setDesc {
		updates["short_desc"] = strings.TrimSpace(shortDesc)
		updates["long_desc"] = strings.TrimSpace(longDesc)
	}
	if len(updates) == 0 {
		return doc, nil
	}
	// Model را خالی می‌گذاریم تا association ازپیش‌لودشده Specialty
	// مقدار specialty_id را در SaveBeforeAssociations به تخصص قبلی برنگرداند.
	if err := r.DB.Model(&models.Doctor{}).Where("id = ?", doctorID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.GetByID(doctorID)
}

// SetActive sets IsActive for an approved doctor (controls public website visibility).
// Inputs: doctorID, active flag.
// Output: error when doctor missing/unapproved or update fails.
func (r *DoctorRepo) SetActive(doctorID uint, active bool) error {
	doc, err := r.GetByID(doctorID)
	if err != nil {
		return err
	}
	if doc == nil || !doc.IsApproved {
		return fmt.Errorf("doctor not approved")
	}
	return r.DB.Model(doc).Update("is_active", active).Error
}

// DeleteApproved soft-deletes an approved doctor from the server.
// Inputs: doctorID.
// Output: error when doctor missing/unapproved or delete fails.
func (r *DoctorRepo) DeleteApproved(doctorID uint) error {
	doc, err := r.GetByID(doctorID)
	if err != nil {
		return err
	}
	if doc == nil || !doc.IsApproved {
		return fmt.Errorf("doctor not approved")
	}
	return r.DB.Delete(doc).Error
}

// GetByID loads a doctor by primary key.
// Inputs: id.
// Output: doctor with Specialty preloaded, or error.
func (r *DoctorRepo) GetByID(id uint) (*models.Doctor, error) {
	var doctor models.Doctor
	if err := r.DB.Preload("Specialty").First(&doctor, id).Error; err != nil {
		return nil, err
	}
	return &doctor, nil
}

// GetPublicByClinicAndSlug پزشک تأییدشده و فعال را با اسلاگ مرکز برای صفحه رزرو بارگذاری می‌کند.
// ورودی: شناسه مرکز و اسلاگ پزشک. خروجی: پزشک با تخصص، یا ErrRecordNotFound اگر تخصص در نوبت‌دهی مخفی باشد.
func (r *DoctorRepo) GetPublicByClinicAndSlug(clinicID uint, slug string) (*models.Doctor, error) {
	slug = strings.TrimSpace(slug)
	if r == nil || r.DB == nil || clinicID == 0 || slug == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var doctor models.Doctor
	err := r.DB.Where(
		"clinic_id = ? AND slug = ? AND is_approved = ? AND is_active = ? AND external_id <> '' AND specialty_id IN (?)",
		clinicID, slug, true, true, bookableSpecialtyIDsQuery(r.DB),
	).Preload("Specialty").First(&doctor).Error
	if err != nil {
		return nil, err
	}
	return &doctor, nil
}

// EnsureSlug assigns a unique per-clinic slug when missing.
// Inputs: doctor pointer (must have ID and ClinicID).
// Output: error when persistence fails.
func (r *DoctorRepo) EnsureSlug(doctor *models.Doctor) error {
	if r == nil || r.DB == nil || doctor == nil || doctor.ID == 0 {
		return fmt.Errorf("doctor slug ensure unavailable")
	}
	if strings.TrimSpace(doctor.Slug) != "" {
		return nil
	}
	base := slugifyDoctorName(doctor)
	slugVal := base
	for i := 0; i < 100; i++ {
		if i > 0 {
			slugVal = fmt.Sprintf("%s-%d", base, i+1)
		}
		var count int64
		err := r.DB.Model(&models.Doctor{}).
			Where("clinic_id = ? AND slug = ? AND id <> ?", doctor.ClinicID, slugVal, doctor.ID).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count == 0 {
			doctor.Slug = slugVal
			return r.DB.Model(doctor).Update("slug", slugVal).Error
		}
	}
	return fmt.Errorf("unable to allocate unique doctor slug")
}

// resolveApprovedDoctorNames نام قابل‌نمایش پزشک تأییدشده را می‌سازد.
// ورودی: نام، نام کوچک و نام خانوادگی خام. خروجی: سه مقدار نرمال‌شده، یا خطا اگر هیچ نامی نماند.
func resolveApprovedDoctorNames(name, firstName, lastName string) (string, string, string, error) {
	firstName = normalizeDoctorName(firstName)
	lastName = normalizeDoctorName(lastName)
	name = normalizeDoctorName(name)
	if name == "" {
		name = strings.TrimSpace(firstName + " " + lastName)
	}
	if name == "" {
		return "", "", "", fmt.Errorf("doctor name required")
	}
	return name, firstName, lastName, nil
}

// slugifyDoctorName builds a base slug from doctor display name fields.
func slugifyDoctorName(doctor *models.Doctor) string {
	name := strings.TrimSpace(doctor.Name)
	if name == "" {
		name = strings.TrimSpace(doctor.FirstName + " " + doctor.LastName)
	}
	return slug.Make(name)
}

// ListByClinic returns all doctors stored for a clinic (newest first).
// Inputs: clinicID.
// Output: doctor rows or error.
func (r *DoctorRepo) ListByClinic(clinicID uint) ([]models.Doctor, error) {
	var rows []models.Doctor
	err := r.DB.Where("clinic_id = ?", clinicID).
		Preload("Specialty").
		Order("id desc").
		Find(&rows).Error
	return rows, err
}

// ListApprovedByClinic returns approved doctors with an external ID for a clinic.
// Inputs: clinicID.
// Output: approved doctor rows ordered by name.
func (r *DoctorRepo) ListApprovedByClinic(clinicID uint) ([]models.Doctor, error) {
	var rows []models.Doctor
	err := r.DB.Where(
		"clinic_id = ? AND is_approved = ? AND external_id <> ''",
		clinicID, true,
	).
		Preload("Specialty").
		Order("name asc").
		Find(&rows).Error
	return rows, err
}

// DoctorPublicFilter scopes public doctor listing for booking pages.
type DoctorPublicFilter struct {
	ClinicIDs   []uint
	SpecialtyID uint
	Query       string
}

// ListPublic پزشکان تأییدشده و فعال نوبت‌دهی را برمی‌گرداند و تخصص‌های مخفی از نوبت‌دهی را کنار می‌گذارد.
// ورودی: فیلتر (شناسه مراکز الزامی؛ تخصص و نام اختیاری). خروجی: پزشکان با تخصص، مرتب بر اساس نام.
func (r *DoctorRepo) ListPublic(filter DoctorPublicFilter) ([]models.Doctor, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("doctor repo unavailable")
	}
	if len(filter.ClinicIDs) == 0 {
		return nil, nil
	}
	q := r.DB.Where(
		"clinic_id IN ? AND is_approved = ? AND is_active = ? AND external_id <> '' AND specialty_id IN (?)",
		filter.ClinicIDs, true, true, bookableSpecialtyIDsQuery(r.DB),
	)
	if filter.SpecialtyID > 0 {
		q = q.Where("specialty_id = ?", filter.SpecialtyID)
	}
	q = applyDoctorNameSearch(q, filter.Query)
	var rows []models.Doctor
	err := q.Preload("Specialty").Order("name asc").Find(&rows).Error
	return rows, err
}

// bookableSpecialtyIDsQuery شناسه تخصص‌های تأییدشده و قابل‌نمایش در نوبت‌دهی را انتخاب می‌کند.
// ورودی: اتصال GORM. خروجی: زیرپرس‌وجوی id برای استفاده در IN.
func bookableSpecialtyIDsQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&models.Specialty{}).
		Select("id").
		Where("is_approved = ? AND show_in_booking = ?", true, true)
}

// normalizeDoctorName نام پزشک را با قاعدهٔ محافظه‌کارانه نرمال می‌کند.
// ورودی: متن خام. خروجی: متن انسانی. ئ، ة، رقم و slug عوض نمی‌شوند.
func normalizeDoctorName(s string) string {
	return text.NormalizePersianText(s)
}

// applyDoctorNameSearch عبارت نام را با شکل جدید و در صورت تفاوت با شکل legacy جستجو می‌کند.
// ورودی: query جاری و متن کاربر. خروجی: همان query با شرط LIKE. ذخیره را عوض نمی‌کند.
func applyDoctorNameSearch(q *gorm.DB, raw string) *gorm.DB {
	forms := text.SearchLegacyForms(raw)
	if len(forms) == 0 || q == nil {
		return q
	}
	clause := "(name LIKE ? OR first_name LIKE ? OR last_name LIKE ? OR (first_name + ' ' + last_name) LIKE ?)"
	args := make([]any, 0, len(forms)*4)
	parts := make([]string, 0, len(forms))
	for _, form := range forms {
		like := "%" + form + "%"
		parts = append(parts, clause)
		args = append(args, like, like, like, like)
	}
	return q.Where("("+strings.Join(parts, " OR ")+")", args...)
}

// MigrateAllDoctorNamesToPersian نام‌های عربی ذخیره‌شده را به فارسی بروزرسانی می‌کند.
// ورودی: ندارد. خروجی: تعداد ردیف‌های نام‌تغییریافته. slug و شناسه نوشته نمی‌شوند.
func (r *DoctorRepo) MigrateAllDoctorNamesToPersian() (int, error) {
	if r == nil || r.DB == nil {
		return 0, fmt.Errorf("doctor repo unavailable")
	}
	var doctors []models.Doctor
	if err := r.DB.Select("id", "name", "first_name", "last_name").Find(&doctors).Error; err != nil {
		return 0, err
	}
	updated := 0
	for _, doc := range doctors {
		name := normalizeDoctorName(doc.Name)
		firstName := normalizeDoctorName(doc.FirstName)
		lastName := normalizeDoctorName(doc.LastName)
		if name == doc.Name && firstName == doc.FirstName && lastName == doc.LastName {
			continue
		}
		if err := r.DB.Model(&models.Doctor{}).Where("id = ?", doc.ID).Updates(map[string]any{
			"name":       name,
			"first_name": firstName,
			"last_name":  lastName,
		}).Error; err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
