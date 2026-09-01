package localdb

import (
	"database/sql"

	pt "github.com/yaa110/go-persian-calendar"
)

// PezList گروه بیماران منتظر یک پزشک را نگه می‌دارد.
type PezList struct {
	DoctorName  string    `json:"doctor_name"`
	DoctorCode  int       `json:"doctor_code"`
	ListPatient []Patient `json:"list_patient"`
}

// Patient یک بیمار در صف انتظار پذیرش است.
type Patient struct {
	PatientName       string `json:"patient_name"`
	PatientCode       int    `json:"patient_code"`
	PatientCmeli      string `json:"patient_cmeli"`
	VisitTime         string `json:"visit_time"`
	PazUser           int    `json:"paz_user"`
	DoctorName        string `json:"doctor_name"`
	ReceptionUserName string `json:"reception_user_name"`
}

// GetWaitePatientPezeshkListSite لیست بیماران در حال انتظار ویزیت پزشکان سایت را از HIS می‌خواند.
// ورودی: ندارد (تاریخ امروز شمسی از ساعت سیستم).
// خروجی: گروه‌های پزشک با بیماران منتظر، یا خطای دیتابیس.
func (s *Store) GetWaitePatientPezeshkListSite() ([]PezList, error) {
	now := pt.Now().Format("yy/MM/dd")
	query := `
	SELECT 
	    CONCAT((CASE paziresh.jens WHEN 1 THEN 'آقای' ELSE 'خانم' END), ' ', pname, ' ', pfamil) AS patient,
	    paziresh.cpatient,
	    paziresh.cmelli,
	    paz_tm,
	    paz_user,
	    cpez,
	    CONCAT('دکتر', ' ', p1.user_name, ' ', p1.user_famil),
	    CONCAT(p2.user_name, ' ', p2.user_famil) AS reception_user
	FROM paziresh
	INNER JOIN personel p1 ON p1.user_code = cpez
	INNER JOIN personel p2 ON p2.user_code = paz_user
	WHERE paz_dt = @today
	  AND cpez IN (
	      SELECT user_code FROM personel
	      WHERE xpezsite = 1
	  )
	  AND visit = 0
	ORDER BY cpez, paz_tm, cpatient`

	rows, err := s.DB.Query(query, sql.Named("today", now))
	if err != nil {
		return []PezList{}, err
	}
	defer rows.Close()

	uniqePezCode := make(map[int]bool)
	pezLists := make([]PezList, 0)

	for rows.Next() {
		var patient Patient
		var cpez int
		err := rows.Scan(
			&patient.PatientName,
			&patient.PatientCode,
			&patient.PatientCmeli,
			&patient.VisitTime,
			&patient.PazUser,
			&cpez,
			&patient.DoctorName,
			&patient.ReceptionUserName,
		)
		if err != nil {
			return []PezList{}, err
		}

		if !uniqePezCode[cpez] {
			pezLists = append(pezLists, PezList{
				DoctorName:  patient.DoctorName,
				DoctorCode:  cpez,
				ListPatient: []Patient{patient},
			})
			uniqePezCode[cpez] = true
			continue
		}

		for i := range pezLists {
			if pezLists[i].DoctorCode == cpez {
				pezLists[i].ListPatient = append(pezLists[i].ListPatient, patient)
				break
			}
		}
	}

	return pezLists, rows.Err()
}
