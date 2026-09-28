package repository

import (
	"sort"
	"strings"

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

// Delete یک خدمت و تمام انتصاب‌های وابسته به آن (به بیمه‌های مراکز، پزشکان و بسته‌ها) را در قالب تراکنش حذف می‌کند.
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
		if err := tx.Where("service_id = ?", id).Delete(&models.ServicePackageItem{}).Error; err != nil {
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

// ServicePackageWithIDs یک بسته خدمات همراه با شناسه‌های خدمات عضو آن است.
type ServicePackageWithIDs struct {
	Package    models.ServicePackage
	ServiceIDs []uint
}

// ListPackages تمام بسته‌های خدمات را به ترتیب نام برمی‌گرداند.
// ورودی: ندارد.
// خروجی: اسلایس ServicePackage یا خطای دیتابیس.
func (r *ServiceRepo) ListPackages() ([]models.ServicePackage, error) {
	var rows []models.ServicePackage
	err := r.DB.Order("name asc").Find(&rows).Error
	return rows, err
}

// GetPackageByID یک بسته خدمات را بر اساس شناسه یکتا دریافت می‌کند.
// ورودی: id شناسه بسته.
// خروجی: اشاره‌گر به مدل ServicePackage یا خطای دیتابیس.
func (r *ServiceRepo) GetPackageByID(id uint) (*models.ServicePackage, error) {
	var row models.ServicePackage
	if err := r.DB.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// CreatePackage یک بسته خدمات جدید ایجاد می‌کند.
// ورودی: row مدل بسته برای ذخیره‌سازی.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) CreatePackage(row *models.ServicePackage) error {
	return r.DB.Create(row).Error
}

// UpdatePackage اطلاعات یک بسته خدمات موجود را بروزرسانی می‌کند.
// ورودی: row مدل بسته حاوی شناسه و فیلدهای تغییریافته.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) UpdatePackage(row *models.ServicePackage) error {
	return r.DB.Save(row).Error
}

// DeletePackage یک بسته، عضویت خدمات آن و انتصابش به بخش‌ها را در قالب تراکنش حذف می‌کند.
// ورودی: id شناسه بسته.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) DeletePackage(id uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("package_id = ?", id).Delete(&models.ServicePackageItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("package_id = ?", id).Delete(&models.SectionServicePackage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.ServicePackage{}, id).Error
	})
}

// ListServiceIDsByPackageID شناسه‌های خدمات عضو یک بسته را برمی‌گرداند.
// ورودی: packageID شناسه بسته.
// خروجی: اسلایس شناسه‌های عددی خدمات یا خطای دیتابیس.
func (r *ServiceRepo) ListServiceIDsByPackageID(packageID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&models.ServicePackageItem{}).
		Where("package_id = ?", packageID).
		Pluck("service_id", &ids).Error
	return ids, err
}

// CountServicesByPackageIDs تعداد خدمات هر بسته را برای شناسه‌های داده‌شده برمی‌گرداند.
// ورودی: packageIDs لیست شناسه بسته‌ها.
// خروجی: نگاشت شناسه بسته به تعداد خدمات یا خطای دیتابیس.
func (r *ServiceRepo) CountServicesByPackageIDs(packageIDs []uint) (map[uint]int, error) {
	out := make(map[uint]int, len(packageIDs))
	if len(packageIDs) == 0 {
		return out, nil
	}
	type countRow struct {
		PackageID uint
		Cnt       int
	}
	var rows []countRow
	if err := r.DB.Model(&models.ServicePackageItem{}).
		Select("package_id, count(*) as cnt").
		Where("package_id IN ?", packageIDs).
		Group("package_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PackageID] = row.Cnt
	}
	return out, nil
}

// ReplacePackageServices خدمات عضو یک بسته را جایگزین می‌کند.
// ورودی: packageID شناسه بسته، serviceIDs لیست شناسه‌های جدید خدمات.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *ServiceRepo) ReplacePackageServices(packageID uint, serviceIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if packageID == 0 {
			return nil
		}
		if err := tx.Where("package_id = ?", packageID).Delete(&models.ServicePackageItem{}).Error; err != nil {
			return err
		}
		if len(serviceIDs) == 0 {
			return nil
		}
		rows := make([]models.ServicePackageItem, 0, len(serviceIDs))
		seen := make(map[uint]struct{}, len(serviceIDs))
		for _, id := range serviceIDs {
			if id == 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			rows = append(rows, models.ServicePackageItem{PackageID: packageID, ServiceID: id})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

// ListPackagesWithServiceIDs تمام بسته‌ها را همراه با شناسه خدمات عضو هر بسته برمی‌گرداند.
// ورودی: ندارد.
// خروجی: اسلایس ServicePackageWithIDs یا خطای دیتابیس.
func (r *ServiceRepo) ListPackagesWithServiceIDs() ([]ServicePackageWithIDs, error) {
	packages, err := r.ListPackages()
	if err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, nil
	}

	var items []models.ServicePackageItem
	if err := r.DB.Find(&items).Error; err != nil {
		return nil, err
	}
	byPackage := make(map[uint][]uint, len(packages))
	for _, item := range items {
		byPackage[item.PackageID] = append(byPackage[item.PackageID], item.ServiceID)
	}

	out := make([]ServicePackageWithIDs, 0, len(packages))
	for _, pkg := range packages {
		ids := byPackage[pkg.ID]
		if ids == nil {
			ids = []uint{}
		}
		out = append(out, ServicePackageWithIDs{
			Package:    pkg,
			ServiceIDs: ids,
		})
	}
	return out, nil
}

// SearchServices خدمات کاتالوگ را با جستجوی نام یا توضیحات و صفحه‌بندی برمی‌گرداند.
// ورودی: query عبارت جستجو (خالی یعنی بدون فیلتر)، page شماره صفحه از ۱، pageSize اندازه صفحه.
// خروجی: ردیف‌های همان صفحه، تعداد کل مطابق فیلتر، یا خطای دیتابیس.
func (r *ServiceRepo) SearchServices(query string, page, pageSize int) ([]models.Service, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 15
	}
	var total int64
	if err := r.serviceSearchQuery(query).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Service
	offset := (page - 1) * pageSize
	err := r.serviceSearchQuery(query).Order("name asc").Offset(offset).Limit(pageSize).Find(&rows).Error
	return rows, total, err
}

// serviceSearchQuery کوئری فیلترشده خدمات را از نو می‌سازد تا Count و Find روی یک زنجیره مشترک اثر نگذارند.
// ورودی: query عبارت جستجو. خروجی: کوئری GORM روی مدل Service.
func (r *ServiceRepo) serviceSearchQuery(query string) *gorm.DB {
	db := r.DB.Model(&models.Service{})
	query = strings.TrimSpace(query)
	if query == "" {
		return db
	}
	pattern := "%" + escapeSQLServerLike(query) + "%"
	return db.Where("name LIKE ? OR description LIKE ?", pattern, pattern)
}

// ListPackageNamesByServiceIDs نام بسته‌هایی را که هر خدمت در آن‌ها عضو است برمی‌گرداند.
// ورودی: serviceIDs شناسه خدمات. خروجی: نگاشت شناسه خدمت به نام بسته‌ها، مرتب‌شده بر اساس نام.
func (r *ServiceRepo) ListPackageNamesByServiceIDs(serviceIDs []uint) (map[uint][]string, error) {
	out := make(map[uint][]string, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return out, nil
	}
	type nameRow struct {
		ServiceID uint
		Name      string
	}
	var rows []nameRow
	err := r.DB.Model(&models.ServicePackageItem{}).
		Select("service_package_items.service_id as service_id, service_packages.name as name").
		Joins("JOIN service_packages ON service_packages.id = service_package_items.package_id AND service_packages.deleted_at IS NULL").
		Where("service_package_items.service_id IN ?", serviceIDs).
		Order("service_packages.name asc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	seen := make(map[uint]map[string]struct{}, len(serviceIDs))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			continue
		}
		if seen[row.ServiceID] == nil {
			seen[row.ServiceID] = map[string]struct{}{}
		}
		if _, ok := seen[row.ServiceID][name]; ok {
			continue
		}
		seen[row.ServiceID][name] = struct{}{}
		out[row.ServiceID] = append(out[row.ServiceID], name)
	}
	return out, nil
}

// ListPackageIDsByServiceID شناسه بسته‌هایی را که یک خدمت عضو آن‌هاست برمی‌گرداند.
// ورودی: serviceID شناسه خدمت. خروجی: اسلایس شناسه بسته‌ها یا خطای دیتابیس.
func (r *ServiceRepo) ListPackageIDsByServiceID(serviceID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&models.ServicePackageItem{}).
		Where("service_id = ?", serviceID).
		Pluck("package_id", &ids).Error
	return ids, err
}

// ReplaceServicePackages عضویت یک خدمت در بسته‌ها را جایگزین می‌کند.
// ورودی: serviceID شناسه خدمت، packageIDs بسته‌های جدید (خالی یعنی حذف از همه بسته‌ها).
// خروجی: خطای دیتابیس در صورت وقوع. شناسه‌های ناموجود نادیده گرفته می‌شوند.
func (r *ServiceRepo) ReplaceServicePackages(serviceID uint, packageIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if serviceID == 0 {
			return nil
		}
		if err := tx.Where("service_id = ?", serviceID).Delete(&models.ServicePackageItem{}).Error; err != nil {
			return err
		}
		live, err := existingPackageIDs(tx, packageIDs)
		if err != nil {
			return err
		}
		if len(live) == 0 {
			return nil
		}
		rows := make([]models.ServicePackageItem, 0, len(live))
		for _, id := range live {
			rows = append(rows, models.ServicePackageItem{PackageID: id, ServiceID: serviceID})
		}
		return tx.Create(&rows).Error
	})
}

// ListPackageIDsBySectionID شناسه بسته‌های منتسب به یک بخش را برمی‌گرداند.
// ورودی: sectionID شناسه بخش. خروجی: اسلایس شناسه بسته‌ها یا خطای دیتابیس.
func (r *ServiceRepo) ListPackageIDsBySectionID(sectionID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&models.SectionServicePackage{}).
		Where("section_id = ?", sectionID).
		Pluck("package_id", &ids).Error
	return ids, err
}

// ReplaceSectionPackages بسته‌های منتسب به یک بخش را جایگزین می‌کند.
// ورودی: sectionID شناسه بخش، packageIDs بسته‌های جدید (خالی یعنی حذف همه انتصاب‌ها).
// خروجی: خطای دیتابیس در صورت وقوع. شناسه‌های ناموجود نادیده گرفته می‌شوند.
func (r *ServiceRepo) ReplaceSectionPackages(sectionID uint, packageIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if sectionID == 0 {
			return nil
		}
		if err := tx.Where("section_id = ?", sectionID).Delete(&models.SectionServicePackage{}).Error; err != nil {
			return err
		}
		live, err := existingPackageIDs(tx, packageIDs)
		if err != nil {
			return err
		}
		if len(live) == 0 {
			return nil
		}
		rows := make([]models.SectionServicePackage, 0, len(live))
		for _, id := range live {
			rows = append(rows, models.SectionServicePackage{SectionID: sectionID, PackageID: id})
		}
		return tx.Create(&rows).Error
	})
}

// SectionPublicService یک خدمت قابل نمایش در صفحه عمومی بخش همراه با نام بسته‌های مبدأ است.
type SectionPublicService struct {
	Service      models.Service
	PackageNames []string
}

// ListPublicServicesBySection خدمات یکتای بسته‌های غیرخالی منتسب به بخش را برمی‌گرداند.
// ورودی: sectionID شناسه بخش.
// خروجی: خدمات مرتب‌شده بر اساس نام، یا اسلایس خالی اگر بسته‌ای منتسب نباشد یا همه بسته‌ها خالی باشند.
func (r *ServiceRepo) ListPublicServicesBySection(sectionID uint) ([]SectionPublicService, error) {
	if sectionID == 0 {
		return nil, nil
	}
	packageIDs, err := r.ListPackageIDsBySectionID(sectionID)
	if err != nil {
		return nil, err
	}
	if len(packageIDs) == 0 {
		return nil, nil
	}
	var packages []models.ServicePackage
	if err := r.DB.Where("id IN ?", packageIDs).Order("name asc").Find(&packages).Error; err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, nil
	}
	liveIDs := make([]uint, 0, len(packages))
	nameByID := make(map[uint]string, len(packages))
	for _, pkg := range packages {
		liveIDs = append(liveIDs, pkg.ID)
		nameByID[pkg.ID] = pkg.Name
	}
	var items []models.ServicePackageItem
	if err := r.DB.Where("package_id IN ?", liveIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	serviceIDs := make([]uint, 0, len(items))
	namesByService := make(map[uint][]string)
	seenName := make(map[uint]map[string]struct{})
	for _, item := range items {
		name := strings.TrimSpace(nameByID[item.PackageID])
		if name == "" {
			continue
		}
		if seenName[item.ServiceID] == nil {
			seenName[item.ServiceID] = map[string]struct{}{}
			serviceIDs = append(serviceIDs, item.ServiceID)
		}
		if _, ok := seenName[item.ServiceID][name]; ok {
			continue
		}
		seenName[item.ServiceID][name] = struct{}{}
		namesByService[item.ServiceID] = append(namesByService[item.ServiceID], name)
	}
	if len(serviceIDs) == 0 {
		return nil, nil
	}
	var services []models.Service
	if err := r.DB.Where("id IN ?", serviceIDs).Order("name asc").Find(&services).Error; err != nil {
		return nil, err
	}
	out := make([]SectionPublicService, 0, len(services))
	for _, svc := range services {
		names := namesByService[svc.ID]
		sort.Strings(names)
		out = append(out, SectionPublicService{Service: svc, PackageNames: names})
	}
	return out, nil
}

// existingPackageIDs شناسه‌های بسته موجود را بدون تکرار و با حفظ ترتیب ورودی برمی‌گرداند.
// ورودی: tx تراکنش جاری، packageIDs شناسه‌های درخواستی.
// خروجی: شناسه‌های معتبر یا خطای دیتابیس.
func existingPackageIDs(tx *gorm.DB, packageIDs []uint) ([]uint, error) {
	if len(packageIDs) == 0 {
		return nil, nil
	}
	var found []uint
	if err := tx.Model(&models.ServicePackage{}).Where("id IN ?", packageIDs).Pluck("id", &found).Error; err != nil {
		return nil, err
	}
	ok := make(map[uint]struct{}, len(found))
	for _, id := range found {
		ok[id] = struct{}{}
	}
	out := make([]uint, 0, len(found))
	seen := make(map[uint]struct{}, len(found))
	for _, id := range packageIDs {
		if _, exists := ok[id]; !exists {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// escapeSQLServerLike نویسه‌های خاص LIKE در SQL Server را بی‌اثر می‌کند.
// ورودی: s متن خام کاربر. خروجی: متن امن برای قرار گرفتن داخل الگوی LIKE.
func escapeSQLServerLike(s string) string {
	return strings.NewReplacer("[", "[[]", "%", "[%]", "_", "[_]").Replace(s)
}
