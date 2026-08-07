# معماری سیستم نوبت‌دهی و جوابدهی طب‌پرداز

## نمای کلی

مونو-ریپوی Go Workspace با سه ماژول:

| ماژول | نقش |
|--------|-----|
| `shared` | پروتکل وب‌سوکت، ثابت‌ها، و cipher مشترک |
| `server` | سایت عمومی چند-tenant، پنل ادمین، هاب وب‌سوکت |
| `client` | ایجنت محلی روی سرور هر مرکز (HIS/LIS → سرور مرکزی) |

```
[مرورگر کاربر] → HTTP/templ → [server]
                                    ↕ WebSocket (protocol)
                              [client per clinic]
                                    ↕ RAW SQL / LIS API
                              [DB محلی مرکز / LIS]
```

## Server

- **Gin** برای HTTP، **templ** برای UI، **GORM + SQL Server** برای داده مرکزی
- **Tenant resolver** دامنه/اسلاگ را به کلینیک یا سازمان نگاشت می‌کند
- **WebSocket Hub** کلاینت‌های مراکز را نگه می‌دارد و booking/sync را dispatch می‌کند
- فرانت با **Tailwind CSS 3.4** و PostCSS (autoprefixer + postcss-preset-env) برای سازگاری با مرورگرهای قدیمی (از جمله IE11)

## Client

- کانفیگ YAML به‌ازای هر مرکز (`configs/clinic.example.yaml`)
- درایور پیش‌فرض دیتابیس محلی: **`github.com/microsoft/go-mssqldb`** (SQL Server)
- حلقه‌های sync پزشک/نوبت و پاسخ به درخواست‌های booking و جواب آزمایش

## محدودیت مرورگر (عمومی)

سایت عمومی باید روی Windows 7 و وب‌ویوهای قدیمی اندروید قابل استفاده باشد:

- Tailwind **v3.4** (نه v4 / نه lightningcss / نه oklch)
- رنگ‌ها hex/rgb در `tailwind.config.js`
- `.browserslistrc` شامل IE 11
- بدون `backdrop-filter`، `:has()`، subgrid، container queries در CSS دست‌نویس
- فونت با `@font-face` و fallback `woff2` → `woff`

## وضعیت اسکفولد

منطق کسب‌وکار، migration واقعی، و رمزنگاری واقعی هنوز پیاده نشده‌اند — فقط ساختار و استاب/TODO.
