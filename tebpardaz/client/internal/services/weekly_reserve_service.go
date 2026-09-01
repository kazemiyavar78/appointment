package services

import (
	"tebpardaz/client/internal/localdb"
	"tebpardaz/shared/protocol"
)

// WeeklyReserveService لیست نوبت هفتگی پزشکان را از HIS محلی به سایت مرکزی پوش می‌کند.
// سرور این داده را در دیتابیس ذخیره نمی‌کند؛ فقط در کش نگه می‌دارد.
type WeeklyReserveService struct {
	Store  *localdb.Store
	Sender Sender
}

// NewWeeklyReserveService سازنده WeeklyReserveService است.
// ورودی: store (دسترسی HIS محلی)، sender (خروجی WebSocket).
// خروجی: اشاره‌گر به WeeklyReserveService.
func NewWeeklyReserveService(store *localdb.Store, sender Sender) *WeeklyReserveService {
	return &WeeklyReserveService{Store: store, Sender: sender}
}

// PushWeeklyReserves رزروهای هفتگی را از HIS می‌خواند و به سرور پوش می‌کند.
// ورودی: requestID برای همبستگی با درخواست سرور (یا شناسه سینک خودکار).
// خروجی: خطا از خواندن محلی یا ارسال WebSocket.
func (s *WeeklyReserveService) PushWeeklyReserves(requestID string) error {
	if s == nil || s.Store == nil || s.Sender == nil {
		return nil
	}
	rows, shifts, err := s.Store.GetReserve()
	if err != nil {
		return err
	}
	items := make([]protocol.WeeklyReserveDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, protocol.WeeklyReserveDTO{
			DoctorFullName:  row.DoctorFullName,
			ReserveCount:    row.ReserveCount,
			ReserveMaxCount: row.ReserveMaxCount,
			ReserveDate:     row.ReserveDate,
			ReserveTime:     row.ReserveTime,
			Speciality:      row.Speciality,
			ShiftName:       row.ShiftName,
			VisitTime:       row.VisitTime,
			OutTime:         row.OutTime,
		})
	}
	env, err := protocol.NewEnvelope(protocol.TypeWeeklyReserveListPush, requestID, protocol.WeeklyReserveListPush{
		Reserves: items,
		Shifts:   shifts,
	})
	if err != nil {
		return err
	}
	return s.Sender.SendEnvelope(env)
}

// HandleListRequest به weekly_reserve.list.request سمت سرور پاسخ می‌دهد.
// ورودی: requestID، req (در حال حاضر بدون فیلد).
// خروجی: خطا از PushWeeklyReserves.
func (s *WeeklyReserveService) HandleListRequest(requestID string, req *protocol.WeeklyReserveListRequest) error {
	_ = req
	return s.PushWeeklyReserves(requestID)
}
