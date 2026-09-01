package booking

import (
	"errors"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"

	ptime "github.com/yaa110/go-persian-calendar"
)

// PatientLookupDTO اطلاعات بیمار برای پر کردن خودکار فرم.
type PatientLookupDTO struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Mobile     string `json:"mobile"`
	NationalID string `json:"national_id"`
	BirthDate  string `json:"birth_date"` // yyyy/MM/dd شمسی
	Sex        string `json:"sex"`
}

// LookupPatientByNationalID پروفایل ذخیره‌شده بیمار را برای autofill برمی‌گرداند.
// ورودی: کد ملی. خروجی: DTO و true اگر یافت شد.
func (s *Service) LookupPatientByNationalID(nationalID string) (*PatientLookupDTO, bool) {
	if s == nil || s.Appointments == nil {
		return nil, false
	}
	nationalID = digitsOnly(nationalID)
	if !IsValidIranianNationalID(nationalID) {
		return nil, false
	}
	p, err := s.Appointments.GetByNationalID(nationalID)
	if err != nil {
		if errors.Is(err, repository.ErrAppointmentNotFound) {
			return nil, false
		}
		return nil, false
	}
	return patientToLookupDTO(p), true
}

// patientToLookupDTO مدل Patient را به DTO فرم تبدیل می‌کند.
func patientToLookupDTO(p *models.Patient) *PatientLookupDTO {
	if p == nil {
		return nil
	}
	birth := ""
	if !p.BirthDate.IsZero() {
		birth = ptime.New(p.BirthDate).Format("yyyy/MM/dd")
	}
	sex := string(p.Sex)
	if sex == string(models.OTHER) {
		sex = ""
	}
	return &PatientLookupDTO{
		FirstName:  p.FirstName,
		LastName:   p.LastName,
		Mobile:     p.Mobile,
		NationalID: p.NationalID,
		BirthDate:  birth,
		Sex:        sex,
	}
}
