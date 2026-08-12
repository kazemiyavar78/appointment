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
	"tebpardaz/server/internal/testresult"
	"tebpardaz/server/internal/waitingqueue"
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
	// کش پزشکان تأییدنشده — بدون دیتابیس؛ فقط پس از تأیید ادمین در DB ذخیره می‌شوند
	doctorCache := cache.NewDoctorCache(memCache)
	// کش لیست نوبت هفتگی پزشکان — بدون دیتابیس؛ کلاینت هر ۱۵ دقیقه بروزرسانی می‌کند
	weeklyReserveCache := cache.NewWeeklyReserveCache(memCache)
	// کش صف انتظار بیماران — بدون دیتابیس؛ ۱۰ دقیقه در حالت خاموش، ۵ ثانیه وقتی بیمار وصل است
	waitingQueueCache := cache.NewWaitingQueueCache(memCache)
	waitingQueueRefresher := waitingqueue.NewRefresher(hub)
	waitingQueueRefresher.Start()

	adminAuth := auth.NewAdminAuth(userRepo, cfg.SessionSecret)
	approvalHandler := adminhandlers.NewApprovalHandler(doctorRepo, specialtyRepo, clinicRepo, clinicScope, hub, doctorCache)
	appointmentHandler := adminhandlers.NewAppointmentHandler(doctorRepo, clinicRepo, slotCache, clinicScope, hub)
	specialtyHandler := adminhandlers.NewSpecialtyHandler(specialtyRepo)
	userAdminHandler := adminhandlers.NewUserAdminHandler(userRepo, clinicRepo, organizationRepo)
	newsAdminHandler := adminhandlers.NewNewsAdminHandler(newsSvc, clinicScope)
	wsHandlers := websocket.NewHandlers(hub, clinicRepo, doctorRepo, slotCache, doctorCache, weeklyReserveCache, waitingQueueCache)

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
		admin.POST("/approvals/decide",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			approvalHandler.Decide,
		)
		admin.POST("/approvals/update",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			approvalHandler.UpdateApproved,
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
		admin.GET("/users",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			userAdminHandler.List,
		)
		admin.POST("/users",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			userAdminHandler.Create,
		)
		admin.POST("/users/:id/update",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			userAdminHandler.Update,
		)
		admin.POST("/users/:id/deactivate",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			userAdminHandler.Deactivate,
		)
		admin.POST("/users/:id/activate",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			userAdminHandler.Activate,
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
	publicHandler := public.NewHomeHandler(newsSvc, clinicRepo, specialtyRepo)
	newsPublicHandler := public.NewNewsHandler(newsSvc, clinicRepo)
	// لیست عمومی پزشکان نزدیک‌ترین نوبت را از کش می‌خواند (نه از جدول DoctorSlot)
	bookingListing := booking.NewListingService(doctorRepo, specialtyRepo, clinicRepo, slotCache)
	doctorListHandler := public.NewDoctorListHandler(bookingListing, clinicRepo)
	weeklyScheduleHandler := public.NewWeeklyScheduleHandler(clinicRepo, weeklyReserveCache)

	appointmentRepo := repository.NewAppointmentRepo(conns.Appointment)
	bookingGuard := booking.NewGuard()
	bookingSvc := booking.NewService(hub, slotCache, appointmentRepo, bookingGuard)
	csrfMgr := csrf.NewManager(cfg.SessionSecret)
	bookingHandler := public.NewBookingHandler(doctorRepo, clinicRepo, slotCache, bookingSvc, csrfMgr, bookingGuard)
	testResultSvc := testresult.NewService("")
	testResultLimiter := testresult.NewRateLimiter()
	testResultHandler := public.NewTestResultHandler(clinicRepo, testResultSvc, testResultLimiter, csrfMgr, hub)
	waitingQueueHandler := public.NewWaitingQueueHandler(clinicRepo, waitingQueueCache, csrfMgr, waitingQueueRefresher)

	pub := r.Group("/", tenantMW)
	{
		pub.GET("/", publicHandler.Get)
		pub.GET("/doctors", doctorListHandler.Get)
		pub.GET("/weekly-schedule", weeklyScheduleHandler.Get)
		pub.GET("/booking/*path", bookingHandler.Get)
		pub.POST("/booking/*path", bookingHandler.PostSubmit)
		pub.GET("/ws/booking/*path", bookingHandler.ServeBookingWS)
		pub.GET("/news", newsPublicHandler.List)
		pub.GET("/news/:id", newsPublicHandler.GetDetail)
		pub.GET("/test-results", testResultHandler.GetForm)
		pub.POST("/test-results", testResultHandler.PostLookup)
		pub.GET("/waiting-queue", waitingQueueHandler.GetForm)
		pub.POST("/waiting-queue", waitingQueueHandler.PostLookup)
		// لینک QR / مانیتورینگ path-based (ارگان و پلتفرم: با clinic_id)
		pub.GET("/:admission_no/:national_id/:clinic_id/waiting-queue", waitingQueueHandler.GetDeepLink)
		pub.GET("/:admission_no/:national_id/:clinic_id/ws/waiting-queue", waitingQueueHandler.ServeWS)
		// دامنه خصوصی مرکز (بدون clinic_id در مسیر)
		pub.GET("/:admission_no/:national_id/waiting-queue", waitingQueueHandler.GetDeepLink)
		pub.GET("/:admission_no/:national_id/ws/waiting-queue", waitingQueueHandler.ServeWS)
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
