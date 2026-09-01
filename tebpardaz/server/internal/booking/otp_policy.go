package booking

import "time"

// سیاست‌های OTP عمومی رزرو آنلاین.
const (
	// otpLength طول کد عددی پیامکی.
	otpLength = 5
	// otpCodeTTL مدت اعتبار کد تایید پس از ارسال.
	otpCodeTTL = 2 * time.Hour
	// otpSessionTTL مدت اعتبار نشست پس از تایید موفق.
	otpSessionTTL = 2 * time.Hour
	// otpResendCooldown حداقل فاصله بین دو ارسال متوالی.
	otpResendCooldown = 120 * time.Second
	// otpMaxVerifyAttempts حداکثر تلاش اشتباه برای وارد کردن کد.
	otpMaxVerifyAttempts = 5
	// otpMaxSendsPerWindow حداکثر تعداد ارسال در یک پنجره.
	otpMaxSendsPerWindow = 2
	// otpSendWindow طول پنجره محدودیت ارسال.
	otpSendWindow = 2 * time.Hour
	// otpMaxBookingsPerSession حداکثر نوبت با یک نشست تاییدشده.
	otpMaxBookingsPerSession = 5
)
