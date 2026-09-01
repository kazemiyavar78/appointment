package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tebpardaz/client/internal/config"
	"tebpardaz/client/internal/localdb"
	"tebpardaz/client/internal/services"
	"tebpardaz/client/internal/sync"
	"tebpardaz/client/internal/wsclient"

	"github.com/google/uuid"
)

const reconnectDelay = 5 * time.Second

func main() {
	configPath := flag.String("config", "configs/clinic.yaml", "path to clinic YAML config")
	flag.Parse()

	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	store, err := localdb.Open(cfg.LocalDBDSN)
	if err != nil {
		log.Fatalf("local db: %v", err)
	}
	defer store.Close()

	//create calmn external_id column in localdb personel table if not exist
	store.DB.Exec(`
	IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = 'personel' AND COLUMN_NAME = 'external_id')
BEGIN
	ALTER TABLE personel ADD external_id NVARCHAR(255)
END
	`)


	
	log.Printf("local db connected (%s)", cfg.ClinicName)

	stop := make(chan struct{})
	go runDoctorSyncLoop(cfg, store, stop)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down...")
	close(stop)
}

// runDoctorSyncLoop keeps the WebSocket up and handles doctor list push/ack cycles.
func runDoctorSyncLoop(cfg *config.Config, store *localdb.Store, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}

		if err := runSession(cfg, store, stop); err != nil {
			log.Printf("session ended: %v", err)
		}

		select {
		case <-stop:
			return
		case <-time.After(reconnectDelay):
		}
	}
}

// runSession dials the server, pushes doctors once, then serves inbound requests until disconnect.
func runSession(cfg *config.Config, store *localdb.Store, stop <-chan struct{}) error {
	conn, err := wsclient.Dial(cfg.ServerWSURL, cfg.AuthToken)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("websocket connected to %s", cfg.ServerWSURL)

	doctorSvc := services.NewDoctorService(store, conn)
	// سرویس نوبت‌ها: سینک خودکار + پاسخ به appointment.list.request (یک/همه پزشکان)
	appointmentSvc := services.NewAppointmentService(store, conn)
	bookingSvc := services.NewBookingService(store, conn)
	testResultSvc := services.NewTestResultService(cfg.PDFDir, conn)
	// سرویس لیست نوبت هفتگی پزشکان (بروزرسانی هر ۱۵ دقیقه)
	weeklyReserveSvc := services.NewWeeklyReserveService(store, conn)
	// سرویس صف انتظار بیماران (بروزرسانی هر ۱۰ دقیقه؛ در صورت تماشای بیماران سرور هر ۵ ثانیه درخواست می‌دهد)
	monitoringSvc := services.NewMonitoringService(store, conn)
	handlers := wsclient.NewHandlers(doctorSvc, appointmentSvc, bookingSvc, testResultSvc, weeklyReserveSvc, monitoringSvc, conn)

	heartbeat := wsclient.NewHeartbeat(conn, time.Duration(cfg.HeartbeatIntervalSec)*time.Second)
	heartbeat.Start()
	defer heartbeat.Stop()

	// پوش اولیه پزشکان ثبت‌نشده پس از اتصال
	if err := doctorSvc.PushDoctorList(uuid.NewString()); err != nil {
		log.Printf("initial doctor push: %v", err)
	}
	// پوش اولیه نوبت‌ها تا کش سرور بلافاصله پر شود (سپس هر ۱ دقیقه تکرار می‌شود)
	if err := appointmentSvc.PushAppointmentsForAllDoctors(uuid.NewString(), true); err != nil {
		log.Printf("initial appointment push: %v", err)
	}
	// پوش اولیه لیست نوبت هفتگی تا صفحه عمومی بلافاصله داده داشته باشد
	if err := weeklyReserveSvc.PushWeeklyReserves(uuid.NewString()); err != nil {
		log.Printf("initial weekly reserve push: %v", err)
	}
	// پوش اولیه صف انتظار
	if err := monitoringSvc.PushWaitingQueue(uuid.NewString()); err != nil {
		log.Printf("initial monitoring push: %v", err)
	}

	doctorSync := sync.NewDoctorSync(doctorSvc)
	appointmentSync := sync.NewAppointmentSync(appointmentSvc, nil)
	weeklyReserveSync := sync.NewWeeklyReserveSync(weeklyReserveSvc)
	monitoringSync := sync.NewMonitoringSync(monitoringSvc)
	syncStop := make(chan struct{})
	// سینک خودکار پزشکان ۳ بار در روز (مثلاً ۰۸:۰۰، ۱۴:۰۰، ۲۰:۰۰)
	go runScheduledDoctorSync(doctorSync, cfg.ParsedDoctorSyncTimes(), syncStop, stop)
	// سینک خودکار نوبت‌ها هر ۱ دقیقه (پیش‌فرض)؛ سرور در کش ۴ ساعته نگه می‌دارد
	go runPeriodicAppointmentSync(appointmentSync, time.Duration(cfg.AppointmentSyncIntervalSec)*time.Second, syncStop, stop)
	// سینک خودکار لیست نوبت هفتگی هر ۱۵ دقیقه (پیش‌فرض)
	go runPeriodicWeeklyReserveSync(weeklyReserveSync, time.Duration(cfg.WeeklyReserveSyncIntervalSec)*time.Second, syncStop, stop)
	// سینک خودکار صف انتظار هر ۱۰ دقیقه (پیش‌فرض)
	go runPeriodicMonitoringSync(monitoringSync, time.Duration(cfg.MonitoringSyncIntervalSec)*time.Second, syncStop, stop)
	defer close(syncStop)

	readDone := make(chan struct{})
	go func() {
		conn.ReadLoop(handlers.HandleMessage)
		close(readDone)
	}()

	select {
	case <-stop:
		return nil
	case <-readDone:
		return nil
	}
}

// runPeriodicAppointmentSync نوبت همه پزشکان را در فاصله ثابت (پیش‌فرض ۱ دقیقه) پوش می‌کند.
func runPeriodicAppointmentSync(as *sync.AppointmentSync, interval time.Duration, sessionStop, appStop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-appStop:
			return
		case <-sessionStop:
			return
		case <-ticker.C:
			if err := as.RunOnce(); err != nil {
				log.Printf("appointment sync: %v", err)
			}
		}
	}
}

// runPeriodicMonitoringSync صف انتظار را در فاصله ثابت (پیش‌فرض ۱۰ دقیقه) پوش می‌کند.
// ورودی: sync runner، فاصله زمانی، کانال‌های توقف نشست و برنامه.
func runPeriodicMonitoringSync(ms *sync.MonitoringSync, interval time.Duration, sessionStop, appStop <-chan struct{}) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-appStop:
			return
		case <-sessionStop:
			return
		case <-ticker.C:
			if err := ms.RunOnce(); err != nil {
				log.Printf("monitoring sync: %v", err)
			}
		}
	}
}

// runPeriodicWeeklyReserveSync لیست نوبت هفتگی را در فاصله ثابت (پیش‌فرض ۱۵ دقیقه) پوش می‌کند.
// ورودی: sync runner، فاصله زمانی، کانال‌های توقف نشست و برنامه.
func runPeriodicWeeklyReserveSync(ws *sync.WeeklyReserveSync, interval time.Duration, sessionStop, appStop <-chan struct{}) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-appStop:
			return
		case <-sessionStop:
			return
		case <-ticker.C:
			if err := ws.RunOnce(); err != nil {
				log.Printf("weekly reserve sync: %v", err)
			}
		}
	}
}

// runScheduledDoctorSync pushes the doctor list at fixed local clock times (typically 3×/day).
// Inputs: doctor sync runner, daily clock times, session/app stop channels.
// Output: none (blocks until stop).
func runScheduledDoctorSync(ds *sync.DoctorSync, times []time.Time, sessionStop, appStop <-chan struct{}) {
	if len(times) == 0 {
		return
	}
	for {
		wait := durationUntilNext(time.Now(), times)
		timer := time.NewTimer(wait)
		select {
		case <-appStop:
			timer.Stop()
			return
		case <-sessionStop:
			timer.Stop()
			return
		case <-timer.C:
			if err := ds.RunOnce(); err != nil {
				log.Printf("doctor sync: %v", err)
			}
		}
	}
}

// durationUntilNext returns the wait until the next scheduled HH:MM (local time).
// Inputs: now, clock times with hour/minute set.
// Output: positive duration until the soonest future slot.
func durationUntilNext(now time.Time, times []time.Time) time.Duration {
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	var soonest time.Time
	for _, t := range times {
		cand := today.Add(time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute)
		if !cand.After(now) {
			cand = cand.Add(24 * time.Hour)
		}
		if soonest.IsZero() || cand.Before(soonest) {
			soonest = cand
		}
	}
	d := soonest.Sub(now)
	if d < time.Second {
		return time.Second
	}
	return d
}
