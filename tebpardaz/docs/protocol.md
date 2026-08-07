# پروتکل ارتباط WebSocket بین server و client

## Envelope

همه‌ی فریم‌ها داخل `shared/protocol.Envelope` هستند:

| فیلد | توضیح |
|------|--------|
| `type` | نوع پیام (`MessageType`) |
| `request_id` | همبستگی درخواست/پاسخ |
| `clinic_key` | شناسه کلاینت مرکز (`WSClientKey`) |
| `timestamp` | unix seconds |
| `payload` | JSON تایپ‌شده |
| `error` | خطای پروتکل (اختیاری) |

## انواع پیام

### پزشکان
| type | جهت | payload |
|------|------|---------|
| `doctor.list.request` | server → client | `DoctorListRequest` |
| `doctor.list.push` | client → server | `DoctorListPush` |
| `doctor.list.ack` | server → client | `DoctorListAck` |
| `doctor.code.update` | client → server | `DoctorCodeUpdate` (بروزرسانی `ExternalID` پزشک تأییدشده روی سایت) |
| `doctor.code.update.ack` | server → client | `DoctorCodeUpdateAck` |
| `doctor.approval.notify` | server → client | `DoctorApprovalNotify` |

### نوبت‌ها
| type | جهت | payload |
|------|------|---------|
| `appointment.list.request` | server → client | `AppointmentListRequest` (`scope=one\|all`) |
| `appointment.list.push` | client → server | `AppointmentListPush` |
| `appointment.list.ack` | server → client | `AppointmentListAck` |

کلاینت هر **۲۰ دقیقه** یک‌بار `scope=all` را برای پزشکان قبلاً ارسال‌شده push می‌کند.

### رزرو از سایت
| type | جهت | payload |
|------|------|---------|
| `booking.create` | server → client | `BookingCreate` |
| `booking.create.ack` | client → server | `BookingCreateAck` |
| `booking.cancel` | server → client | `BookingCancel` |
| `booking.cancel.ack` | client → server | `BookingCancelAck` |

### سایر
| type | جهت | توضیح |
|------|------|--------|
| `ping` / `pong` | دوطرفه | keepalive |
| `test_result.request` / `test_result.response` | server ↔ client | جواب آزمایش |
| `error` | هر طرف | `ProtocolError` |

## سرویس‌های کلاینت

1. دریافت لیست پزشکان → `DoctorService.PushDoctorList` / `localdb.ListDoctors`
2. بروزرسانی کد پزشک تأییدشده روی سایت → `DoctorService.UpdateApprovedDoctorCode`
3. نوبت‌های یک دکتر → `AppointmentService.PushAppointmentsForDoctor`
4. نوبت همه‌ی دکترهای ارسال‌شده (۲۰ دقیقه) → `AppointmentService.PushAppointmentsForAllDoctors` + `SyncScheduler`
5. ذخیره نوبت جدید سایت → `BookingService.HandleCreate` / `localdb.SaveBooking`

SQL محلی عمداً خالی است و بعداً پر می‌شود.

## Tenant / Layout (HTTP)

میدلور `server/internal/tenant` از روی Host تصمیم می‌گیرد:

| تشخیص | Layout |
|--------|--------|
| apex/`www` دامنه پایه | `platform` |
| دامنه اختصاصی کلینیک یا `{slug}.base` | `private` |
| دامنه/اسلاگ سازمان | `organ` |
