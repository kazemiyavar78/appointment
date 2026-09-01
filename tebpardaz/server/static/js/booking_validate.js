/**
 * اعتبارسنجی سمت کلاینت فرم رزرو — هم‌راستا با booking/validate.go
 */
(function (global) {
  "use strict";

  var PERSIAN_NAME = /^[\u0600-\u06FF\uFB50-\uFDFF\uFE70-\uFEFF\s\u200c]+$/;
  var MOBILE_RE = /^09\d{9}$/;

  /** ارقام فارسی/عربی را به ASCII تبدیل می‌کند. */
  function digitsOnly(s) {
    var out = "";
    var i;
    s = (s || "").trim();
    for (i = 0; i < s.length; i++) {
      var c = s.charCodeAt(i);
      if (c >= 48 && c <= 57) out += s.charAt(i);
      else if (c >= 1776 && c <= 1785) out += String(c - 1776);
      else if (c >= 1632 && c <= 1641) out += String(c - 1632);
    }
    return out;
  }

  /** بررسی checksum کد ملی ایران. */
  function isValidNationalID(id) {
    id = digitsOnly(id);
    if (id.length !== 10) return false;
    if (/^(\d)\1{9}$/.test(id)) return false;
    var sum = 0;
    var i;
    for (i = 0; i < 9; i++) sum += parseInt(id.charAt(i), 10) * (10 - i);
    var r = sum % 11;
    var check = parseInt(id.charAt(9), 10);
    if (r < 2) return check === r;
    return check === 11 - r;
  }

  /** نام فارسی معتبر. */
  function isPersianName(s) {
    s = normalizeName(s);
    return s.length > 0 && PERSIAN_NAME.test(s);
  }

  /** نرمال‌سازی ی/ک عربی. */
  function normalizeName(s) {
    return (s || "")
      .trim()
      .replace(/ي/g, "ی")
      .replace(/ك/g, "ک")
      .replace(/\s+/g, " ");
  }

  /** موبایل ۱۱ رقمی با ۰۹. */
  function isValidMobile(m) {
    return MOBILE_RE.test(digitsOnly(m));
  }

  /** تبدیل ارقام تاریخ تولد. */
  function normalizeBirthDigits(s) {
    return digitsOnly((s || "").replace(/-/g, "/"));
  }

  /**
   * اعتبار تاریخ تولد شمسی yyyy/mm/dd
   * ورودی: رشته تاریخ. خروجی: {ok, message}
   */
  function validateBirthDate(raw) {
    raw = (raw || "").trim().replace(/-/g, "/");
    if (!raw) return { ok: false, message: "تاریخ تولد را انتخاب کنید" };
    var parts = raw.split("/");
    if (parts.length !== 3) return { ok: false, message: "تاریخ تولد نامعتبر است" };
    var y = parseInt(digitsOnly(parts[0]), 10);
    var m = parseInt(digitsOnly(parts[1]), 10);
    var d = parseInt(digitsOnly(parts[2]), 10);
    if (!y || !m || !d || y < 1200 || y > 1600) {
      return { ok: false, message: "تاریخ تولد نامعتبر است" };
    }
    if (typeof persianDate !== "undefined") {
      try {
        var pd = new persianDate([y, m, d]);
        var g = pd.toCalendar("gregorian").toDate();
        var now = new Date();
        var minAge = new Date(now.getFullYear() - 1, now.getMonth(), now.getDate());
        var maxAge = new Date(now.getFullYear() - 120, now.getMonth(), now.getDate());
        if (g > minAge) return { ok: false, message: "حداقل سن بیمار ۱ سال است" };
        if (g < maxAge) return { ok: false, message: "تاریخ تولد نامعتبر است" };
      } catch (e) {
        return { ok: false, message: "تاریخ تولد نامعتبر است" };
      }
    }
    return { ok: true, message: "" };
  }

  /** اعتبارسنجی کامل فرم بیمار. */
  function validatePatientForm(form) {
    var errors = {};
    var fd = new FormData(form);
    var first = normalizeName(fd.get("first_name"));
    var last = normalizeName(fd.get("last_name"));
    var nid = digitsOnly(fd.get("national_id"));
    var mobile = digitsOnly(fd.get("mobile"));
    var birth = fd.get("birth_date");
    var sex = fd.get("sex");

    if (!isPersianName(first)) errors.first_name = "نام باید فارسی باشد";
    if (!isPersianName(last)) errors.last_name = "نام خانوادگی باید فارسی باشد";
    if (!isValidNationalID(nid)) errors.national_id = "کد ملی معتبر نیست";
    if (!isValidMobile(mobile)) errors.mobile = "موبایل باید ۱۱ رقم و با ۰۹ شروع شود";
    var bd = validateBirthDate(birth);
    if (!bd.ok) errors.birth_date = bd.message;
    if (sex !== "مرد" && sex !== "زن") errors.sex = "جنسیت را انتخاب کنید";

    return { ok: Object.keys(errors).length === 0, errors: errors };
  }

  /** نمایش خطاهای فیلد زیر input. */
  function showFieldErrors(errors) {
    var fields = ["national_id", "first_name", "last_name", "mobile", "birth_date", "sex"];
    var i;
    for (i = 0; i < fields.length; i++) {
      var name = fields[i];
      var errEl = document.getElementById("err-" + name);
      var input = document.getElementById(name) || document.querySelector('[name="' + name + '"]');
      if (errEl) {
        if (errors[name]) {
          errEl.textContent = errors[name];
          errEl.classList.remove("bk-hidden");
          errEl.classList.add("is-visible");
        } else {
          errEl.textContent = "";
          errEl.classList.add("bk-hidden");
          errEl.classList.remove("is-visible");
        }
      }
      if (input && input.classList) {
        if (errors[name]) input.classList.add("is-invalid");
        else input.classList.remove("is-invalid");
      }
    }
  }

  global.BookingValidate = {
    digitsOnly: digitsOnly,
    isValidNationalID: isValidNationalID,
    isPersianName: isPersianName,
    isValidMobile: isValidMobile,
    validatePatientForm: validatePatientForm,
    showFieldErrors: showFieldErrors,
    normalizeName: normalizeName,
  };
})(typeof window !== "undefined" ? window : this);
