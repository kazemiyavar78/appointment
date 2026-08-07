package repository

import (
	"fmt"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/shared/protocol"

	"gorm.io/gorm"
)

// SlotRepo لایه قدیمی persistence اسلات در دیتابیس است.
// توجه: سینک نوبت‌ها دیگر در DB ذخیره نمی‌شود؛ از cache.SlotCache با TTL چهار ساعته استفاده کنید.
// این فایل صرفاً برای سازگاری/ارجاع احتمالی نگه داشته شده و در مسیر اصلی استفاده نمی‌شود.
type SlotRepo struct {
	DB *gorm.DB
}

// NewSlotRepo سازنده SlotRepo است (منسوخ برای مسیر سینک؛ ترجیحاً SlotCache).
func NewSlotRepo(db *gorm.DB) *SlotRepo {
	return &SlotRepo{DB: db}
}

// UpsertFromPush اسلات‌ها را در دیتابیس می‌نویسد — منسوخ؛ مسیر فعال از کش استفاده می‌کند.
func (r *SlotRepo) UpsertFromPush(clinicID uint, push *protocol.AppointmentListPush, doctors []models.Doctor) (int, error) {
	if r == nil || r.DB == nil || push == nil {
		return 0, nil
	}

	byExternal := make(map[string]models.Doctor, len(doctors))
	doctorIDs := make([]uint, 0, len(doctors))
	for _, doc := range doctors {
		if doc.ExternalID == "" {
			continue
		}
		byExternal[doc.ExternalID] = doc
		doctorIDs = append(doctorIDs, doc.ID)
	}

	if push.FullReplace && len(doctorIDs) > 0 {
		if err := r.DB.Where("clinic_id = ? AND doctor_id IN ?", clinicID, doctorIDs).
			Delete(&models.DoctorSlot{}).Error; err != nil {
			return 0, err
		}
	}

	written := 0
	for _, dto := range push.Appointments {
		doc, ok := byExternal[dto.DoctorExternalID]
		if !ok {
			continue
		}
		slot := models.DoctorSlot{
			DoctorID:       doc.ID,
			ClinicID:       clinicID,
			StartsAt:       time.Unix(dto.StartsAtUnix, 0),
			EndsAt:         time.Unix(dto.EndsAtUnix, 0),
			Capacity:       dto.Capacity,
			BookedCount:    dto.BookedCount,
			IsAvailable:    dto.IsAvailable,
			ExternalSlotID: slotExternalID(dto),
		}
		var existing models.DoctorSlot
		err := r.DB.Where(
			"clinic_id = ? AND doctor_id = ? AND starts_at = ?",
			clinicID, doc.ID, slot.StartsAt,
		).First(&existing).Error
		switch err {
		case nil:
			slot.ID = existing.ID
			slot.CreatedAt = existing.CreatedAt
			if saveErr := r.DB.Save(&slot).Error; saveErr != nil {
				return written, saveErr
			}
		case gorm.ErrRecordNotFound:
			if createErr := r.DB.Create(&slot).Error; createErr != nil {
				return written, createErr
			}
		default:
			return written, err
		}
		written++
	}
	return written, nil
}

// ListByClinic اسلات‌های ذخیره‌شده یک مرکز را از DB برمی‌گرداند (منسوخ).
func (r *SlotRepo) ListByClinic(clinicID uint) ([]models.DoctorSlot, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("slot repo unavailable")
	}
	var rows []models.DoctorSlot
	err := r.DB.Where("clinic_id = ?", clinicID).
		Order("starts_at asc").
		Find(&rows).Error
	return rows, err
}

// NearestAvailableByDoctors نزدیک‌ترین اسلات از DB را برمی‌گرداند (منسوخ؛ مسیر فعال از کش است).
func (r *SlotRepo) NearestAvailableByDoctors(clinicIDs, doctorIDs []uint, from, to time.Time) (map[uint]models.DoctorSlot, error) {
	out := make(map[uint]models.DoctorSlot)
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("slot repo unavailable")
	}
	if len(clinicIDs) == 0 || len(doctorIDs) == 0 {
		return out, nil
	}
	if from.IsZero() {
		from = time.Now()
	}

	q := r.DB.Where(
		"clinic_id IN ? AND doctor_id IN ? AND is_available = ? AND starts_at >= ? AND booked_count < capacity",
		clinicIDs, doctorIDs, true, from,
	)
	if !to.IsZero() {
		q = q.Where("starts_at < ?", to)
	}

	var rows []models.DoctorSlot
	if err := q.Order("starts_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if _, exists := out[row.DoctorID]; exists {
			continue
		}
		out[row.DoctorID] = row
	}
	return out, nil
}

func slotExternalID(dto protocol.AppointmentDTO) string {
	if dto.ExternalSlotID != "" {
		return dto.ExternalSlotID
	}
	return fmt.Sprintf("%s:%d", dto.DoctorExternalID, dto.StartsAtUnix)
}
