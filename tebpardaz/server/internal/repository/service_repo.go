package repository

import (
	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// ServiceRepo عملیات پایگاه داده مربوط به خدمات، انتصاب خدمات به بیمه‌های مرکز و پزشکان را مدیریت می‌کند.
type ServiceRepo struct {
	DB *gorm.DB
}

// DoctorServiceWithInsurances ساختار نگهدارنده اطلاعات یک خدمت و لیست بیمه‌های پوشش‌دهنده آن در مرکز درمانی است.
type DoctorServiceWithInsurances struct {
	Service    models.Service
	Insurances []models.Insurance
}

// NewServiceRepo یک نمونه جدید از ServiceRepo می‌سازد.
// ورودی: handle دیتابیس GORM (مربوط به appointment_tapesh).
// خروجی: اشاره‌گر به ServiceRepo.
func NewServiceRepo(db *gorm.DB) *ServiceRepo {
	return &ServiceRepo{DB: db}
}

// ListAll تمام خدمات ثبت‌شده در سیستم را به ترتیب نام برمی‌گرداند.
// ورودی: ندارد.
// خروجی: اسلایس تمام رکورد‌های Service یا خطای دیتابیس.
func (r *ServiceRepo) ListAll() ([]models.Service, error) {
	var rows []models.Service
	err := r.DB.Order("name asc").Find(&rows).Error
	return rows, err
}

// GetByID یک خدمت را بر اساس شناسه یکتا از دیتابیس دریافت می‌کند.
// ورودی: id شناسه خدمت.
// خروجی: اشاره‌گر به مدل Service یا خطای دیتابیس.
func (r *ServiceRepo) GetByID(id uint) (*models.Service, error) {
	var row models.Service
	if err := r.DB.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Create یک خدمت جدید در کاتالوگ خدمات پلتفرم ایجاد می‌کند.
// ورودی: row مدل خدمت برای ذخیره‌سازی.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) Create(row *models.Service) error {
	return r.DB.Create(row).Error
}

// Update اطلاعات یک خدمت موجود را بروزرسانی می‌کند.
// ورودی: row مدل خدمت حاوی شناسه و فیلدهای تغییریافته.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) Update(row *models.Service) error {
	return r.DB.Save(row).Error
}

// Delete یک خدمت و تمام انتصاب‌های وابسته به آن (به بیمه‌های مراکز و پزشکان) را در قالب تراکنش حذف می‌کند.
// ورودی: id شناسه خدمت.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) Delete(id uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("service_id = ?", id).Delete(&models.ClinicInsuranceService{}).Error; err != nil {
			return err
		}
		if err := tx.Where("service_id = ?", id).Delete(&models.DoctorService{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Service{}, id).Error
	})
}

// ListServicesByDoctorID لیست خدمات انتصاب داده شده به یک پزشک خاص را برمی‌گرداند.
// ورودی: doctorID شناسه پزشک.
// خروجی: لیست مدل‌های Service یا خطای دیتابیس.
func (r *ServiceRepo) ListServicesByDoctorID(doctorID uint) ([]models.Service, error) {
	var serviceIDs []uint
	if err := r.DB.Model(&models.DoctorService{}).
		Where("doctor_id = ?", doctorID).
		Pluck("service_id", &serviceIDs).Error; err != nil {
		return nil, err
	}
	if len(serviceIDs) == 0 {
		return nil, nil
	}
	var rows []models.Service
	err := r.DB.Where("id IN ?", serviceIDs).Order("name asc").Find(&rows).Error
	return rows, err
}

// ListServiceIDsByDoctorID شناسه‌های خدمات انتصاب داده شده به یک پزشک را برمی‌گرداند.
// ورودی: doctorID شناسه پزشک.
// خروجی: اسلایس شناسه‌های عددی خدمات یا خطای دیتابیس.
func (r *ServiceRepo) ListServiceIDsByDoctorID(doctorID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&models.DoctorService{}).
		Where("doctor_id = ?", doctorID).
		Pluck("service_id", &ids).Error
	return ids, err
}

// ReplaceDoctorServices خدمات انتصاب داده شده به یک پزشک را جایگزین می‌کند.
// ورودی: doctorID شناسه پزشک، serviceIDs لیست شناسه‌های جدید خدمات.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) ReplaceDoctorServices(doctorID uint, serviceIDs []uint) error {
	return r.ReplaceDoctorsServices([]uint{doctorID}, serviceIDs)
}

// ReplaceDoctorsServices خدمات انتصاب داده شده به چند پزشک را در قالب یک تراکنش جایگزین می‌کند.
// ورودی: doctorIDs لیست شناسه‌های پزشکان، serviceIDs لیست شناسه‌های خدمات جدید.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) ReplaceDoctorsServices(doctorIDs []uint, serviceIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if len(doctorIDs) == 0 {
			return nil
		}
		if err := tx.Where("doctor_id IN ?", doctorIDs).Delete(&models.DoctorService{}).Error; err != nil {
			return err
		}
		if len(serviceIDs) == 0 {
			return nil
		}
		rows := make([]models.DoctorService, 0, len(doctorIDs)*len(serviceIDs))
		for _, docID := range doctorIDs {
			if docID == 0 {
				continue
			}
			for _, id := range serviceIDs {
				if id == 0 {
					continue
				}
				rows = append(rows, models.DoctorService{DoctorID: docID, ServiceID: id})
			}
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

// ListServicesByClinicAndInsurance لیست خدمات انتصاب داده شده به یک بیمه خاص در یک مرکز را برمی‌گرداند.
// ورودی: clinicID شناسه مرکز درمانی، insuranceID شناسه بیمه.
// خروجی: لیست مدل‌های Service یا خطای دیتابیس.
func (r *ServiceRepo) ListServicesByClinicAndInsurance(clinicID uint, insuranceID uint) ([]models.Service, error) {
	var serviceIDs []uint
	if err := r.DB.Model(&models.ClinicInsuranceService{}).
		Where("clinic_id = ? AND insurance_id = ?", clinicID, insuranceID).
		Pluck("service_id", &serviceIDs).Error; err != nil {
		return nil, err
	}
	if len(serviceIDs) == 0 {
		return nil, nil
	}
	var rows []models.Service
	err := r.DB.Where("id IN ?", serviceIDs).Order("name asc").Find(&rows).Error
	return rows, err
}

// ListServiceIDsByClinicAndInsurance شناسه‌های خدمات انتصاب داده شده به یک بیمه در یک مرکز را برمی‌گرداند.
// ورودی: clinicID شناسه مرکز درمانی، insuranceID شناسه بیمه.
// خروجی: اسلایس شناسه‌های عددی خدمات یا خطای دیتابیس.
func (r *ServiceRepo) ListServiceIDsByClinicAndInsurance(clinicID uint, insuranceID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&models.ClinicInsuranceService{}).
		Where("clinic_id = ? AND insurance_id = ?", clinicID, insuranceID).
		Pluck("service_id", &ids).Error
	return ids, err
}

// ReplaceClinicInsuranceServices خدمات انتصاب داده شده به بیمه یک مرکز را جایگزین می‌کند.
// ورودی: clinicID شناسه مرکز، insuranceID شناسه بیمه، serviceIDs لیست شناسه‌های جدید خدمات.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) ReplaceClinicInsuranceServices(clinicID uint, insuranceID uint, serviceIDs []uint) error {
	return r.ReplaceClinicInsurancesServices(clinicID, []uint{insuranceID}, serviceIDs)
}

// ReplaceClinicInsurancesServices خدمات انتصاب داده شده به چند بیمه در یک مرکز را در قالب تراکنش جایگزین می‌کند.
// ورودی: clinicID شناسه مرکز، insuranceIDs لیست شناسه‌های بیمه‌ها، serviceIDs لیست شناسه‌های جدید خدمات.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) ReplaceClinicInsurancesServices(clinicID uint, insuranceIDs []uint, serviceIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if len(insuranceIDs) == 0 {
			return nil
		}
		if err := tx.Where("clinic_id = ? AND insurance_id IN ?", clinicID, insuranceIDs).
			Delete(&models.ClinicInsuranceService{}).Error; err != nil {
			return err
		}
		if len(serviceIDs) == 0 {
			return nil
		}
		rows := make([]models.ClinicInsuranceService, 0, len(insuranceIDs)*len(serviceIDs))
		for _, insID := range insuranceIDs {
			if insID == 0 {
				continue
			}
			for _, id := range serviceIDs {
				if id == 0 {
					continue
				}
				rows = append(rows, models.ClinicInsuranceService{
					ClinicID:    clinicID,
					InsuranceID: insID,
					ServiceID:   id,
				})
			}
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

// GetDoctorServicesWithInsurances خدمات تحت پوشش پزشک و بیمه‌هایی از مرکز که آن خدمات را پوشش می‌دهند برمی‌گرداند.
// ورودی: clinicID شناسه مرکز درمانی، doctorID شناسه پزشک.
// خروجی: اسلایس DoctorServiceWithInsurances شامل اطلاعات خدمت و بیمه‌های پشتیبان یا خطای دیتابیس.
func (r *ServiceRepo) GetDoctorServicesWithInsurances(clinicID uint, doctorID uint) ([]DoctorServiceWithInsurances, error) {
	doctorServices, err := r.ListServicesByDoctorID(doctorID)
	if err != nil {
		return nil, err
	}
	if len(doctorServices) == 0 {
		return nil, nil
	}

	serviceIDs := make([]uint, 0, len(doctorServices))
	for _, s := range doctorServices {
		serviceIDs = append(serviceIDs, s.ID)
	}

	// دریافت تمام رکوردهای بیمه-خدمت این مرکز برای خدمات این پزشک
	var cisRows []models.ClinicInsuranceService
	if err := r.DB.Where("clinic_id = ? AND service_id IN ?", clinicID, serviceIDs).
		Find(&cisRows).Error; err != nil {
		return nil, err
	}

	// دریافت لیست متمایز شناسه‌های بیمه
	insIDSet := make(map[uint]struct{})
	for _, row := range cisRows {
		insIDSet[row.InsuranceID] = struct{}{}
	}

	insuranceMap := make(map[uint]models.Insurance)
	if len(insIDSet) > 0 {
		insIDs := make([]uint, 0, len(insIDSet))
		for id := range insIDSet {
			insIDs = append(insIDs, id)
		}
		var insRows []models.Insurance
		if err := r.DB.Where("id IN ?", insIDs).Order("name asc").Find(&insRows).Error; err != nil {
			return nil, err
		}
		for _, ins := range insRows {
			insuranceMap[ins.ID] = ins
		}
	}

	// نگاشت شناسه‌های بیمه به هر شناسه خدمت
	serviceToInsurances := make(map[uint][]models.Insurance)
	for _, row := range cisRows {
		if ins, ok := insuranceMap[row.InsuranceID]; ok {
			serviceToInsurances[row.ServiceID] = append(serviceToInsurances[row.ServiceID], ins)
		}
	}

	result := make([]DoctorServiceWithInsurances, 0, len(doctorServices))
	for _, s := range doctorServices {
		result = append(result, DoctorServiceWithInsurances{
			Service:    s,
			Insurances: serviceToInsurances[s.ID],
		})
	}
	return result, nil
}
