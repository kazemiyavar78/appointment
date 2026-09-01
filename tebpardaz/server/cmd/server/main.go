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
	"tebpardaz/server/internal/sms"
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
	// اتصال دیتابیس برای ثبت لاگ رفتار مراکز روی WebSocket
	websocket.InitClinicLogDB(conns.Appointment)

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
	insuranceRepo := repository.NewInsuranceRepo(conns.Appointment)
	insuranceHandler := adminhandlers.NewInsuranceHandler(insuranceRepo, clinicScope)
	serviceRepo := repository.NewServiceRepo(conns.Appointment)
	serviceAdminHandler := adminhandlers.NewServiceHandler(serviceRepo, insuranceRepo, doctorRepo, clinicRepo, clinicScope)
	sectionRepo := repository.NewSectionRepo(conns.Appointment)
	sectionAdminHandler := adminhandlers.NewSectionAdminHandler(sectionRepo, clinicRepo, clinicScope)
	public.SetPublicSectionRepo(sectionRepo)
	public.SetPublicClinicRepo(clinicRepo)
	sectionPublicHandler := public.NewSectionPublicHandler(sectionRepo, clinicRepo)
	userAdminHandler := adminhandlers.NewUserAdminHandler(userRepo, clinicRepo, organizationRepo)
	newsAdminHandler := adminhandlers.NewNewsAdminHandler(newsSvc, clinicScope)
	wsHandlers := websocket.NewHandlers(hub, clinicRepo, doctorRepo, slotCache, doctorCache, weeklyReserveCache, waitingQueueCache)

	r := gin.Default()
	//where am i
	aa, _ := os.Getwd()
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
		admin.POST("/approvals/set-active",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			approvalHandler.SetActive,
		)
		admin.POST("/approvals/delete",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			approvalHandler.DeleteApproved,
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
		insuranceAssignRoles := []constants.UserRole{
			constants.UserRoleSuperAdmin,
			constants.UserRoleAdmin,
			constants.UserRoleClinicAdmin,
			constants.UserRoleOrganAdmin,
		}
		admin.GET("/insurances",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			insuranceHandler.List,
		)
		admin.POST("/insurances",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			insuranceHandler.Create,
		)
		admin.GET("/insurances/assign",
			adminAuth.RequireRoles(insuranceAssignRoles...),
			insuranceHandler.AssignForm,
		)
		admin.POST("/insurances/assign",
			adminAuth.RequireRoles(insuranceAssignRoles...),
			insuranceHandler.SaveAssign,
		)
		admin.POST("/insurances/:id/update",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			insuranceHandler.Update,
		)
		admin.POST("/insurances/:id/delete",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			insuranceHandler.Delete,
		)
		admin.GET("/services",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.List,
		)
		admin.POST("/services",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.Create,
		)
		admin.POST("/services/:id/update",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.Update,
		)
		admin.POST("/services/:id/delete",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.Delete,
		)
		admin.GET("/services/clinic-insurances",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.ClinicInsuranceServicesForm,
		)
		admin.POST("/services/clinic-insurances",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.SaveClinicInsuranceServices,
		)
		admin.GET("/services/doctor-services",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.DoctorServicesForm,
		)
		admin.POST("/services/doctor-services",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.SaveDoctorServices,
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

		sectionRoles := []constants.UserRole{
			constants.UserRoleSuperAdmin,
			constants.UserRoleAdmin,
			constants.UserRoleClinicAdmin,
			constants.UserRoleOrganAdmin,
		}
		admin.GET("/sections", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.List)
		admin.POST("/sections", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.Create)
		admin.POST("/sections/quota", adminAuth.RequireRoles(constants.UserRoleSuperAdmin), sectionAdminHandler.UpdateQuota)
		admin.POST("/sections/:id/update", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.Update)
		admin.POST("/sections/:id/delete", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.Delete)

		admin.GET("/sections/:id/banner", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.BannerEdit)
		admin.POST("/sections/:id/banner", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.BannerSave)

		admin.GET("/sections/:id/schedule", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.ScheduleEdit)
		admin.POST("/sections/:id/schedule", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.ScheduleSave)

		admin.GET("/sections/:id/messages", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.MessagesList)
		admin.POST("/sections/:id/messages", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.MessageCreate)
		admin.POST("/sections/:id/messages/:msg_id/update", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.MessageUpdate)
		admin.POST("/sections/:id/messages/:msg_id/delete", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.MessageDelete)

		admin.GET("/sections/:id/equipment", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.EquipmentList)
		admin.POST("/sections/:id/equipment", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.EquipmentCreate)
		admin.POST("/sections/:id/equipment/:eq_id/update", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.EquipmentUpdate)
		admin.POST("/sections/:id/equipment/:eq_id/delete", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.EquipmentDelete)
	}

	tenantMW := tenant.Middleware(tenant.NewResolver(cfg.BaseDomain, clinicRepo, organizationRepo))
	// لیست عمومی پزشکان نزدیک‌ترین نوبت را از کش می‌خواند (نه از جدول DoctorSlot)
	bookingListing := booking.NewListingService(doctorRepo, specialtyRepo, clinicRepo, slotCache)
	publicHandler := public.NewHomeHandler(newsSvc, clinicRepo, specialtyRepo, insuranceRepo, bookingListing)
	newsPublicHandler := public.NewNewsHandler(newsSvc, clinicRepo)
	doctorListHandler := public.NewDoctorListHandler(bookingListing, clinicRepo)
	weeklyScheduleHandler := public.NewWeeklyScheduleHandler(clinicRepo, weeklyReserveCache)

	appointmentRepo := repository.NewAppointmentRepo(conns.Appointment)
	reviewRepo := repository.NewReviewRepo(conns.Appointment)
	bookingGuard := booking.NewGuard()
	otpStore := booking.NewOTPStore()
	smsClient, err := sms.NewClient(sms.Config{
		BaseURL:    cfg.MessagingBaseURL,
		AuthToken:  cfg.MessagingAuthToken,
		AESKeyB64:  cfg.MessagingAESKey,
		HMACKeyB64: cfg.MessagingHMACKey,
		UserCode:   cfg.MessagingUserCode,
		Messenger:  cfg.MessagingMessenger,
		Operator:   cfg.MessagingOperator,
	})
	if err != nil {
		log.Fatalf("messaging client: %v", err)
	}
	bookingNotifier := booking.NewNotifier(smsClient)
	bookingSvc := booking.NewService(hub, slotCache, appointmentRepo, bookingGuard, otpStore, bookingNotifier)
	csrfMgr := csrf.NewManager(cfg.SessionSecret)
	bookingHandler := public.NewBookingHandler(doctorRepo, clinicRepo, slotCache, bookingSvc, csrfMgr, bookingGuard, reviewRepo, otpStore, smsClient, serviceRepo)
	staticHandler := public.NewStaticPageHandler(clinicRepo, reviewRepo, csrfMgr)
	reviewAdminHandler := adminhandlers.NewReviewAdminHandler(reviewRepo, clinicRepo, clinicScope)
	testResultSvc := testresult.NewService("")
	testResultLimiter := testresult.NewRateLimiter()
	testResultHandler := public.NewTestResultHandler(clinicRepo, sectionRepo, testResultSvc, testResultLimiter, csrfMgr, hub)
	waitingQueueHandler := public.NewWaitingQueueHandler(clinicRepo, sectionRepo, waitingQueueCache, csrfMgr, waitingQueueRefresher)
	sitemapHandler := public.NewSitemapHandler(clinicRepo, doctorRepo, sectionRepo, newsRepo)
	robotsHandler := public.NewRobotsHandler()

	admin.GET("/reviews",
		adminAuth.RequireRoles(
			constants.UserRoleSuperAdmin,
			constants.UserRoleAdmin,
			constants.UserRoleClinicAdmin,
			constants.UserRoleOrganAdmin,
		),
		reviewAdminHandler.List,
	)
	admin.POST("/reviews/decide",
		adminAuth.RequireRoles(
			constants.UserRoleSuperAdmin,
			constants.UserRoleAdmin,
			constants.UserRoleClinicAdmin,
			constants.UserRoleOrganAdmin,
		),
		reviewAdminHandler.Decide,
	)

	pub := r.Group("/", tenantMW)
	{
		pub.GET("/", publicHandler.Get)
		pub.GET("/doctors", doctorListHandler.Get)
		pub.GET("/weekly-schedule", weeklyScheduleHandler.Get)
		pub.POST("/otp/send", bookingHandler.SendOTP)
		pub.POST("/otp/verify", bookingHandler.VerifyOTP)
		pub.GET("/patient/lookup", bookingHandler.LookupPatient)
		pub.GET("/booking/*path", bookingHandler.Get)
		pub.POST("/booking/*path", bookingHandler.PostSubmit)
		pub.GET("/ws/booking/*path", bookingHandler.ServeBookingWS)
		pub.GET("/news", newsPublicHandler.List)
		pub.GET("/news/:id", newsPublicHandler.GetDetail)

		// لیست بخش‌های درمانی (همراه با تگ مرکز در لایه ارگان و پلتفرم)
		pub.GET("/sections", sectionPublicHandler.ListSections)
		pub.GET("/clinics/:clinic_slug/sections", sectionPublicHandler.ListSections)

		// صفحه اصلی یک بخش مشخص و زیرصفحات تخصصی آن در دامنه مرکز
		pub.GET("/section/:slug", sectionPublicHandler.Get)
		pub.GET("/section/:slug/ساعات-کاری", sectionPublicHandler.GetWorkingHours)
		pub.GET("/section/:slug/working-hours", sectionPublicHandler.GetWorkingHours)
		pub.GET("/section/:slug/پیام-به-مراجعین", sectionPublicHandler.GetMessages)
		pub.GET("/section/:slug/message-to-visitors", sectionPublicHandler.GetMessages)
		pub.GET("/section/:slug/تجهیزات", sectionPublicHandler.GetEquipment)
		pub.GET("/section/:slug/equipment", sectionPublicHandler.GetEquipment)
		pub.GET("/section/:slug/معرفی", sectionPublicHandler.GetBannerIntro)

		// صفحه اصلی یک بخش مشخص و زیرصفحات تخصصی آن در لایه ارگان و پلتفرم
		pub.GET("/clinics/:clinic_slug/section/:slug", sectionPublicHandler.Get)
		pub.GET("/clinics/:clinic_slug/section/:slug/ساعات-کاری", sectionPublicHandler.GetWorkingHours)
		pub.GET("/clinics/:clinic_slug/section/:slug/working-hours", sectionPublicHandler.GetWorkingHours)
		pub.GET("/clinics/:clinic_slug/section/:slug/پیام-به-مراجعین", sectionPublicHandler.GetMessages)
		pub.GET("/clinics/:clinic_slug/section/:slug/message-to-visitors", sectionPublicHandler.GetMessages)
		pub.GET("/clinics/:clinic_slug/section/:slug/تجهیزات", sectionPublicHandler.GetEquipment)
		pub.GET("/clinics/:clinic_slug/section/:slug/equipment", sectionPublicHandler.GetEquipment)
		pub.GET("/clinics/:clinic_slug/section/:slug/معرفی", sectionPublicHandler.GetBannerIntro)

		// مسیرهای عمومی سطح مرکز (در صورت فراخوانی بدون تعیین بخش، لیست بخش‌ها باز می‌شود)
		pub.GET("/ساعات-کاری", sectionPublicHandler.GetWorkingHours)
		pub.GET("/پیام-به-مراجعین", sectionPublicHandler.GetMessages)
		pub.GET("/تجهیزات", sectionPublicHandler.GetEquipment)
		pub.GET("/برنامه-هفتگی-پزشکان", weeklyScheduleHandler.Get)
		pub.GET("/معرفی", sectionPublicHandler.GetBannerIntro)

		// مسیرهای پشتیبان انگلیسی سطح مرکز
		pub.GET("/working-hours", sectionPublicHandler.GetWorkingHours)
		pub.GET("/message-to-visitors", sectionPublicHandler.GetMessages)
		pub.GET("/equipment", sectionPublicHandler.GetEquipment)

		// مسیرهای پشتیبان سطح ارگان/پلتفرم
		pub.GET("/clinics/:clinic_slug/ساعات-کاری", sectionPublicHandler.GetWorkingHours)
		pub.GET("/clinics/:clinic_slug/پیام-به-مراجعین", sectionPublicHandler.GetMessages)
		pub.GET("/clinics/:clinic_slug/تجهیزات", sectionPublicHandler.GetEquipment)
		pub.GET("/clinics/:clinic_slug/برنامه-هفتگی-پزشکان", weeklyScheduleHandler.Get)
		pub.GET("/clinics/:clinic_slug/معرفی", sectionPublicHandler.GetBannerIntro)

		pub.GET("/about", staticHandler.About)
		pub.GET("/contact", staticHandler.Contact)
		pub.GET("/terms", staticHandler.Terms)
		pub.POST("/reviews", staticHandler.PostReview)
		pub.GET("/test-results", testResultHandler.GetForm)
		pub.POST("/test-results", testResultHandler.PostLookup)
		pub.GET("/waiting-queue", waitingQueueHandler.GetForm)
		pub.POST("/waiting-queue", waitingQueueHandler.PostLookup)
		pub.GET("/sitemap.xml", sitemapHandler.ServeSitemap)
		pub.GET("/robots.txt", robotsHandler.ServeRobots)
		// لینک QR / مانیتورینگ path-based (ارگان و پلتفرم: با clinic_id)
		pub.GET("/:admission_no/:national_id/:clinic_id/waiting-queue", waitingQueueHandler.GetDeepLink)
		pub.GET("/:admission_no/:national_id/:clinic_id/ws/waiting-queue", waitingQueueHandler.ServeWS)
		// دامنه خصوصی مرکز (بدون clinic_id در مسیر)
		pub.GET("/:admission_no/:national_id/waiting-queue", waitingQueueHandler.GetDeepLink)
		pub.GET("/:admission_no/:national_id/ws/waiting-queue", waitingQueueHandler.ServeWS)
	}

	r.NoRoute(tenantMW, public.NotFound)

	r.GET("/ws/clinic", wsHandlers.ServeWS)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	log.Printf("server listening on %s", cfg.HTTPAddr)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
