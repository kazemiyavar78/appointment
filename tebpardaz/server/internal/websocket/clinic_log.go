package websocket

import (
	"log"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// ثابت‌های نوع رفتار مرکز روی WebSocket (از شروع اتصال تا پایان).
const (
	ActionConnect     = "connect"      // شروع اتصال مرکز
	ActionDisconnect  = "disconnect"   // قطع اتصال مرکز
	ActionAuthFail    = "auth_fail"    // شکست احراز هویت
	ActionUpgradeFail = "upgrade_fail" // شکست ارتقای HTTP به WebSocket
	ActionRecv        = "recv"         // دریافت پیام از مرکز
	ActionSend        = "send"         // ارسال پیام به مرکز
	ActionReply       = "reply"        // تکمیل رفت‌وبرگشت (پاسخ به درخواست)
	ActionTimeout     = "timeout"      // اتمام مهلت درخواست
	ActionError       = "error"        // خطا در پردازش یا ارسال
)

// clinicLogDB اتصال دیتابیس برای ذخیره لاگ رفتار مرکز است.
var clinicLogDB *gorm.DB

// InitClinicLogDB اتصال دیتابیس لاگ رفتار مرکز را تنظیم می‌کند.
// ورودی: db دیتابیس appointment.
// خروجی: ندارد.
func InitClinicLogDB(db *gorm.DB) {
	clinicLogDB = db
}

// LogClinicBehavior یک رویداد رفتار مرکز را در دیتابیس ثبت می‌کند.
// ورودی: clinicID، action، msgType، requestID، detail، err اختیاری.
// خروجی: ندارد؛ ذخیره به‌صورت غیرمسدود انجام می‌شود.
func LogClinicBehavior(clinicID uint, action, msgType, requestID, detail string, err error) {
	entry := models.ClinicBehaviorLog{
		CreatedAt: time.Now(),
		ClinicID:  clinicID,
		Action:    action,
		MsgType:   msgType,
		RequestID: requestID,
		Detail:    detail,
	}
	if err != nil {
		entry.Error = err.Error()
	}

	// چاپ کوتاه برای مانیتور زنده کنسول
	if entry.Error != "" {
		log.Printf("clinic-ws | clinic=%d | action=%s | type=%s | req=%s | err=%s",
			entry.ClinicID, entry.Action, entry.MsgType, entry.RequestID, entry.Error)
	} else {
		log.Printf("clinic-ws | clinic=%d | action=%s | type=%s | req=%s | detail=%s",
			entry.ClinicID, entry.Action, entry.MsgType, entry.RequestID, entry.Detail)
	}

	if clinicLogDB == nil {
		return
	}

	// ذخیره در دیتابیس بدون مسدود کردن مسیر WebSocket
	go func(row models.ClinicBehaviorLog) {
		if dbErr := clinicLogDB.Create(&row).Error; dbErr != nil {
			log.Printf("clinic-ws | db save failed: %v", dbErr)
		}
	}(entry)
}
