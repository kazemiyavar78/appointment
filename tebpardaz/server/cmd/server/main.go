package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tebpardaz/server/internal/analytics"
	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/config"
	"tebpardaz/server/internal/csrf"
	"tebpardaz/server/internal/dashboard"
	"tebpardaz/server/internal/db"
	"tebpardaz/server/internal/filestore"
	adminhandlers "tebpardaz/server/internal/handlers/admin"
	"tebpardaz/server/internal/handlers/public"
	"tebpardaz/server/internal/iprestrict"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/news"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/sms"
	"tebpardaz/server/internal/statusnotify"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/internal/testresult"
	"tebpardaz/server/internal/waitingqueue"
	"tebpardaz/server/internal/webpush"
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

	if err := conns.MigrateManagementClinicBranding(); err != nil {
		log.Fatalf("migrate management clinic branding: %v", err)
	}
	log.Println("migrate: tp_managment clinic branding columns OK")

	userRepo := repository.NewUserRepo(conns.Appointment)
	if err := userRepo.EnsureSuperAdmin(cfg.SuperAdminUser, cfg.SuperAdminPass); err != nil {
		log.Fatalf("seed superadmin: %v", err)
	}

	doctorRepo := repository.NewDoctorRepo(conns.Appointment)
	if updated, err := doctorRepo.MigrateAllDoctorNamesToPersian(); err != nil {
		log.Printf("doctor names migration: %v", err)
	} else if updated > 0 {
		log.Printf("doctor names migration: %d row(s) normalized to Persian", updated)
	}

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
	// کش صف انتظار بیماران — بدون دیتابیس؛ ۱۰ دقیقه در حالت خاموش، ۵ ثانیه وقتی بیمار یا اشتراک پوش وصل است
	waitingQueueCache := cache.NewWaitingQueueCache(memCache)
	pushSubs := cache.NewPushSubscriptionCache(memCache)
	nidLimit := cache.NewNationalIDLimiter(memCache)
	waitingQueueRefresher := waitingqueue.NewRefresher(hub)
	waitingQueueRefresher.HasPushInterest = pushSubs.HasClinic
	waitingQueueRefresher.Start()

	visitCache := analytics.NewMemoryVisitCache()
	visitRepo := analytics.NewGormVisitRepository(conns.Appointment)
	visitTracker := analytics.NewTracker(visitCache, analytics.NewUserAgentParser(), analytics.NewReferrerAnalyzer())
	visitFlusher := analytics.NewFlusher(visitCache, visitRepo, analytics.DefaultFlushInterval)
	visitFlusher.Start()

	adminAuth := auth.NewAdminAuth(userRepo, cfg.SessionSecret, auth.NewAdminOTPStore(), nil)
	approvalHandler := adminhandlers.NewApprovalHandler(doctorRepo, specialtyRepo, clinicRepo, clinicScope, hub, doctorCache)
	appointmentHandler := adminhandlers.NewAppointmentHandler(doctorRepo, clinicRepo, slotCache, clinicScope, hub)
	appointmentRepo := repository.NewAppointmentRepo(conns.Appointment)
	clinicLogRepo := repository.NewClinicBehaviorLogRepo(conns.Appointment)
	clinicLogCleaner := repository.NewClinicBehaviorLogCleaner(clinicLogRepo, 0)
	clinicLogCleaner.Start()
	clinicLogHandler := adminhandlers.NewClinicLogHandler(clinicLogRepo, clinicRepo, clinicScope)
	visitorHandler := adminhandlers.NewVisitorHandler(visitRepo, clinicRepo, clinicScope)
	otpRepo := repository.NewOTPRepo(conns.Appointment)
	registeredAppointmentHandler := adminhandlers.NewRegisteredAppointmentHandler(appointmentRepo, otpRepo, visitRepo, clinicRepo, clinicScope, hub)
	dashboardSvc := dashboard.NewService(conns.Appointment, slotCache)
	dashboardHandler := adminhandlers.NewDashboardHandler(dashboardSvc, clinicRepo, clinicScope)
	clinicBrandingHandler := adminhandlers.NewClinicBrandingHandler(clinicRepo, clinicScope)
	specialtyHandler := adminhandlers.NewSpecialtyHandler(specialtyRepo)
	insuranceRepo := repository.NewInsuranceRepo(conns.Appointment)
	insuranceHandler := adminhandlers.NewInsuranceHandler(insuranceRepo, clinicScope)
	serviceRepo := repository.NewServiceRepo(conns.Appointment)
	sectionRepo := repository.NewSectionRepo(conns.Appointment)
	serviceAdminHandler := adminhandlers.NewServiceHandler(serviceRepo, insuranceRepo, doctorRepo, clinicRepo, clinicScope, sectionRepo)
	sectionAdminHandler := adminhandlers.NewSectionAdminHandler(sectionRepo, clinicRepo, clinicScope)
	public.SetPublicSectionRepo(sectionRepo)
	public.SetPublicClinicRepo(clinicRepo)
	userAdminHandler := adminhandlers.NewUserAdminHandler(userRepo, clinicRepo, organizationRepo)
	newsAdminHandler := adminhandlers.NewNewsAdminHandler(newsSvc, clinicScope)
	wsHandlers := websocket.NewHandlers(hub, clinicRepo, doctorRepo, slotCache, doctorCache, weeklyReserveCache, waitingQueueCache, appointmentRepo)

	r := gin.Default()
	if err := r.SetTrustedProxies(cfg.ProxyList()); err != nil {
		log.Fatalf("trusted proxies: %v", err)
	}
	urlTrust, err := seo.NewProxyTrust(cfg.ProxyList())
	if err != nil {
		log.Fatalf("public url proxies: %v", err)
	}
	public.SetPublicProxyTrust(urlTrust)
	//where am i
	aa, _ := os.Getwd()
	fmt.Println("where am i", aa)
	r.Static("/static", "./static")

	ipRestrictRepo := repository.NewIPRestrictionRepo(conns.Management)
	// آی‌پی بلاک‌شده یا بالاتر از سقف rate limit را قبل از بقیه مسیرها رد می‌کند.
	r.Use(iprestrict.Middleware(ipRestrictRepo))

	var fileStore *filestore.Store
	if strings.TrimSpace(cfg.FileStorageRoot) == "" {
		log.Printf("FILE_STORAGE_ROOT is empty; patient file pages are disabled")
	} else if opened, openErr := filestore.Open(cfg.FileStorageRoot); openErr != nil {
		log.Fatalf("file storage: %v", openErr)
	} else {
		fileStore = opened
		if mkErr := os.MkdirAll(fileStore.Root(), 0o750); mkErr != nil {
			log.Printf("file storage mkdir: %v", mkErr)
		}
	}
	fileDownloadHandler := public.NewFileDownloadHandler(conns.Management, fileStore)
	r.GET("/storage/:category/:month/:clinic_code/:filename", fileDownloadHandler.Show)
	r.GET("/storage/:category/:month/:clinic_code/:filename/download", fileDownloadHandler.Download)
	r.GET("/storage/:category/:month/:clinic_code/:filename/ads", fileDownloadHandler.Ads)
	r.POST("/storage/:category/:month/:clinic_code/:filename/ads", fileDownloadHandler.Ads)
	r.POST("/storage/:category/:month/:clinic_code/:filename/ads/:ad_id/click", fileDownloadHandler.AdClick)
	r.GET("/file-ads/media/:name", fileDownloadHandler.AdMedia)

	r.GET("/admin/login", adminAuth.LoginPage)
	r.POST("/admin/login", adminAuth.Login)
	r.GET("/admin/logout", adminAuth.Logout)

	admin := r.Group("/admin", adminAuth.Middleware())
	{
		adminDashboardRoles := []constants.UserRole{
			constants.UserRoleSuperAdmin,
			constants.UserRoleAdmin,
			constants.UserRoleClinicAdmin,
			constants.UserRoleOrganAdmin,
		}
		admin.GET("", adminAuth.RequireRoles(adminDashboardRoles...), dashboardHandler.Page)
		admin.GET("/", adminAuth.RequireRoles(adminDashboardRoles...), dashboardHandler.Page)
		admin.GET("/dashboard", adminAuth.RequireRoles(adminDashboardRoles...), dashboardHandler.Page)
		admin.GET("/dashboard/data", adminAuth.RequireRoles(adminDashboardRoles...), dashboardHandler.Data)
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
		admin.GET("/bookings",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			registeredAppointmentHandler.List,
		)
		admin.GET("/bookings/otp/:id/patient",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			registeredAppointmentHandler.OTPPatientJSON,
		)
		admin.GET("/bookings/otp/:id/behavior",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			registeredAppointmentHandler.OTPBehaviorJSON,
		)
		admin.GET("/bookings/:id/patient",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			registeredAppointmentHandler.PatientJSON,
		)
		admin.GET("/bookings/:id/behavior",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			registeredAppointmentHandler.BehaviorJSON,
		)
		admin.POST("/bookings/otp/:id/visit-check",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			registeredAppointmentHandler.OTPVisitJSON,
		)
		admin.POST("/bookings/:id/visit-check",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			registeredAppointmentHandler.VisitJSON,
		)
		admin.GET("/logs",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
			),
			clinicLogHandler.List,
		)
		admin.GET("/logs/ws",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
			),
			clinicLogHandler.ServeWS,
		)
		admin.GET("/visitors",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			visitorHandler.ListIPs,
		)
		admin.GET("/visitors/visits",
			adminAuth.RequireRoles(
				constants.UserRoleSuperAdmin,
				constants.UserRoleAdmin,
				constants.UserRoleClinicAdmin,
				constants.UserRoleOrganAdmin,
			),
			visitorHandler.ListVisits,
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
		clinicBrandingRoles := []constants.UserRole{
			constants.UserRoleSuperAdmin,
			constants.UserRoleAdmin,
			constants.UserRoleClinicAdmin,
			constants.UserRoleOrganAdmin,
		}
		admin.GET("/clinic/branding",
			adminAuth.RequireRoles(clinicBrandingRoles...),
			clinicBrandingHandler.Form,
		)
		admin.POST("/clinic/branding",
			adminAuth.RequireRoles(clinicBrandingRoles...),
			clinicBrandingHandler.Save,
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
		admin.GET("/services/packages",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.ListPackages,
		)
		admin.POST("/services/packages",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.CreatePackage,
		)
		admin.POST("/services/packages/:id/update",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.UpdatePackage,
		)
		admin.POST("/services/packages/:id/delete",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.DeletePackage,
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
		admin.GET("/services/section-packages",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.SectionPackagesForm,
		)
		admin.POST("/services/section-packages",
			adminAuth.RequireRoles(constants.UserRoleSuperAdmin),
			serviceAdminHandler.SaveSectionPackages,
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

		admin.GET("/sections/:id/doctors", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.DoctorsList)
		admin.POST("/sections/:id/doctors", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.DoctorAssign)
		admin.POST("/sections/:id/doctors/:doctor_id/delete", adminAuth.RequireRoles(sectionRoles...), sectionAdminHandler.DoctorRemove)
	}

	tenantResolver := tenant.NewResolver(cfg.BaseDomain, clinicRepo, organizationRepo)
	// جستجوی دامنه و اسلاگ مستأجر از کش حافظه خوانده می‌شود.
	tenantResolver.UseCache(cache.NewTenantCache(memCache))
	tenantMW := tenant.Middleware(tenantResolver, visitTracker)
	// لیست عمومی پزشکان نزدیک‌ترین نوبت را از کش می‌خواند (نه از جدول DoctorSlot)
	bookingListing := booking.NewListingService(doctorRepo, specialtyRepo, clinicRepo, slotCache)
	sectionPublicHandler := public.NewSectionPublicHandler(sectionRepo, clinicRepo, bookingListing, serviceRepo)
	publicHandler := public.NewHomeHandler(newsSvc, clinicRepo, specialtyRepo, insuranceRepo, bookingListing)
	newsPublicHandler := public.NewNewsHandler(newsSvc, clinicRepo)
	doctorListHandler := public.NewDoctorListHandler(bookingListing, clinicRepo)
	weeklyScheduleHandler := public.NewWeeklyScheduleHandler(clinicRepo, weeklyReserveCache)

	reviewRepo := repository.NewReviewRepo(conns.Appointment)
	bookingGuard := booking.NewGuard()
	// بلاک ثبت نوبت را در جدول ip_restrictions هم ذخیره می‌کند.
	bookingGuard.SetBanPersister(func(ip, reason string, until time.Time) {
		exp := until
		if err := ipRestrictRepo.Block(ip, models.IPRestrictionScopeGlobal, reason, &exp); err != nil {
			log.Printf("ip restriction: persist ban: %v", err)
		}
	})
	otpStore := booking.NewOTPStore(conns.Appointment)
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
	mgmtUsers := repository.NewManagementUserRepo(conns.Management)
	statusNotifier := statusnotify.New(mgmtUsers, clinicRepo, smsClient)
	adminAuth.Status = statusNotifier
	hub.SetPresenceHook(statusNotifier.OnClinicPresence)
	bookingSvc := booking.NewService(hub, slotCache, appointmentRepo, bookingGuard, otpStore, bookingNotifier, statusNotifier)
	csrfMgr := csrf.NewManager(cfg.SessionSecret)
	bookingHandler := public.NewBookingHandler(doctorRepo, clinicRepo, slotCache, bookingSvc, csrfMgr, bookingGuard, reviewRepo, otpStore, smsClient, serviceRepo, statusNotifier)
	staticHandler := public.NewStaticPageHandler(clinicRepo, reviewRepo, csrfMgr)
	reviewAdminHandler := adminhandlers.NewReviewAdminHandler(reviewRepo, clinicRepo, doctorRepo, clinicScope)
	testResultSvc := testresult.NewService("")
	testResultLimiter := testresult.NewRateLimiter()
	testResultHandler := public.NewTestResultHandler(clinicRepo, sectionRepo, testResultSvc, testResultLimiter, csrfMgr, hub)
	waitingQueueHandler := public.NewWaitingQueueHandler(clinicRepo, sectionRepo, waitingQueueCache, csrfMgr, waitingQueueRefresher)
	vapidKeys, err := webpush.LoadKeys(cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject, cfg.VAPIDKeysFile)
	if err != nil {
		log.Fatalf("vapid: %v", err)
	}
	log.Printf("vapid: public key ready")
	waitingQueueHandler.Submissions = repository.NewWaitingQueueSubmissionRepo(conns.Appointment)
	waitingQueueHandler.NIDLimit = nidLimit
	waitingQueueHandler.VAPIDPublicKey = vapidKeys.PublicKey
	pushHandler := public.NewWaitingQueuePushHandler(waitingQueueHandler, pushSubs)
	pwaHandler := public.NewPWAHandler()
	wsHandlers.OnWaitingQueueUpdated = waitingqueue.NewPushNotifier(pushSubs, waitingQueueCache, webpush.NewSender(vapidKeys)).OnQueueUpdated
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

	pub := r.Group("/", tenantMW, public.AbortIfTenantMissing)
	{
		public.GETAndHEAD(pub, "/", publicHandler.Get)
		public.GETAndHEAD(pub, "/doctors", doctorListHandler.Get)
		public.GETAndHEAD(pub, "/weekly-schedule", weeklyScheduleHandler.Get)
		pub.POST("/otp/send", bookingHandler.SendOTP)
		pub.POST("/otp/verify", bookingHandler.VerifyOTP)
		pub.GET("/patient/lookup", bookingHandler.LookupPatient)
		public.GETAndHEAD(pub, "/booking/*path", bookingHandler.Get)
		pub.POST("/booking/*path", bookingHandler.PostSubmit)
		pub.GET("/ws/booking/*path", bookingHandler.ServeBookingWS)
		public.GETAndHEAD(pub, "/news", newsPublicHandler.List)
		public.GETAndHEAD(pub, "/news/:id", newsPublicHandler.GetDetail)

		// لیست بخش‌های درمانی (همراه با تگ مرکز در لایه ارگان و پلتفرم)
		public.GETAndHEAD(pub, "/sections", sectionPublicHandler.ListSections)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/sections", sectionPublicHandler.ListSections)

		// صفحه اصلی یک بخش مشخص و زیرصفحات تخصصی آن در دامنه مرکز
		public.GETAndHEAD(pub, "/section/:slug", sectionPublicHandler.Get)
		public.GETAndHEAD(pub, "/section/:slug/ساعات-کاری", sectionPublicHandler.GetWorkingHours)
		public.GETAndHEAD(pub, "/section/:slug/working-hours", sectionPublicHandler.GetWorkingHours)
		public.GETAndHEAD(pub, "/section/:slug/پیام-به-مراجعین", sectionPublicHandler.GetMessages)
		public.GETAndHEAD(pub, "/section/:slug/message-to-visitors", sectionPublicHandler.GetMessages)
		public.GETAndHEAD(pub, "/section/:slug/تجهیزات", sectionPublicHandler.GetEquipment)
		public.GETAndHEAD(pub, "/section/:slug/equipment", sectionPublicHandler.GetEquipment)
		public.GETAndHEAD(pub, "/section/:slug/معرفی", sectionPublicHandler.GetBannerIntro)

		// صفحه اصلی یک بخش مشخص و زیرصفحات تخصصی آن در لایه ارگان و پلتفرم
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/section/:slug", sectionPublicHandler.Get)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/section/:slug/ساعات-کاری", sectionPublicHandler.GetWorkingHours)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/section/:slug/working-hours", sectionPublicHandler.GetWorkingHours)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/section/:slug/پیام-به-مراجعین", sectionPublicHandler.GetMessages)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/section/:slug/message-to-visitors", sectionPublicHandler.GetMessages)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/section/:slug/تجهیزات", sectionPublicHandler.GetEquipment)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/section/:slug/equipment", sectionPublicHandler.GetEquipment)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/section/:slug/معرفی", sectionPublicHandler.GetBannerIntro)

		// مسیرهای عمومی سطح مرکز (در صورت فراخوانی بدون تعیین بخش، لیست بخش‌ها باز می‌شود)
		public.GETAndHEAD(pub, "/ساعات-کاری", sectionPublicHandler.GetWorkingHours)
		public.GETAndHEAD(pub, "/پیام-به-مراجعین", sectionPublicHandler.GetMessages)
		public.GETAndHEAD(pub, "/تجهیزات", sectionPublicHandler.GetEquipment)
		public.GETAndHEAD(pub, "/برنامه-هفتگی-پزشکان", weeklyScheduleHandler.Get)
		public.GETAndHEAD(pub, "/معرفی", sectionPublicHandler.GetBannerIntro)

		// مسیرهای پشتیبان انگلیسی سطح مرکز
		public.GETAndHEAD(pub, "/working-hours", sectionPublicHandler.GetWorkingHours)
		public.GETAndHEAD(pub, "/message-to-visitors", sectionPublicHandler.GetMessages)
		public.GETAndHEAD(pub, "/equipment", sectionPublicHandler.GetEquipment)

		// مسیرهای پشتیبان سطح ارگان/پلتفرم
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/ساعات-کاری", sectionPublicHandler.GetWorkingHours)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/پیام-به-مراجعین", sectionPublicHandler.GetMessages)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/تجهیزات", sectionPublicHandler.GetEquipment)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/برنامه-هفتگی-پزشکان", weeklyScheduleHandler.Get)
		public.GETAndHEAD(pub, "/clinics/:clinic_slug/معرفی", sectionPublicHandler.GetBannerIntro)

		public.GETAndHEAD(pub, "/about", staticHandler.About)
		public.GETAndHEAD(pub, "/contact", staticHandler.Contact)
		public.GETAndHEAD(pub, "/terms", staticHandler.Terms)
		pub.POST("/reviews", staticHandler.PostReview)
		public.GETAndHEAD(pub, "/test-results", testResultHandler.GetForm)
		pub.POST("/test-results", testResultHandler.PostLookup)
		pub.GET("/waiting-queue", waitingQueueHandler.GetForm)
		pub.POST("/waiting-queue", waitingQueueHandler.PostLookup)
		pub.GET("/sw.js", pwaHandler.ServiceWorker)
		pub.GET("/manifest.webmanifest", pwaHandler.Manifest)
		public.GETAndHEAD(pub, "/sitemap.xml", sitemapHandler.ServeSitemap)
		public.GETAndHEAD(pub, "/robots.txt", robotsHandler.ServeRobots)
		// لینک QR / مانیتورینگ path-based (ارگان و پلتفرم: با clinic_id)
		pub.GET("/:admission_no/:national_id/:clinic_id/waiting-queue", waitingQueueHandler.GetDeepLink)
		pub.GET("/:admission_no/:national_id/:clinic_id/ws/waiting-queue", waitingQueueHandler.ServeWS)
		pub.POST("/:admission_no/:national_id/:clinic_id/push-subscribe", pushHandler.Subscribe)
		// دامنه خصوصی مرکز (بدون clinic_id در مسیر)
		pub.GET("/:admission_no/:national_id/waiting-queue", waitingQueueHandler.GetDeepLink)
		pub.GET("/:admission_no/:national_id/ws/waiting-queue", waitingQueueHandler.ServeWS)
		pub.POST("/:admission_no/:national_id/push-subscribe", pushHandler.Subscribe)
	}

	r.NoRoute(tenantMW, public.NotFound)

	r.GET("/ws/clinic", wsHandlers.ServeWS)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}

	go func() {
		log.Printf("server listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("server shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	visitTracker.Wait()
	visitFlusher.Stop()
	waitingQueueRefresher.Stop()
	clinicLogCleaner.Stop()
	log.Println("server stopped")
}
