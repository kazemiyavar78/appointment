package repository

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/slug"
	"tebpardaz/server/internal/text"

	"gorm.io/gorm"
)

// ErrSpecialtyNameTaken یعنی نام نرمال‌شدهٔ تخصص از قبل وجود دارد.
var ErrSpecialtyNameTaken = errors.New("specialty name already exists")

// ErrSpecialtySlugUsed یعنی slug پایه و slug با پسوند ID هر دو گرفته شده‌اند.
var ErrSpecialtySlugUsed = errors.New("specialty slug is already used")

// specialtySlugLockSQL ایجاد همزمان تخصص را تا پایان تراکنش سریال می‌کند.
const specialtySlugLockSQL = `
DECLARE @rc int;
EXEC @rc = sp_getapplock
	@Resource = N'specialty-slug',
	@LockMode = N'Exclusive',
	@LockOwner = N'Transaction',
	@LockTimeout = 10000;
SELECT @rc;`

// specialtyIdentityInsertSQL ردیف برخورد را با ID ازپیش‌معلوم و slug نهایی درج می‌کند.
const specialtyIdentityInsertSQL = `
INSERT INTO specialties (
	id, created_at, updated_at, name, slug, name_en, short_description, description,
	icon, color, sort_order, show_in_booking, is_approved
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// specialtySlugOnUpdate اگر اسلاگ ذخیره‌شده باشد همان را برمی‌گرداند.
// ورودی: اسلاگ فعلی. خروجی: مقدار پایدار و true، یا false وقتی باید ساخته شود.
func specialtySlugOnUpdate(current string) (string, bool) {
	current = strings.TrimSpace(current)
	if current == "" {
		return "", false
	}
	return current, true
}

// PlanSpecialtySlugs اسلاگ پایدار ردیف‌های بدون slug را تعیین می‌کند.
// ورودی: ردیف‌ها، شامل حذف‌شده‌ها. خروجی: slug نهایی هر ID، یا خطا اگر نام قابل‌تبدیل نباشد.
// اسلاگ موجود overwrite نمی‌شود. برخورد با پسوند ID حل می‌شود.
func PlanSpecialtySlugs(rows []models.Specialty) (map[uint]string, error) {
	sorted := append([]models.Specialty(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	taken := map[string]uint{}
	for _, row := range sorted {
		current := strings.TrimSpace(row.Slug)
		if current == "" {
			continue
		}
		if owner, ok := taken[current]; ok && owner != row.ID {
			return nil, fmt.Errorf("specialty slug %q is used by id %d and id %d", current, owner, row.ID)
		}
		taken[current] = row.ID
	}
	out := make(map[uint]string, len(sorted))
	for _, row := range sorted {
		current := strings.TrimSpace(row.Slug)
		if current != "" {
			out[row.ID] = current
			continue
		}
		if row.ID == 0 {
			return nil, fmt.Errorf("specialty slug requires id")
		}
		chosen, err := allocateSpecialtySlug(row.Name, row.ID, taken)
		if err != nil {
			return nil, fmt.Errorf("specialty id %d: %w", row.ID, err)
		}
		taken[chosen] = row.ID
		out[row.ID] = chosen
	}
	return out, nil
}

// allocateSpecialtySlug اسلاگ عادی یا در برخورد اسلاگ با پسوند ID را انتخاب می‌کند.
// ورودی: نام، شناسه و اسلاگ‌های گرفته‌شده. خروجی: اسلاگ یا خطا. fallback ساختگی نمی‌سازد.
func allocateSpecialtySlug(name string, id uint, taken map[string]uint) (string, error) {
	base, err := slug.MakePersian(name)
	if err != nil {
		return "", err
	}
	if !slugUsedByOther(taken, base, id) {
		if err := validateSpecialtySlug(base); err != nil {
			return "", err
		}
		return base, nil
	}
	if id == 0 {
		return "", fmt.Errorf("specialty slug %q needs id", base)
	}
	suffixed := base + "-" + strconv.FormatUint(uint64(id), 10)
	if err := validateSpecialtySlug(suffixed); err != nil {
		return "", err
	}
	if slugUsedByOther(taken, suffixed, id) {
		return "", fmt.Errorf("specialty slug %q is already used", suffixed)
	}
	return suffixed, nil
}

// validateSpecialtySlug طول اسلاگ را با سقف مدل می‌سنجد.
// ورودی: اسلاگ. خروجی: خطا اگر خالی یا بلندتر از SpecialtySlugMaxRunes باشد.
func validateSpecialtySlug(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || value == "*" {
		return slug.ErrEmpty
	}
	if utf8.RuneCountInString(value) > models.SpecialtySlugMaxRunes {
		return fmt.Errorf("specialty slug longer than %d", models.SpecialtySlugMaxRunes)
	}
	return nil
}

func slugUsedByOther(taken map[string]uint, value string, id uint) bool {
	owner, ok := taken[value]
	return ok && owner != id
}

// BackfillSpecialtySlugs اسلاگ خالی تخصص‌های موجود را در یک تراکنش می‌نویسد.
// ورودی: اتصال appointment. خروجی: خطا. اجرای دوباره اسلاگ موجود را عوض نمی‌کند.
func BackfillSpecialtySlugs(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Specialty{}) || !db.Migrator().HasColumn(&models.Specialty{}, "slug") {
		return nil
	}
	var rows []models.Specialty
	if err := db.Unscoped().Find(&rows).Error; err != nil {
		return err
	}
	plan, err := PlanSpecialtySlugs(rows)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			if strings.TrimSpace(row.Slug) != "" {
				continue
			}
			next := plan[row.ID]
			if next == "" {
				return fmt.Errorf("specialty id %d: empty slug plan", row.ID)
			}
			if err := tx.Unscoped().Model(&models.Specialty{}).Where("id = ?", row.ID).Update("slug", next).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// decideSpecialtyCreateSlug اسلاگ نهایی را قبل از INSERT انتخاب می‌کند.
// ورودی: نام، ردیف‌های موجود (شامل حذف‌شده) و ID بعدی. خروجی: slug، ID صریح فقط برای برخورد، یا خطا.
// نام نرمال تکراری رد می‌شود. placeholder مشترک ساخته نمی‌شود.
func decideSpecialtyCreateSlug(name string, rows []models.Specialty, nextID uint) (string, uint, error) {
	base, err := slug.MakePersian(name)
	if err != nil {
		return "", 0, err
	}
	if err := validateSpecialtySlug(base); err != nil {
		return "", 0, err
	}
	norm := text.NormalizePersianText(name)
	taken := map[string]uint{}
	for _, row := range rows {
		current := strings.TrimSpace(row.Slug)
		if current != "" {
			taken[current] = row.ID
		}
		if row.DeletedAt.Valid || norm == "" {
			continue
		}
		if text.NormalizePersianText(row.Name) == norm {
			return "", 0, ErrSpecialtyNameTaken
		}
	}
	if _, ok := taken[base]; !ok {
		return base, 0, nil
	}
	if nextID == 0 {
		return "", 0, fmt.Errorf("specialty slug %q needs id", base)
	}
	suffixed := base + "-" + strconv.FormatUint(uint64(nextID), 10)
	if err := validateSpecialtySlug(suffixed); err != nil {
		return "", 0, err
	}
	if _, ok := taken[suffixed]; ok {
		return "", 0, fmt.Errorf("%w: %q", ErrSpecialtySlugUsed, suffixed)
	}
	return suffixed, nextID, nil
}

// nextSpecialtyID شناسهٔ بعدی را از بزرگ‌ترین ID موجود، شامل حذف‌شده، حساب می‌کند.
// ورودی: ردیف‌ها. خروجی: max(id)+1. جدول خالی مقدار ۱ می‌دهد.
func nextSpecialtyID(rows []models.Specialty) uint {
	var maxID uint
	for _, row := range rows {
		if row.ID > maxID {
			maxID = row.ID
		}
	}
	return maxID + 1
}

// lockSpecialtyCreates قفل تراکنشی SQL Server را برای تخصیص slug می‌گیرد.
// ورودی: تراکنش باز. خروجی: خطا اگر قفل داده نشود.
func lockSpecialtyCreates(tx *gorm.DB) error {
	var rc int
	if err := tx.Raw(specialtySlugLockSQL).Scan(&rc).Error; err != nil {
		return err
	}
	if rc < 0 {
		return fmt.Errorf("specialty slug lock failed: %d", rc)
	}
	return nil
}

// insertSpecialtyWithID ردیف را با IDENTITY_INSERT و slug نهایی می‌نویسد.
// ورودی: تراکنش و ردیف با ID و Slug پر. خروجی: خطا. در شکست OFF هم اجرا می‌شود تا اتصال آلوده نماند.
func insertSpecialtyWithID(tx *gorm.DB, row *models.Specialty) error {
	if err := tx.Exec("SET IDENTITY_INSERT specialties ON").Error; err != nil {
		return err
	}
	defer tx.Exec("SET IDENTITY_INSERT specialties OFF")
	now := time.Now()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	insertErr := tx.Exec(specialtyIdentityInsertSQL,
		row.ID, row.CreatedAt, row.UpdatedAt, row.Name, row.Slug, row.NameEN,
		row.ShortDescription, row.Description, row.Icon, row.Color, row.SortOrder,
		row.ShowInBooking, row.IsApproved,
	).Error
	offErr := tx.Exec("SET IDENTITY_INSERT specialties OFF").Error
	if insertErr != nil {
		return insertErr
	}
	return offErr
}

// GetBySlug یک تخصص را با اسلاگ ذخیره‌شده پیدا می‌کند.
// ورودی: اسلاگ دقیق. خروجی: ردیف یا ErrRecordNotFound. نام دوباره ساخته نمی‌شود.
func (r *SpecialtyRepo) GetBySlug(value string) (*models.Specialty, error) {
	value = strings.TrimSpace(value)
	if r == nil || r.DB == nil || value == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var row models.Specialty
	if err := r.DB.Where("slug = ?", value).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
