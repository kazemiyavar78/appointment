// بسته handler: خواندن رزروهای هفتگی از دیتابیس و ذخیره تنظیمات نمایش TV به‌ازای IP.
package localdb

import (
	"database/sql"
	
	"fmt"
	
	"sort"
	"strconv"
	"strings"
	"time"

	ptime "github.com/yaa110/go-persian-calendar"
)

// ReserveModel یک ردیف نوبت/رزرو برای نمایش در جدول است.
type ReserveModel struct {
	DoctorFullName  string // نام کامل پزشک
	ReserveCount    int    // تعداد رزروها
	ReserveMaxCount int    // تعداد حداکثر رزروها
	ReserveDate     string // تاریخ رزرو (شمسی از دیتابیس)
	ReserveTime     string // ساعت رزرو
	Speciality      string // عنوان تخصص
	ShiftName       string // برچسب شیفت ترکیبی (مثلاً «شنبه صبح») برای تطبیق با ردیف جدول
	VisitTime       int    // مدت زمان هر نوبت
	OutTime         string // زمان خروج پزشک

}

const (
	MorningShiftFrom   = "00:00" // شروع بازه صبح (شامل)
	MorningShiftTo     = "12:00" // پایان بازه صبح (غیرشامل؛ ۱۲:۰۰ متعلق به عصر است)
	AfternoonShiftFrom = "12:00" // شروع بازه عصر (شامل)
	AfternoonShiftTo   = "23:59" // پایان بازه عصر (شامل)
)

// GetReserve از دیتابیس رزروهای هفته را می‌خواند، شیفت را از روی ساعت تشخیص می‌دهد و لیست مرتب‌شده برمی‌گرداند.
// ورودی: اتصال sql.DB. خروجی: اسلایس رزروها، نقشه نام شیفت‌ها (در حال حاضر برای سازگاری API)، خطا.
func (s *Store) GetReserve() ([]ReserveModel, map[string][]string, error) {
	var weeklyDate = returnWeeklyDate()
	var shiftsNames = make(map[string][]string)

	fmt.Println("weeklyDate", weeklyDate)

	rows, err := s.DB.Query(`
			SELECT
				CONCAT(ps.user_name , ' ' , ps.user_famil) as doctor_full_name,
				ts.sharh as speciality,
				res_dt,
				res_tm ,
				rv.rnobat as reserve_max_count,
				(SELECT COUNT(*) FROM reserve WHERE cpez = rv.cpez AND res_dt = rv.res_dt  AND pvaz <>1) as reserve_count,
				(select ISNULL(visit_tm ,  0) from personel where user_code=rv.cpez) as visit_tm
			FROM 
				reserve rv
			INNER JOIN personel ps ON  ps.user_code = rv.cpez
			INNER JOIN takhasos ts ON ts.tcode = ps.takhasos
			WHERE
				rv.pvaz=1 AND rv.res_dt BETWEEN @startDate AND @endDate 
			GROUP by cpez,res_dt,res_tm ,ts.sharh ,ps.user_name ,ps.user_famil, visit_tm ,rnobat
			ORDER BY res_dt,res_tm
	`, sql.Named("startDate", weeklyDate[0]), sql.Named("endDate", weeklyDate[1]))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get reserve: %v", err)
	}
	defer rows.Close()

	var reserveList = []ReserveModel{}
	for rows.Next() {
		var reserve ReserveModel
		err := rows.Scan(
			&reserve.DoctorFullName,
			&reserve.Speciality,
			&reserve.ReserveDate,
			&reserve.ReserveTime,
			&reserve.ReserveMaxCount,
			&reserve.ReserveCount,
			&reserve.VisitTime,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan reserve: %v", err)
		}
		reserve.Speciality = strings.TrimSpace(reserve.Speciality)

		// نام روز هفته برای ساخت ShiftName مثل «یکشنبه صبح»
		NameDay := returnDayName(reserve.ReserveDate)

		// زمان خروج پزشک
		visitTime, err := time.Parse("15:04", reserve.ReserveTime)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse reserve time: %v", err)
		}
		if reserve.VisitTime == 0 {
			reserve.VisitTime = 5
		}
		outtimeTemp := visitTime.Add(time.Duration(reserve.VisitTime) * time.Minute)

		if reserve.ReserveDate == weeklyDate[0] {
			if outtimeTemp.Before(time.Now()) {
				continue
			}
		}
		
		reserve.OutTime = outtimeTemp.Format("15:04")
		


		// کلید شیفت = تاریخ + نام روز + صبح/عصر تا دو تاریخ هم‌نام هفته ادغام نشوند
		shiftKey := ""
		if reserve.ReserveTime >= MorningShiftFrom && reserve.ReserveTime < MorningShiftTo {
			reserve.ShiftName = NameDay + " صبح"
			shiftKey = reserve.ReserveDate + "|" + reserve.ShiftName
		} else if reserve.ReserveTime >= AfternoonShiftFrom && reserve.ReserveTime <= AfternoonShiftTo {
			reserve.ShiftName = NameDay + " عصر"
			shiftKey = reserve.ReserveDate + "|" + reserve.ShiftName
		}
		if shiftKey != "" {
			if !returnShiftsNames(shiftsNames[shiftKey], shiftKey) {
				// نگه‌داشتن نمونه تاریخ برای آن برچسب شیفت (استفاده قدیمی/سازگاری)
				shiftsNames[shiftKey] = []string{reserve.ShiftName, reserve.ReserveDate}
			}
		}

		reserveList = append(reserveList, reserve)
	}

	// مرتب‌سازی بر اساس نام تخصص برای ثبات نمایش ستون‌ها
	sort.Slice(reserveList, func(i, j int) bool {
		return reserveList[i].Speciality < reserveList[j].Speciality
	})


	return reserveList, shiftsNames, nil
}

// returnWeeklyDate بازه تاریخ شمسی فیلتر SQL را برمی‌گرداند [شروع، پایان].
func returnWeeklyDate() []string {
	var Now = ptime.Now()
	// var FirstWeekDay = Now.FirstWeekDay().Format("yyyy/MM/dd")
	var FirstWeekDay = Now.Format("yyyy/MM/dd")
	// FirstWeekDay = "1402/01/01"
	// var LastWeekDay = Now.LastWeekday().Format("yyyy/MM/dd")
	// امروز تا ۶ روز بعد = ۷ روز (بدون تکرار همان روز هفته در دو تاریخ)
	var LastWeekDay = Now.Add(6 * 24 * time.Hour).Format("yyyy/MM/dd")
	//  LastWeekDay = "1402/01/07"

	var date = []string{FirstWeekDay, LastWeekDay}
	return date
}

// returnDayName تاریخ شمسی yyyy/mm/dd را به نام فارسی weekday تبدیل می‌کند.
func returnDayName(dateShamsi string) string {
	var date = strings.Split(dateShamsi, "/")
	var year = date[0]
	var month = date[1]
	var day = date[2]

	yearInt, _ := strconv.Atoi(year)
	monthInt, _ := strconv.Atoi(month)
	dayInt, _ := strconv.Atoi(day)

	var pt ptime.Time = ptime.Date(yearInt, ptime.Month(monthInt), dayInt, 0, 0, 0, 0, ptime.Iran())

	return pt.Weekday().String()
}

// returnShiftsNames اگر shiftNameOut از قبل در آرایه باشد true برمی‌گرداند (جلوگیری از تکرار در map).
func returnShiftsNames(shiftsNames []string, shiftNameOut string) bool {
	for _, shiftName := range shiftsNames {
		if shiftName == shiftNameOut {
			return true
		}
	}

	return false
}
