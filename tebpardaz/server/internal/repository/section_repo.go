package repository

import (
	"errors"
	"fmt"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// SectionRepo provides database operations for clinic sections and their child entities.
type SectionRepo struct {
	db *gorm.DB
}

// NewSectionRepo initializes a SectionRepo instance with the appointment DB.
// Input: gorm.DB instance for appointment_tapesh.
// Output: pointer to SectionRepo.
func NewSectionRepo(db *gorm.DB) *SectionRepo {
	return &SectionRepo{db: db}
}

// defaultDayNames contains Persian names for days of the week starting with Saturday (0).
var defaultDayNames = [7]string{
	"شنبه",
	"یکشنبه",
	"دوشنبه",
	"سه‌شنبه",
	"چهارشنبه",
	"پنجشنبه",
	"جمعه",
}

// CreateSection creates a new clinic section along with default banner and default 7-day schedule.
// Input: section pointer with ClinicID, Title, Slug, IsActive, SortOrder.
// Output: error if database insert or transaction fails.
func (r *SectionRepo) CreateSection(section *models.AppointmentClinicSection) error {
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	if section == nil {
		return fmt.Errorf("section is nil")
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(section).Error; err != nil {
			return err
		}

		// 1. Auto-create default banner
		banner := models.DefaultSectionBanner(section.ID, section.Title)
		if err := tx.Create(&banner).Error; err != nil {
			return err
		}

		// 2. Auto-create 7-day schedule
		for dayIdx := 0; dayIdx < 7; dayIdx++ {
			dayName := defaultDayNames[dayIdx]
			isOpen := dayIdx != 6   // Friday is closed by default
			hasShift2 := dayIdx < 5 // Saturday through Wednesday have 2 shifts by default
			schedule := models.SectionSchedule{
				SectionID:   section.ID,
				DayOfWeek:   dayIdx,
				DayName:     dayName,
				IsOpen:      isOpen,
				Shift1Start: "08:00",
				Shift1End:   "14:00",
				Shift2Start: "16:00",
				Shift2End:   "20:00",
				HasShift2:   hasShift2,
			}
			if err := tx.Create(&schedule).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// UpdateSection updates basic info of a clinic section (Title, Slug, SortOrder, IsActive).
// Input: section pointer.
// Output: error if database update fails.
func (r *SectionRepo) UpdateSection(section *models.AppointmentClinicSection) error {
	if r.db == nil || section == nil {
		return fmt.Errorf("invalid arguments")
	}
	return r.db.Model(section).Updates(map[string]interface{}{
		"title":      section.Title,
		"slug":       section.Slug,
		"sort_order": section.SortOrder,
		"is_active":  section.IsActive,
	}).Error
}

// DeleteSection deletes a section and its associated banner, schedules, doctors, messages, and equipments.
// Input: section ID and clinic ID for security check.
// Output: error if record not found or deletion fails.
func (r *SectionRepo) DeleteSection(id uint, clinicID uint) error {
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		var section models.AppointmentClinicSection
		if err := tx.Where("id = ? AND clinic_id = ?", id, clinicID).First(&section).Error; err != nil {
			return err
		}

		if err := tx.Where("section_id = ?", id).Delete(&models.SectionBanner{}).Error; err != nil {
			return err
		}
		if err := tx.Where("section_id = ?", id).Delete(&models.SectionSchedule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("section_id = ?", id).Delete(&models.SectionMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("section_id = ?", id).Delete(&models.SectionEquipment{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("section_id = ?", id).Delete(&models.SectionDoctor{}).Error; err != nil {
			return err
		}
		if err := tx.Where("section_id = ?", id).Delete(&models.SectionServicePackage{}).Error; err != nil {
			return err
		}

		return tx.Delete(&section).Error
	})
}

// GetSectionByID finds a section by its primary key ID.
// Input: section ID.
// Output: pointer to ClinicSection or error.
func (r *SectionRepo) GetSectionByID(id uint) (*models.AppointmentClinicSection, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var section models.AppointmentClinicSection
	if err := r.db.Preload("Banner").Preload("Schedules").Preload("Messages").Preload("Equipment").First(&section, id).Error; err != nil {
		return nil, err
	}
	return &section, nil
}

// GetSectionByClinicAndSlug finds an active or inactive section by clinic ID and slug.
// Input: clinic ID and section slug.
// Output: pointer to ClinicSection or error.
func (r *SectionRepo) GetSectionByClinicAndSlug(clinicID uint, slug string) (*models.AppointmentClinicSection, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var section models.AppointmentClinicSection
	if err := r.db.Preload("Banner").Preload("Schedules").Preload("Messages", "is_active = ?", true).Preload("Equipment", "is_active = ?", true).
		Where("clinic_id = ? AND slug = ?", clinicID, slug).
		First(&section).Error; err != nil {
		return nil, err
	}
	return &section, nil
}

// ListSectionsByClinic retrieves all sections for a given clinic.
// Input: clinic ID.
// Output: slice of ClinicSection, ordered by SortOrder then ID, or error.
func (r *SectionRepo) ListSectionsByClinic(clinicID uint) ([]models.AppointmentClinicSection, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var sections []models.AppointmentClinicSection
	err := r.db.Where("clinic_id = ?", clinicID).Order("sort_order asc, id asc").Find(&sections).Error
	return sections, err
}

// ListActiveSectionsByClinic retrieves only active sections for a clinic (for public navbar and pages).
// Input: clinic ID.
// Output: slice of active ClinicSection, ordered by SortOrder then ID, or error.
func (r *SectionRepo) ListActiveSectionsByClinic(clinicID uint) ([]models.AppointmentClinicSection, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var sections []models.AppointmentClinicSection
	err := r.db.Where("clinic_id = ? AND is_active = ?", clinicID, true).Order("sort_order asc, id asc").Find(&sections).Error
	return sections, err
}

// GetBannerBySectionID retrieves the banner for a section.
// Input: section ID.
// Output: pointer to SectionBanner or error.
func (r *SectionRepo) GetBannerBySectionID(sectionID uint) (*models.SectionBanner, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var banner models.SectionBanner
	err := r.db.Where("section_id = ?", sectionID).First(&banner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fallback := models.DefaultSectionBanner(sectionID, "")
		return &fallback, nil
	}
	return &banner, err
}

// SaveBanner creates or updates the banner for a section.
// Input: banner pointer with SectionID and updated fields.
// Output: error if database operation fails.
func (r *SectionRepo) SaveBanner(banner *models.SectionBanner) error {
	if r.db == nil || banner == nil {
		return fmt.Errorf("invalid arguments")
	}
	var existing models.SectionBanner
	err := r.db.Where("section_id = ?", banner.SectionID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.Create(banner).Error
	}
	if err != nil {
		return err
	}
	return r.db.Model(&existing).Updates(map[string]interface{}{
		"slogan":                  banner.Slogan,
		"description":             banner.Description,
		"services":                banner.Services,
		"background_color":        banner.BackgroundColor,
		"image_url":               banner.ImageURL,
		"background_color_end":    banner.BackgroundColorEnd,
		"use_background_gradient": banner.UseBackgroundGradient,
		"background_gradient_dir": banner.BackgroundGradientDir,
		"overlay_color":           banner.OverlayColor,
		"use_overlay_gradient":    banner.UseOverlayGradient,
		"overlay_opacity_left":    banner.OverlayOpacityLeft,
		"overlay_opacity_bottom":  banner.OverlayOpacityBottom,
	}).Error
}

// ListActiveSectionsWithBannerByClinic retrieves active sections with banner preloaded for public cards.
// Input: clinic ID.
// Output: slice of active ClinicSection including Banner, or error.
func (r *SectionRepo) ListActiveSectionsWithBannerByClinic(clinicID uint) ([]models.AppointmentClinicSection, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var sections []models.AppointmentClinicSection
	err := r.db.Preload("Banner").
		Where("clinic_id = ? AND is_active = ?", clinicID, true).
		Order("sort_order asc, id asc").
		Find(&sections).Error
	return sections, err
}

// GetScheduleBySectionID retrieves the 7-day schedule for a section, ordered by DayOfWeek.
// Input: section ID.
// Output: slice of 7 SectionSchedule entries, or error.
func (r *SectionRepo) GetScheduleBySectionID(sectionID uint) ([]models.SectionSchedule, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var list []models.SectionSchedule
	err := r.db.Where("section_id = ?", sectionID).Order("day_of_week asc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		// Auto-initialize if empty
		for dayIdx := 0; dayIdx < 7; dayIdx++ {
			dayName := defaultDayNames[dayIdx]
			isOpen := dayIdx != 6
			hasShift2 := dayIdx < 5
			item := models.SectionSchedule{
				SectionID:   sectionID,
				DayOfWeek:   dayIdx,
				DayName:     dayName,
				IsOpen:      isOpen,
				Shift1Start: "08:00",
				Shift1End:   "14:00",
				Shift2Start: "16:00",
				Shift2End:   "20:00",
				HasShift2:   hasShift2,
			}
			_ = r.db.Create(&item).Error
			list = append(list, item)
		}
	}
	return list, nil
}

// SaveSchedule updates the 7-day schedule for a section.
// Input: section ID and slice of 7 SectionSchedule entries.
// Output: error if database transaction fails.
func (r *SectionRepo) SaveSchedule(sectionID uint, schedules []models.SectionSchedule) error {
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, s := range schedules {
			s.SectionID = sectionID
			if s.DayOfWeek >= 0 && s.DayOfWeek < 7 && s.DayName == "" {
				s.DayName = defaultDayNames[s.DayOfWeek]
			}
			var existing models.SectionSchedule
			err := tx.Where("section_id = ? AND day_of_week = ?", sectionID, s.DayOfWeek).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&s).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				if err := tx.Model(&existing).Updates(map[string]interface{}{
					"day_name":     s.DayName,
					"is_open":      s.IsOpen,
					"shift1_start": s.Shift1Start,
					"shift1_end":   s.Shift1End,
					"shift2_start": s.Shift2Start,
					"shift2_end":   s.Shift2End,
					"has_shift2":   s.HasShift2,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// ListMessagesBySectionID returns all patient messages for a section, ordered by SortOrder then ID.
// Input: section ID.
// Output: slice of SectionMessage, or error.
func (r *SectionRepo) ListMessagesBySectionID(sectionID uint) ([]models.SectionMessage, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var messages []models.SectionMessage
	err := r.db.Where("section_id = ?", sectionID).Order("sort_order asc, id asc").Find(&messages).Error
	return messages, err
}

// ListActiveMessagesBySectionID returns active messages for a section.
// Input: section ID.
// Output: slice of active SectionMessage, or error.
func (r *SectionRepo) ListActiveMessagesBySectionID(sectionID uint) ([]models.SectionMessage, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var messages []models.SectionMessage
	err := r.db.Where("section_id = ? AND is_active = ?", sectionID, true).Order("sort_order asc, id asc").Find(&messages).Error
	return messages, err
}

// GetMessageByID retrieves a single message by ID.
// Input: message ID.
// Output: pointer to SectionMessage, or error.
func (r *SectionRepo) GetMessageByID(id uint) (*models.SectionMessage, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var message models.SectionMessage
	err := r.db.First(&message, id).Error
	return &message, err
}

// CreateMessage adds a new patient message to a section.
// Input: message pointer.
// Output: error if insert fails.
func (r *SectionRepo) CreateMessage(msg *models.SectionMessage) error {
	if r.db == nil || msg == nil {
		return fmt.Errorf("invalid arguments")
	}
	return r.db.Create(msg).Error
}

// UpdateMessage updates an existing patient message.
// Input: message pointer with updated fields.
// Output: error if update fails.
func (r *SectionRepo) UpdateMessage(msg *models.SectionMessage) error {
	if r.db == nil || msg == nil {
		return fmt.Errorf("invalid arguments")
	}
	return r.db.Model(msg).Updates(map[string]interface{}{
		"title":        msg.Title,
		"content":      msg.Content,
		"sender_title": msg.SenderTitle,
		"sort_order":   msg.SortOrder,
		"is_active":    msg.IsActive,
	}).Error
}

// DeleteMessage deletes a patient message by ID and SectionID.
// Input: message ID and section ID.
// Output: error if deletion fails.
func (r *SectionRepo) DeleteMessage(id uint, sectionID uint) error {
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	return r.db.Where("id = ? AND section_id = ?", id, sectionID).Delete(&models.SectionMessage{}).Error
}

// ListEquipmentBySectionID returns all equipment items for a section.
// Input: section ID.
// Output: slice of SectionEquipment, or error.
func (r *SectionRepo) ListEquipmentBySectionID(sectionID uint) ([]models.SectionEquipment, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var items []models.SectionEquipment
	err := r.db.Where("section_id = ?", sectionID).Order("sort_order asc, id asc").Find(&items).Error
	return items, err
}

// ListActiveEquipmentBySectionID returns active equipment items for a section.
// Input: section ID.
// Output: slice of active SectionEquipment, or error.
func (r *SectionRepo) ListActiveEquipmentBySectionID(sectionID uint) ([]models.SectionEquipment, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var items []models.SectionEquipment
	err := r.db.Where("section_id = ? AND is_active = ?", sectionID, true).Order("sort_order asc, id asc").Find(&items).Error
	return items, err
}

// GetEquipmentByID retrieves a single equipment item by ID.
// Input: equipment ID.
// Output: pointer to SectionEquipment, or error.
func (r *SectionRepo) GetEquipmentByID(id uint) (*models.SectionEquipment, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var item models.SectionEquipment
	err := r.db.First(&item, id).Error
	return &item, err
}

// CreateEquipment creates a new equipment introduction item.
// Input: equipment pointer.
// Output: error if insert fails.
func (r *SectionRepo) CreateEquipment(eq *models.SectionEquipment) error {
	if r.db == nil || eq == nil {
		return fmt.Errorf("invalid arguments")
	}
	return r.db.Create(eq).Error
}

// UpdateEquipment updates an existing equipment introduction item.
// Input: equipment pointer with updated fields.
// Output: error if update fails.
func (r *SectionRepo) UpdateEquipment(eq *models.SectionEquipment) error {
	if r.db == nil || eq == nil {
		return fmt.Errorf("invalid arguments")
	}
	return r.db.Model(eq).Updates(map[string]interface{}{
		"quote_title": eq.QuoteTitle,
		"quote_text":  eq.QuoteText,
		"badge":       eq.Badge,
		"image_url":   eq.ImageURL,
		"title":       eq.Title,
		"subtitle":    eq.Subtitle,
		"description": eq.Description,
		"tags":        eq.Tags,
		"button_text": eq.ButtonText,
		"button_url":  eq.ButtonURL,
		"sort_order":  eq.SortOrder,
		"is_active":   eq.IsActive,
	}).Error
}

// DeleteEquipment deletes an equipment item by ID and SectionID.
// Input: equipment ID and section ID.
// Output: error if deletion fails.
func (r *SectionRepo) DeleteEquipment(id uint, sectionID uint) error {
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	return r.db.Where("id = ? AND section_id = ?", id, sectionID).Delete(&models.SectionEquipment{}).Error
}

// DefaultMaxSectionsPerClinic سقف پیش‌فرض تعداد بخش‌های هر مرکز را مشخص می‌کند.
const DefaultMaxSectionsPerClinic = 5

// CountSectionsByClinic تعداد بخش‌های تعریف‌شده برای یک مرکز را شمارش می‌کند.
// ورودی: شناسه کلینیک (clinicID). خروجی: تعداد بخش‌ها و خطای احتمالی دیتابیس.
func (r *SectionRepo) CountSectionsByClinic(clinicID uint) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	var count int64
	err := r.db.Model(&models.AppointmentClinicSection{}).Where("clinic_id = ?", clinicID).Count(&count).Error
	return count, err
}

// GetClinicSectionQuota سقف مجاز بخش‌های یک مرکز را دریافت می‌کند (پیش‌فرض ۵ در صورت عدم تعیین).
// ورودی: شناسه کلینیک (clinicID). خروجی: حداکثر تعداد بخش‌های مجاز و خطای احتمالی.
func (r *SectionRepo) GetClinicSectionQuota(clinicID uint) (int, error) {
	if r.db == nil {
		return DefaultMaxSectionsPerClinic, fmt.Errorf("db not initialized")
	}
	var quota models.ClinicSectionQuota
	err := r.db.Where("clinic_id = ?", clinicID).First(&quota).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DefaultMaxSectionsPerClinic, nil
		}
		return DefaultMaxSectionsPerClinic, err
	}
	if quota.MaxSections <= 0 {
		return DefaultMaxSectionsPerClinic, nil
	}
	return quota.MaxSections, nil
}

// SetClinicSectionQuota سقف مجاز بخش‌های یک مرکز را توسط سوپرادمین ذخیره یا به‌روزرسانی می‌کند.
// ورودی: شناسه کلینیک (clinicID) و حداکثر تعداد مجاز (maxSections). خروجی: خطای احتمالی دیتابیس.
func (r *SectionRepo) SetClinicSectionQuota(clinicID uint, maxSections int) error {
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	if maxSections <= 0 {
		maxSections = DefaultMaxSectionsPerClinic
	}
	var existing models.ClinicSectionQuota
	err := r.db.Where("clinic_id = ?", clinicID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			quota := models.ClinicSectionQuota{
				ClinicID:    clinicID,
				MaxSections: maxSections,
			}
			return r.db.Create(&quota).Error
		}
		return err
	}
	existing.MaxSections = maxSections
	return r.db.Save(&existing).Error
}

// HasSectionByName بررسی می‌کند آیا مرکز بخش فعالی با عنوان مشخص‌شده دارد یا خیر.
// ورودی: شناسه کلینیک (clinicID) و الگوی نام بخش (namePattern). خروجی: وجود بخش و خطای دیتابیس.
func (r *SectionRepo) HasSectionByName(clinicID uint, namePattern string) (bool, error) {
	if r.db == nil || clinicID == 0 {
		return false, nil
	}
	var count int64
	err := r.db.Model(&models.AppointmentClinicSection{}).
		Where("clinic_id = ? AND is_active = ? AND title LIKE ?", clinicID, true, "%"+namePattern+"%").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasLabSection بررسی می‌کند آیا مرکز بخش فعال «آزمایشگاه» دارد یا خیر.
// ورودی: شناسه کلینیک (clinicID). خروجی: بولین نشان‌دهنده وجود آزمایشگاه فعال.
func (r *SectionRepo) HasLabSection(clinicID uint) bool {
	has, _ := r.HasSectionByName(clinicID, "آزمایشگاه")
	return has
}

// HasDoctorSiteSection بررسی می‌کند آیا مرکز بخش فعال «سایت پزشک» دارد یا خیر.
// ورودی: شناسه کلینیک (clinicID). خروجی: بولین نشان‌دهنده وجود سایت پزشک فعال.
func (r *SectionRepo) HasDoctorSiteSection(clinicID uint) bool {
	has, _ := r.HasSectionByName(clinicID, "سایت پزشک")
	return has
}

// ListClinicIDsWithLab لیست شناسه‌های مراکزی را که بخش فعال «آزمایشگاه» دارند برمی‌گرداند.
// ورودی: آرایه شناسه‌های مراکز کاندید (clinicIDs). خروجی: آرایه شناسه‌های دارای آزمایشگاه و خطای احتمالی.
func (r *SectionRepo) ListClinicIDsWithLab(clinicIDs []uint) ([]uint, error) {
	return r.listClinicIDsWithSection(clinicIDs, "آزمایشگاه")
}

// ListClinicIDsWithDoctorSite لیست شناسه‌های مراکزی را که بخش فعال «سایت پزشک» دارند برمی‌گرداند.
// ورودی: آرایه شناسه‌های مراکز کاندید (clinicIDs). خروجی: آرایه شناسه‌های دارای سایت پزشک و خطای احتمالی.
func (r *SectionRepo) ListClinicIDsWithDoctorSite(clinicIDs []uint) ([]uint, error) {
	return r.listClinicIDsWithSection(clinicIDs, "سایت پزشک")
}

// ErrSectionDoctorExists وقتی پزشک از قبل به بخش وصل شده باشد برمی‌گردد.
var ErrSectionDoctorExists = errors.New("doctor already assigned to section")

// ErrSectionDoctorNotEligible وقتی پزشک تأییدشدهٔ همان مرکز نباشد برمی‌گردد.
var ErrSectionDoctorNotEligible = errors.New("doctor is not an approved clinic member")

// ListSectionDoctorsBySectionID انتصاب‌های پزشک یک بخش را با پیش‌بارگذاری پزشک و تخصص برمی‌گرداند.
// ورودی: شناسه بخش. خروجی: ردیف‌های SectionDoctor مرتب‌شده یا خطا.
func (r *SectionRepo) ListSectionDoctorsBySectionID(sectionID uint) ([]models.SectionDoctor, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var links []models.SectionDoctor
	err := r.db.Where("section_id = ?", sectionID).
		Preload("Doctor").
		Preload("Doctor.Specialty").
		Order("sort_order asc, id asc").
		Find(&links).Error
	return links, err
}

// ListAssignedPublicDoctors پزشکان تأییدشده و فعال منتسب به بخش را برای صفحه عمومی برمی‌گرداند.
// ورودی: شناسه بخش. خروجی: پزشکان مرتب‌شده یا خطا.
func (r *SectionRepo) ListAssignedPublicDoctors(sectionID uint) ([]models.Doctor, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	links, err := r.ListSectionDoctorsBySectionID(sectionID)
	if err != nil {
		return nil, err
	}
	return filterPublicSectionDoctors(links), nil
}

// ListAssignableApprovedDoctors پزشکان تأییدشده مرکز را که هنوز به این بخش وصل نشده‌اند برمی‌گرداند.
// ورودی: شناسه بخش و شناسه مرکز. خروجی: پزشکان قابل انتخاب یا خطا.
func (r *SectionRepo) ListAssignableApprovedDoctors(sectionID, clinicID uint) ([]models.Doctor, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var assignedIDs []uint
	if err := r.db.Model(&models.SectionDoctor{}).
		Where("section_id = ?", sectionID).
		Pluck("doctor_id", &assignedIDs).Error; err != nil {
		return nil, err
	}

	q := r.db.Where("clinic_id = ? AND is_approved = ?", clinicID, true)
	if len(assignedIDs) > 0 {
		q = q.Where("id NOT IN ?", assignedIDs)
	}
	var doctors []models.Doctor
	err := q.Preload("Specialty").Order("name asc").Find(&doctors).Error
	return doctors, err
}

// AssignDoctorToSection پزشک تأییدشدهٔ همان مرکز را به بخش وصل می‌کند.
// ورودی: شناسه بخش، شناسه مرکز، شناسه پزشک و ترتیب نمایش. خروجی: خطا در صورت تکرار یا عدم صلاحیت.
func (r *SectionRepo) AssignDoctorToSection(sectionID, clinicID, doctorID uint, sortOrder int) error {
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	if sectionID == 0 || clinicID == 0 || doctorID == 0 {
		return ErrSectionDoctorNotEligible
	}

	var doctor models.Doctor
	if err := r.db.Where("id = ? AND clinic_id = ? AND is_approved = ?", doctorID, clinicID, true).
		First(&doctor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSectionDoctorNotEligible
		}
		return err
	}

	var existing models.SectionDoctor
	err := r.db.Where("section_id = ? AND doctor_id = ?", sectionID, doctorID).First(&existing).Error
	if err == nil {
		return ErrSectionDoctorExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if sortOrder <= 0 {
		var maxOrder int
		_ = r.db.Model(&models.SectionDoctor{}).
			Where("section_id = ?", sectionID).
			Select("COALESCE(MAX(sort_order), 0)").
			Scan(&maxOrder).Error
		sortOrder = maxOrder + 1
	}

	return r.db.Create(&models.SectionDoctor{
		SectionID: sectionID,
		DoctorID:  doctorID,
		SortOrder: sortOrder,
	}).Error
}

// UnassignDoctorFromSection پیوند پزشک و بخش را حذف می‌کند.
// ورودی: شناسه بخش و شناسه پزشک. خروجی: خطای حذف در صورت بروز مشکل دیتابیس.
func (r *SectionRepo) UnassignDoctorFromSection(sectionID, doctorID uint) error {
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	return r.db.Unscoped().Where("section_id = ? AND doctor_id = ?", sectionID, doctorID).
		Delete(&models.SectionDoctor{}).Error
}

// filterPublicSectionDoctors پزشکان تأییدشده و فعال با تخصص قابل‌نمایش در نوبت‌دهی را جدا می‌کند.
// ورودی: ردیف‌های SectionDoctor با پزشک و تخصص پیش‌بارگذاری‌شده. خروجی: اسلایس پزشکان قابل نمایش.
func filterPublicSectionDoctors(links []models.SectionDoctor) []models.Doctor {
	out := make([]models.Doctor, 0, len(links))
	for _, link := range links {
		if link.Doctor.ID == 0 || !link.Doctor.IsApproved || !link.Doctor.IsActive {
			continue
		}
		if !link.Doctor.Specialty.IsVisibleInBooking() {
			continue
		}
		out = append(out, link.Doctor)
	}
	return out
}

// listClinicIDsWithSection متد داخلی برای فیلتر مراکز بر اساس وجود بخش با الگوی نام مشخص است.
// ورودی: آرایه شناسه‌های مراکز و الگوی نام بخش. خروجی: آرایه شناسه‌های منطبق و خطا.
func (r *SectionRepo) listClinicIDsWithSection(clinicIDs []uint, namePattern string) ([]uint, error) {
	if r.db == nil || len(clinicIDs) == 0 {
		return nil, nil
	}
	var matchedIDs []uint
	err := r.db.Model(&models.AppointmentClinicSection{}).
		Where("clinic_id IN ? AND is_active = ? AND title LIKE ?", clinicIDs, true, "%"+namePattern+"%").
		Distinct("clinic_id").
		Pluck("clinic_id", &matchedIDs).Error
	if err != nil {
		return nil, err
	}
	return matchedIDs, nil
}
