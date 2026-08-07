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
	handlers := wsclient.NewHandlers(doctorSvc, appointmentSvc, bookingSvc, conn)

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

	doctorSync := sync.NewDoctorSync(doctorSvc)
	appointmentSync := sync.NewAppointmentSync(appointmentSvc, nil)
	syncStop := make(chan struct{})
	go runPeriodicDoctorSync(doctorSync, time.Duration(cfg.DoctorSyncIntervalSec)*time.Second, syncStop, stop)
	// سینک خودکار نوبت‌ها هر ۱ دقیقه (پیش‌فرض)؛ سرور در کش ۴ ساعته نگه می‌دارد
	go runPeriodicAppointmentSync(appointmentSync, time.Duration(cfg.AppointmentSyncIntervalSec)*time.Second, syncStop, stop)
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

// runPeriodicDoctorSync pushes the doctor list on a fixed interval.
func runPeriodicDoctorSync(ds *sync.DoctorSync, interval time.Duration, sessionStop, appStop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-appStop:
			return
		case <-sessionStop:
			return
		case <-ticker.C:
			if err := ds.RunOnce(); err != nil {
				log.Printf("doctor sync: %v", err)
			}
		}
	}
}
