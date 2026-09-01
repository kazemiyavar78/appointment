package localdb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb" // SQL Server driver for local clinic DB
	ptime "github.com/yaa110/go-persian-calendar"

	"tebpardaz/shared/protocol"
)

// Store executes raw SQL against the clinic's local database.
// Driver: github.com/microsoft/go-mssqldb (SQL Server).
// SQL bodies stay empty until each clinic HIS schema is wired.
type Store struct {
	DB *sql.DB
}

// LocalDoctor is a doctor row read from the clinic HIS (before protocol mapping).
type LocalDoctor struct {
	Code       int
	FirstName  string
	LastName   string
	NationalID string
	Mobile     string
	ExternalID string
	// شماره نظام دکتر
	DoctorSystemID int
	Name           string
	SpecialtyName  string
	SpecialtyCode  string
	PhotoURL       string
	IsActive       bool
}

// LocalAppointment is a slot/appointment row from the clinic HIS.
type LocalAppointment struct {
	ExternalSlotID   string
	DoctorExternalID string
	StartsAt         time.Time
	EndsAt           time.Time
	Capacity         int
	BookedCount      int
	IsAvailable      bool
}

// Open connects to the local SQL Server database using database/sql (no ORM).
// Inputs: dsn (SQL Server connection string for go-mssqldb).
// Output: Store pointer or error from open/ping.
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}

// ListDoctors returns HIS doctors for sync (approved and pending) so the server can
// update by national_id + clinic_id and cache unapproved rows.
// Inputs: none (uses clinic DB).
// Output: slice of LocalDoctor or error.
func (s *Store) ListDoctors() ([]LocalDoctor, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("local db not connected")
	}
	rows, err := s.DB.Query(`SELECT
	user_code ,
	user_onvan + ' ' + user_name,
	user_famil,
	cmelli,
	user_mob,
	cnezam,
	external_id
	FROM personel
	WHERE cnezam <> '' AND LEN(cmelli) = 10
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	doctors := make([]LocalDoctor, 0)
	for rows.Next() {
		var doctor LocalDoctor
		var externalID sql.NullString
		var systemID sql.NullString

		err = rows.Scan(
			&doctor.Code,
			&doctor.FirstName,
			&doctor.LastName,
			&doctor.NationalID,
			&doctor.Mobile,
			&systemID,
			&externalID)
		if err != nil {
			return nil, err
		}
		if systemID.Valid {
			if n, convErr := strconv.Atoi(strings.TrimSpace(systemID.String)); convErr == nil {
				doctor.DoctorSystemID = n
			}
		}
		if externalID.Valid {
			doctor.ExternalID = externalID.String
		}
		doctor.Name = strings.TrimSpace(doctor.FirstName + " " + doctor.LastName)
		doctor.IsActive = true
		doctors = append(doctors, doctor)
	}
	return doctors, nil
}

// ListAppointmentsByDoctor returns appointments/slots for one local doctor code.
// Inputs: doctorExternalID (HIS doctor code).
// Output: slice of LocalAppointment or error. SQL is intentionally empty for now.
func (s *Store) ListAppointmentsByDoctor(userCode int) ([]LocalAppointment, error) {
	// TODO: SELECT appointments for one doctor — no SQL yet
	Today := ptime.Now().Format("yyyy/MM/dd")
	DaysLater := ptime.Now().Add(15 * 24 * time.Hour).Format("yyyy/MM/dd")

	rows, err := s.DB.Query(`
	SELECT 
		res_dt ,
		res_tm ,
		(SELECT COUNT(*) FROM reserve WHERE res_dt = rv.res_dt AND res_tm = rv.res_tm AND cpez = @p1 AND pvaz <> 1) as BookedCount,
		rnobat AS FULLCAPACITY,
	    (SELECT ISNULL(visit_tm,5) FROM personel WHERE user_code = @p2) as VisitTime
	FROM reserve rv
	WHERE res_dt BETWEEN @p3 AND @p4 AND cpez = @p5 AND pvaz = 1`,
		sql.Named("p1", userCode),
		sql.Named("p2", userCode),
		sql.Named("p3", Today),
		sql.Named("p4", DaysLater),
		sql.Named("p5", userCode),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	appointments := make([]LocalAppointment, 0)
	for rows.Next() {
		var appointment LocalAppointment
		var FullCapacity, BookCount int
		var StartDate, StartTime string
		var VisitTime int
		err = rows.Scan(&StartDate, &StartTime, &BookCount, &FullCapacity, &VisitTime)
		if err != nil {
			return nil, err
		}
		appointment.StartsAt = toTime(StartDate, StartTime)
		VisitTime = VisitTime * (FullCapacity - (FullCapacity - BookCount))
		appointment.EndsAt = toVisitTime(VisitTime, appointment.StartsAt)
		appointment.Capacity = FullCapacity
		appointment.BookedCount = FullCapacity - BookCount
		appointment.IsAvailable =  FullCapacity >= appointment.BookedCount 

		

		appointments = append(appointments, appointment)
	}
	return appointments, nil
}

func toVisitTime(visitTime int, startsAt time.Time) time.Time {
	return startsAt.Add(time.Duration(visitTime) * time.Minute)
}

// convertr date with format '1405/01/01' plus '00:00' to time.Time
func toTime(dateString, timeString string) time.Time {
	//split dateString to int
	DateParts := strings.Split(dateString, "/")
	yearInt, _ := strconv.Atoi(DateParts[0])
	monthInt, _ := strconv.Atoi(DateParts[1])
	dayInt, _ := strconv.Atoi(DateParts[2])

	//split timeString to int
	TimeParts := strings.Split(timeString, ":")
	hourInt, _ := strconv.Atoi(TimeParts[0])
	minuteInt, _ := strconv.Atoi(TimeParts[1])

	Date := ptime.Date(yearInt, ptime.Month(monthInt), dayInt, hourInt, minuteInt, 0, 0, time.Local)
	return Date.Time()
}

// find doctor by external id
func (s *Store) FindDoctorByExternalID(externalID string) (int, error) {
	fmt.Println("externalID", externalID)
	rows, err := s.DB.Query(`SELECT user_code FROM personel WHERE external_id = @p1`, externalID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if rows.Next() {
		var doctorCode int
		err = rows.Scan(&doctorCode)
		return doctorCode, nil
	}
	return 0, nil
}

// update doctor external id
func (s *Store) UpdateDoctorExternalID(doctorCode int, externalID string) error {
	_, err := s.DB.Exec(`UPDATE personel SET external_id = @p1 WHERE user_code = @p2`, externalID, doctorCode)
	if err != nil {
		return err
	}
	return nil
}

// SetDoctorExternalIDIfEmpty writes ExternalID only when the HIS row still has a null/empty value.
// Inputs: doctorCode (personel.user_code), externalID (server-generated id).
// Output: DB error, if any; nil when already set or updated.
func (s *Store) SetDoctorExternalIDIfEmpty(doctorCode int, externalID string) error {
	_, err := s.DB.Exec(`
		UPDATE personel
		SET external_id = @p1
		WHERE user_code = @p2
		  AND (external_id IS NULL OR LTRIM(RTRIM(external_id)) = '')
	`, externalID, doctorCode)
	return err
}

// ListAppointmentsForDoctors returns appointments/slots for all given doctor codes.
// Used by the periodic (~20 min) full refresh of doctors already sent to the site.
// Inputs: doctorExternalIDs (HIS codes previously pushed to the server).
// Output: slice of LocalAppointment or error. SQL is intentionally empty for now.
func (s *Store) ListAppointmentsForDoctors() ([]LocalAppointment, error) {
	rows, err := s.DB.Query(`SELECT user_code , external_id FROM personel WHERE external_id is not null`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	appointments := make([]LocalAppointment, 0)
	for rows.Next() {
		var doctorCode int
		var externalID string
		if err := rows.Scan(&doctorCode, &externalID); err != nil {
			return nil, err
		}
		docAppts, err := s.ListAppointmentsByDoctor(doctorCode)
		if err != nil {
			return nil, err
		}
		for i := range docAppts {
			docAppts[i].DoctorExternalID = externalID
		}
		appointments = append(appointments, docAppts...)
	}
	return appointments, nil
}

// SaveBooking inserts a website booking into the local HIS after capacity and duplicate checks.
// Inputs: booking payload from protocol.BookingCreate.
// Output: local external appointment id (cbakhsh-based key) and error.
func (s *Store) SaveBooking(booking *protocol.BookingCreate) (externalID string, err error) {
	if booking == nil {
		return "", fmt.Errorf("empty booking")
	}
	doctorCode, err := s.FindDoctorByExternalID(booking.DoctorExternalID)
	if err != nil {
		return "", err
	}
	if doctorCode == 0 {
		return "", fmt.Errorf("doctor not found")
	}

	StartDateTime := ptime.New(booking.StartDateTime).Format("yyyy/MM/dd HH:mm")
	StartDateTimeParts := strings.Split(StartDateTime, " ")
	if len(StartDateTimeParts) != 2 {
		return "", fmt.Errorf("invalid start datetime")
	}
	StartDateTimeDate := StartDateTimeParts[0]
	StartDateTimeTime := StartDateTimeParts[1]


	// Duplicate national ID: any non-template reserve row blocks another website booking.
	exists, err := s.HasActiveBookingByNationalID(booking.NationalID, StartDateTimeDate, StartDateTimeTime)
	if err != nil {
		return "", err
	}
	if exists {
		return "", &protocol.ProtocolError{
			Code:    protocol.ErrCodeAlreadyBooked,
			Message: "با این کد ملی قبلاً نوبت ثبت شده است",
		}
	}

	
	var cbakhsh, capacity, booked int
	err = s.DB.QueryRow(`
		SELECT TOP 1
			cbakhsh,
			rnobat,
			(SELECT COUNT(*) FROM reserve WHERE res_dt = rv.res_dt AND res_tm = rv.res_tm AND cpez = @doctor_code AND pvaz <> 1)
		FROM reserve rv
		WHERE cpez = @doctor_code AND res_dt = @res_dt AND res_tm = @res_tm AND pvaz = 1`,
		sql.Named("doctor_code", doctorCode),
		sql.Named("res_dt", StartDateTimeDate),
		sql.Named("res_tm", StartDateTimeTime),
	).Scan(&cbakhsh, &capacity, &booked)
	if err != nil {
		return "", err
	}
	if capacity > 0 && booked >= capacity {
		return "", &protocol.ProtocolError{
			Code:    protocol.ErrCodeNoCapacity,
			Message: "ظرفیت این نوبت تکمیل شده است",
		}
	}

	_, err = s.DB.Exec(`
		INSERT INTO reserve
			(cbakhsh, cpez, pjens, pname, pfamil, ptell, rnobat, res_dt, res_tm, res_user, noer, cmelli, pvaz, rcomment)
		VALUES
			(
				@cbakhsh,
				@doctor_code,
				@sex,
				@first_name,
				@last_name,
				@mobile,
				(SELECT COUNT(*) FROM reserve WHERE res_dt = @res_dt AND res_tm = @res_tm AND cpez = @doctor_code AND pvaz <> 1),
				@res_dt,
				@res_tm,
				@res_user,
				0,
				@national_id,
				@pvaz,
				@rcomment
			)`,
		sql.Named("cbakhsh", cbakhsh),
		sql.Named("doctor_code", doctorCode),
		sql.Named("sex", booking.Sex),
		sql.Named("first_name", booking.FirstName),
		sql.Named("last_name", booking.LastName),
		sql.Named("mobile", booking.Mobile),
		sql.Named("res_dt", StartDateTimeDate),
		sql.Named("res_tm", StartDateTimeTime),
		sql.Named("res_user", 0),
		sql.Named("national_id", booking.NationalID),
		sql.Named("pvaz", 2),
		sql.Named("rcomment", ""),
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s:%s:%s", doctorCode, StartDateTimeDate, StartDateTimeTime, booking.NationalID), nil
}

// HasActiveBookingByNationalID reports whether the national ID already has a non-cancelled HIS booking.
// Inputs: Iranian national ID (cmelli).
// Output: true when at least one reserve row with pvaz <> 1 exists.
func (s *Store) HasActiveBookingByNationalID(nationalID string , res_dt string , res_tm string) (bool, error) {
	nationalID = strings.TrimSpace(nationalID)
	if nationalID == "" {
		return false, nil
	}
	var count int
	err := s.DB.QueryRow(`
		SELECT COUNT(*) FROM reserve WHERE cmelli = @nid AND pvaz <> 1 AND res_dt = @res_dt AND res_tm = @res_tm`,
		sql.Named("nid", nationalID),
		sql.Named("res_dt", res_dt),
		sql.Named("res_tm", res_tm),
	).Scan(&count)
	fmt.Println("count", count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
// CancelBooking cancels a previously saved website booking in the local HIS.
// Inputs: cancel payload from protocol.BookingCancel.
// Output: error. SQL is intentionally empty for now.
func (s *Store) CancelBooking(_ *protocol.BookingCancel) error {
	// TODO: UPDATE/DELETE local booking — no SQL yet
	return nil
}

// Close closes the underlying sql.DB.
// Inputs: none (receiver).
// Output: error from sql.DB.Close, or nil if DB was never opened.
func (s *Store) Close() error {
	if s.DB == nil {
		return nil
	}
	return s.DB.Close()
}
