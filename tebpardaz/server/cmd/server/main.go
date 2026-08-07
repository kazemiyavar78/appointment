package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/config"
	"tebpardaz/server/internal/csrf"
	"tebpardaz/server/internal/db"
	adminhandlers "tebpardaz/server/internal/handlers/admin"
	"tebpardaz/server/internal/handlers/public"
	"tebpardaz/server/internal/news"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/internal/websocket"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

const (
	DefaultExpiration = 10 * time.Minute
	CleanupInterval   = 1 * time.Hour
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	conns, err := db.Open(cfg.ManagementDSN, cfg.AppointmentDSN)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer conns.Close()

	if err := conns.MigrateAppointment(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrate: appointment_tapesh models OK (Clinic/Org/City skipped — tp_managment)")

	userRepo := repository.NewUserRepo(conns.Appointment)
	if err := userRepo.EnsureSuperAdmin(cfg.SuperAdminUser, cfg.SuperAdminPass); err != nil {
		log.Fatalf("seed superadmin: %v", err)
	}

	doctorRepo := repository.NewDoctorRepo(conns.Appointment)
	specialtyRepo := repository.NewSpecialtyRepo(conns.Appointment)
	clinicRepo := repository.NewClinicRepo(conns.Management)
	clinicScope := auth.NewClinicScope(clinicRepo)
	organizationRepo := repository.NewOrganizationRepo(conns.Management)
	newsRepo := repository.NewNewsRepo(conns.Appointment)
	newsSvc := news.NewService(newsRepo, clinicRepo)

	hub := websocket.NewHub()
	go hub.Run()

	// کش سراسری در حافظه (go-cache)؛ Init با sync.Once امن است
	memCache := cache.Init(cache.DefaultExpiration, cache.CleanupInterval)
	// کش اسلات‌های رزرو پزشکان — بدون دیتابیس، انقضای ۴ ساعته؛ کلاینت هر ۱ دقیقه بروزرسانی می‌کند
	slotCache := cache.NewSlotCache(memCache)

	adminAuth := auth.NewAdminAuth(userRepo, cfg.SessionSecret)
	approvalHandler := adminhandlers.NewApprovalHandler(doctorRepo, specialtyRepo, clinicRepo, clinicScope, hub, memCache, cache.DefaultExpiration, cache.CleanupInterval)
	appointmentHandler := adminhandlers.NewAppointmentHandler(doctorRepo, clinicRepo, slotCache, clinicScope, hub)
	specialtyHandler := adminhandlers.NewSpecialtyHandler(specialtyRepo)
	newsAdminHandler := adminhandlers.NewNewsAdminHandler(newsSvc, clinicScope)
	wsHandlers := websocket.NewHandlers(hub, clinicRepo, doctorRepo, slotCache)

	r := gin.Default()
	//where am i 
	aa , _ := os.Getwd()
	fmt.Println("where am i", aa)
	r.Static("/static", "./static")

	r.GET("/admin/login", adminAuth.LoginPage)
	r.POST("/admin/login", adminAuth.Login)
	r.GET("/admin/logout", adminAuth.Logout)

	admin := r.Group("/admin", adminAuth.Middleware())
	{
		admin.GET("/approvals",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			approvalHandler.ListPending,
		)
		admin.POST("/approvals/:id/decide",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			approvalHandler.Decide,
		)
		admin.GET("/appointments",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			appointmentHandler.List,
		)
		admin.GET("/specialties",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			specialtyHandler.List,
		)
		admin.POST("/specialties",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			specialtyHandler.Create,
		)
		admin.POST("/specialties/:id/update",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			specialtyHandler.Update,
		)
		admin.POST("/specialties/:id/delete",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			specialtyHandler.Delete,
		)

		newsRoles := []constants.UserRole{
			constants.UserRoleSuperAdmin,
			constants.UserRoleAdmin,
			constants.UserRoleClinicAdmin,
			constants.UserRoleOrganAdmin,
			constants.UserRoleEditor,
		}
		admin.GET("/news", adminAuth.RequireRoles(newsRoles...), newsAdminHandler.List)
		admin.GET("/news/new", adminAuth.RequireRoles(newsRoles...), newsAdminHandler.Editor)
		admin.GET("/news/:id/edit", adminAuth.RequireRoles(newsRoles...), newsAdminHandler.Editor)
		admin.POST("/news", adminAuth.RequireRoles(newsRoles...), newsAdminHandler.Create)
		admin.POST("/news/:id/update", adminAuth.RequireRoles(newsRoles...), newsAdminHandler.Update)
		admin.POST("/news/:id/delete", adminAuth.RequireRoles(newsRoles...), newsAdminHandler.Delete)
		admin.POST("/news/upload", adminAuth.RequireRoles(newsRoles...), newsAdminHandler.UploadImage)
	}

	tenantMW := tenant.Middleware(tenant.NewResolver(cfg.BaseDomain, clinicRepo, organizationRepo))
	publicHandler := public.NewHomeHandler(newsSvc, clinicRepo)
	newsPublicHandler := public.NewNewsHandler(newsSvc, clinicRepo)
	// لیست عمومی پزشکان نزدیک‌ترین نوبت را از کش می‌خواند (نه از جدول DoctorSlot)
	bookingListing := booking.NewListingService(doctorRepo, specialtyRepo, clinicRepo, slotCache)
	doctorListHandler := public.NewDoctorListHandler(bookingListing, clinicRepo)

	appointmentRepo := repository.NewAppointmentRepo(conns.Appointment)
	bookingGuard := booking.NewGuard()
	bookingSvc := booking.NewService(hub, slotCache, appointmentRepo, bookingGuard)
	csrfMgr := csrf.NewManager(cfg.SessionSecret)
	bookingHandler := public.NewBookingHandler(doctorRepo, clinicRepo, slotCache, bookingSvc, csrfMgr, bookingGuard)

	pub := r.Group("/", tenantMW)
	{
		pub.GET("/", publicHandler.Get)
		pub.GET("/doctors", doctorListHandler.Get)
		pub.GET("/booking/*path", bookingHandler.Get)
		pub.POST("/booking/*path", bookingHandler.PostSubmit)
		pub.GET("/ws/booking/*path", bookingHandler.ServeBookingWS)
		pub.GET("/news", newsPublicHandler.List)
		pub.GET("/news/:id", newsPublicHandler.GetDetail)
	}

	r.GET("/ws/clinic", wsHandlers.ServeWS)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	log.Printf("server listening on %s", cfg.HTTPAddr)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
