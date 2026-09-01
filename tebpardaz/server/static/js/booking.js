/**
 * ویزارد چندمرحله‌ای رزرو: نوبت → اطلاعات → OTP → مودال نتیجه
 */
(function () {
  "use strict";

  var V = window.BookingValidate;
  var STEPS = [1, 2, 3, 4];
  var OTP_COOLDOWN = 120;
  var SESSION_KEY = "bk_otp_session";

  function $(id) {
    return document.getElementById(id);
  }

  /** به‌روزرسانی استپر بالای صفحه */
  function setStepper(step) {
    var items = document.querySelectorAll(".bk-stepper__item");
    var i;
    for (i = 0; i < items.length; i++) {
      var n = parseInt(items[i].getAttribute("data-step"), 10);
      items[i].classList.remove("is-active", "is-done");
      if (n < step) items[i].classList.add("is-done");
      if (n === step) items[i].classList.add("is-active");
    }
  }

  /** نمایش پنل مرحله (تب‌مانند) */
  function showPanel(step) {
    var slots = $("booking-step-slots");
    var info = $("booking-step-info");
    var otp = $("booking-otp-panel");
		if (slots) {
			slots.classList.toggle("bk-hidden", step !== 1);
      slots.setAttribute("aria-hidden", step !== 1 ? "true" : "false");
    }
    if (info) {
      info.classList.toggle("bk-hidden", step !== 2);
      info.setAttribute("aria-hidden", step !== 2 ? "true" : "false");
    }
    if (otp) {
      otp.classList.toggle("bk-hidden", step !== 3);
      otp.setAttribute("aria-hidden", step !== 3 ? "true" : "false");
    }
    setStepper(step);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  /** وضعیت پیشرفت WebSocket (مخفی) */
  function setProgressStep(step, status) {
    var li = document.querySelector('.booking-step[data-step="' + step + '"]');
    if (!li) return;
    var icon = li.querySelector(".booking-step-icon");
    if (!icon) return;
    if (status === "loading") {
      icon.textContent = "…";
    } else if (status === "done") {
      icon.textContent = "✓";
    } else if (status === "error") {
      icon.textContent = "!";
    }
  }

  /** باز کردن مودال نتیجه */
  function openModal(ok, message, extra) {
    var modal = $("booking-result-modal");
    var msgEl = $("bk-modal-message");
    var summary = $("bk-modal-summary");
    if (!modal) return;
    modal.classList.remove("bk-hidden", "is-error");
    modal.setAttribute("aria-hidden", "false");
    if (!ok) modal.classList.add("is-error");
    if (msgEl) msgEl.textContent = message || (ok ? "نوبت ثبت شد" : "خطا در ثبت");
    if (summary) {
      if (ok && extra) {
        summary.classList.remove("bk-hidden");
        if ($("bk-modal-slot")) $("bk-modal-slot").textContent = extra.slot || "";
        if ($("bk-modal-patient")) $("bk-modal-patient").textContent = extra.patient || "";
        if ($("bk-modal-tracking")) $("bk-modal-tracking").textContent = extra.tracking || "";
      } else {
        summary.classList.add("bk-hidden");
      }
    }
    var title = $("bk-modal-title");
    if (title) title.textContent = ok ? "نوبت با موفقیت ثبت شد" : "ثبت نوبت ناموفق";
  }

  function closeModal() {
    var modal = $("booking-result-modal");
    if (modal) {
      modal.classList.add("bk-hidden");
      modal.setAttribute("aria-hidden", "true");
    }
  }

  function wsURL(path) {
    var proto = location.protocol === "https:" ? "wss:" : "ws:";
    return proto + "//" + location.host + path;
  }

  function formPayload(form) {
    var fd = new FormData(form);
    return {
      csrf_token: fd.get("csrf_token") || "",
      slot_id: fd.get("slot_id") || "",
      first_name: fd.get("first_name") || "",
      last_name: fd.get("last_name") || "",
      national_id: fd.get("national_id") || "",
      mobile: fd.get("mobile") || "",
      birth_date: fd.get("birth_date") || "",
      sex: fd.get("sex") || "",
      otp_ticket: fd.get("otp_ticket") || "",
    };
  }

  function maskMobile(m) {
    m = V.digitsOnly(m);
    if (m.length < 7) return m;
    return m.slice(0, 4) + "***" + m.slice(-4);
  }

  /** ذخیره نشست OTP در sessionStorage */
  function saveSession(mobile, ticket, expiresAt) {
    try {
      sessionStorage.setItem(
        SESSION_KEY,
        JSON.stringify({ mobile: V.digitsOnly(mobile), ticket: ticket, expiresAt: expiresAt })
      );
    } catch (e) {}
  }

  function loadSession(mobile) {
    try {
      var raw = sessionStorage.getItem(SESSION_KEY);
      if (!raw) return null;
      var data = JSON.parse(raw);
      if (V.digitsOnly(mobile) !== data.mobile) return null;
      if (data.expiresAt && Date.now() / 1000 > data.expiresAt) return null;
      return data;
    } catch (e) {
      return null;
    }
  }

  /** راه‌اندازی جعبه‌های OTP */
  function initOTPBoxes(onComplete) {
    var boxes = document.querySelectorAll(".bk-otp__box");
    var hidden = $("otp_code");
    var i;

    function syncHidden() {
      var code = "";
      for (i = 0; i < boxes.length; i++) code += boxes[i].value || "";
      if (hidden) hidden.value = code;
      var verifyBtn = $("booking-otp-verify");
      if (verifyBtn) verifyBtn.disabled = code.length < 5;
      if (code.length === 5 && onComplete) onComplete(code);
    }

    for (i = 0; i < boxes.length; i++) {
      (function (box, idx) {
        box.addEventListener("input", function () {
          var v = V.digitsOnly(box.value).slice(-1);
          box.value = v;
          if (v && idx < boxes.length - 1) boxes[idx + 1].focus();
          syncHidden();
        });
        box.addEventListener("keydown", function (ev) {
          if (ev.key === "Backspace" && !box.value && idx > 0) {
            boxes[idx - 1].focus();
            boxes[idx - 1].value = "";
            syncHidden();
          }
        });
        box.addEventListener("paste", function (ev) {
          ev.preventDefault();
          var text = (ev.clipboardData || window.clipboardData).getData("text");
          text = V.digitsOnly(text).slice(0, 5);
          var j;
          for (j = 0; j < boxes.length; j++) boxes[j].value = text.charAt(j) || "";
          syncHidden();
          if (text.length >= 5) boxes[4].focus();
        });
      })(boxes[i], i);
    }
  }

  function init() {
    var root = $("booking-root");
    var form = $("booking-form");
    if (!root || !form || !V) return;

    var wsPath = root.getAttribute("data-ws-url") || "";
    var clinicPath = root.getAttribute("data-clinic-path") || "";
    var otpSendURL = root.getAttribute("data-otp-send-url") || "/otp/send";
    var otpVerifyURL = root.getAttribute("data-otp-verify-url") || "/otp/verify";
    var lookupURL = root.getAttribute("data-patient-lookup-url") || "/patient/lookup";
    var doctorName = root.getAttribute("data-doctor-name") || "";

    var toInfoBtn = $("booking-to-info");
    var toOtpBtn = $("booking-to-otp");
    var otpBackBtn = $("booking-otp-back");
    var otpVerifyBtn = $("booking-otp-verify");
    var otpResendBtn = $("booking-otp-resend");
    var otpTimerEl = $("bk-otp-timer");
    var otpStatusEl = $("booking-otp-status");
    var otpTicketInput = $("otp_ticket");
    var nationalInput = $("national_id");

    var busy = false;
    var cooldownTimer = null;
    var selectedSlotLabel = "";

    /** انتخاب نوبت */
    form.addEventListener("change", function (ev) {
      if (!ev.target || !ev.target.classList.contains("slot-radio")) return;
      var cards = document.querySelectorAll(".bk-slot-card");
      var k;
      for (k = 0; k < cards.length; k++) cards[k].classList.remove("is-selected");
      var card = ev.target.closest(".bk-slot-card");
      if (card) card.classList.add("is-selected");
      selectedSlotLabel = ev.target.getAttribute("data-slot-label") || "";
      if (toInfoBtn) toInfoBtn.disabled = false;
    });

    if (toInfoBtn) {
      toInfoBtn.addEventListener("click", function () {
        var checked = form.querySelector(".slot-radio:checked");
        if (!checked) {
          var hint = document.querySelector("#booking-step-slots .bk-panel__desc");
          if (hint) hint.textContent = "لطفاً یک نوبت انتخاب کنید";
          return;
        }
        showPanel(2);
        if (typeof window.initPersianDatepickers === "function") {
          window.initPersianDatepickers($("booking-step-info"));
        }
      });
    }

    /** autofill با blur کد ملی */
    if (nationalInput) {
      nationalInput.addEventListener("blur", function () {
        var nid = V.digitsOnly(nationalInput.value);
        if (!V.isValidNationalID(nid)) return;
        fetch(lookupURL + "?national_id=" + encodeURIComponent(nid), {
          credentials: "same-origin",
          headers: { Accept: "application/json" },
        })
          .then(function (r) {
            return r.json();
          })
          .then(function (data) {
            if (!data.found || !data.patient) return;
            var p = data.patient;
            if ($("first_name") && p.first_name) $("first_name").value = p.first_name;
            if ($("last_name") && p.last_name) $("last_name").value = p.last_name;
            if ($("mobile") && p.mobile) $("mobile").value = p.mobile;
            if ($("birth_date") && p.birth_date) $("birth_date").value = p.birth_date;
            if (p.sex) {
              var sexInput = form.querySelector('input[name="sex"][value="' + p.sex + '"]');
              if (sexInput) sexInput.checked = true;
            }
          })
          .catch(function () {});
      });
    }

    function setOtpStatus(msg, isError) {
      if (!otpStatusEl) return;
      otpStatusEl.textContent = msg || "";
      otpStatusEl.classList.remove("is-error", "is-ok");
      if (isError) otpStatusEl.classList.add("is-error");
      else if (msg) otpStatusEl.classList.add("is-ok");
    }

    function startCooldown(sec) {
      var left = sec || OTP_COOLDOWN;
      if (otpResendBtn) otpResendBtn.classList.add("bk-hidden");
      if (cooldownTimer) clearInterval(cooldownTimer);
      function tick() {
        if (left <= 0) {
          clearInterval(cooldownTimer);
          if (otpTimerEl) otpTimerEl.textContent = "";
          if (otpResendBtn) otpResendBtn.classList.remove("bk-hidden");
          return;
        }
        if (otpTimerEl) otpTimerEl.textContent = "ارسال مجدد تا " + left + " ثانیه";
        left -= 1;
      }
      tick();
      cooldownTimer = setInterval(tick, 1000);
    }

    function sendOTP(thenVerify) {
      var result = V.validatePatientForm(form);
      V.showFieldErrors(result.errors);
      if (!result.ok) return;

      var fd = new FormData(form);
      var mobile = V.digitsOnly(fd.get("mobile"));
      var existing = loadSession(mobile);
      if (existing && existing.ticket) {
        if (otpTicketInput) otpTicketInput.value = existing.ticket;
        showPanel(3);
        if ($("bk-otp-mobile-mask")) $("bk-otp-mobile-mask").textContent = maskMobile(mobile);
        setOtpStatus("موبایل قبلاً تایید شده است", false);
        if (thenVerify) submitBooking();
        return;
      }

      setOtpStatus("در حال ارسال کد…", false);
      fetch(otpSendURL, {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({
          csrf_token: fd.get("csrf_token"),
          mobile: mobile,
          first_name: fd.get("first_name"),
          last_name: fd.get("last_name"),
          national_id: fd.get("national_id"),
          clinic_path: clinicPath,
        }),
      })
        .then(function (res) {
          return res.json().then(function (data) {
            return { status: res.status, data: data };
          });
        })
        .then(function (r) {
          if (!r.data.ok) {
            setOtpStatus(r.data.message || "ارسال ناموفق", true);
            if (r.status === 429) startCooldown(r.data.retry_after_sec || 7200);
            return;
          }
          showPanel(3);
          if ($("bk-otp-mobile-mask")) $("bk-otp-mobile-mask").textContent = maskMobile(mobile);
          setOtpStatus(r.data.message || "کد ارسال شد", false);
          startCooldown(OTP_COOLDOWN);
          if (r.data.session_active && r.data.otp_ticket) {
            if (otpTicketInput) otpTicketInput.value = r.data.otp_ticket;
            submitBooking();
          } else {
            var boxes = document.querySelectorAll(".bk-otp__box");
            if (boxes[0]) boxes[0].focus();
          }
        })
        .catch(function () {
          setOtpStatus("خطا در ارتباط با سرور", true);
        });
    }

    if (toOtpBtn) {
      toOtpBtn.addEventListener("click", function () {
        sendOTP(false);
      });
    }

    if (otpBackBtn) {
      otpBackBtn.addEventListener("click", function () {
        showPanel(2);
      });
    }

    if (otpResendBtn) {
      otpResendBtn.addEventListener("click", function () {
        sendOTP(false);
      });
    }

    function verifyAndBook() {
      var fd = new FormData(form);
      var mobile = V.digitsOnly(fd.get("mobile"));
      var code = ($("otp_code") && $("otp_code").value) || "";
      if (code.length < 5) {
        setOtpStatus("کد ۵ رقمی را کامل وارد کنید", true);
        return;
      }
      if (otpVerifyBtn) {
        otpVerifyBtn.disabled = true;
        otpVerifyBtn.classList.add("is-loading");
      }
      setOtpStatus("در حال بررسی…", false);

      fetch(otpVerifyURL, {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({
          csrf_token: fd.get("csrf_token"),
          mobile: mobile,
          code: code,
        }),
      })
        .then(function (res) {
          return res.json().then(function (data) {
            return { data: data };
          });
        })
        .then(function (r) {
          if (otpVerifyBtn) {
            otpVerifyBtn.classList.remove("is-loading");
            otpVerifyBtn.disabled = false;
          }
          if (!r.data.ok || !r.data.otp_ticket) {
            setOtpStatus(r.data.message || "کد نادرست", true);
            return;
          }
          if (otpTicketInput) otpTicketInput.value = r.data.otp_ticket;
          saveSession(mobile, r.data.otp_ticket, r.data.expires_at);
          setOtpStatus("تایید شد؛ در حال ثبت نوبت…", false);
          submitBooking();
        })
        .catch(function () {
          if (otpVerifyBtn) {
            otpVerifyBtn.classList.remove("is-loading");
            otpVerifyBtn.disabled = false;
          }
          setOtpStatus("خطا در ارتباط", true);
        });
    }

    if (otpVerifyBtn) {
      otpVerifyBtn.addEventListener("click", verifyAndBook);
    }

    initOTPBoxes(function (code) {
      if (code.length === 5 && !busy) verifyAndBook();
    });

    /** ثبت نوبت از طریق WebSocket */
    function submitBooking() {
      if (busy) return;
      var result = V.validatePatientForm(form);
      if (!result.ok) {
        V.showFieldErrors(result.errors);
        showPanel(2);
        return;
      }
      if (!otpTicketInput || !otpTicketInput.value) {
        setOtpStatus("ابتدا کد تایید را وارد کنید", true);
        return;
      }

      busy = true;
      if (otpVerifyBtn) {
        otpVerifyBtn.disabled = true;
        otpVerifyBtn.classList.add("is-loading");
      }

      var socket;
      try {
        socket = new WebSocket(wsURL(wsPath));
      } catch (e) {
        openModal(false, "امکان برقراری ارتباط وجود ندارد");
        busy = false;
        return;
      }

      var fd = new FormData(form);
      var patientName = (fd.get("first_name") || "") + " " + (fd.get("last_name") || "");

      socket.addEventListener("open", function () {
        socket.send(JSON.stringify(formPayload(form)));
      });

      socket.addEventListener("message", function (msg) {
        var data;
        try {
          data = JSON.parse(msg.data);
        } catch (e) {
          return;
        }
        if (data.step) setProgressStep(data.step, data.status || "loading");
        if (data.done) {
          busy = false;
          if (otpVerifyBtn) {
            otpVerifyBtn.classList.remove("is-loading");
            otpVerifyBtn.disabled = false;
          }
          setStepper(4);
          openModal(!!data.ok, data.message, {
            slot: selectedSlotLabel || doctorName,
            patient: patientName.trim(),
            tracking: data.external_id || "",
          });
          try {
            socket.close();
          } catch (e) {}
        }
      });

      socket.addEventListener("error", function () {
        busy = false;
        openModal(false, "خطا در ارتباط با سرور");
      });

      socket.addEventListener("close", function () {
        if (busy) {
          busy = false;
          openModal(false, "ارتباط قطع شد");
        }
      });
    }

    if ($("bk-modal-close")) $("bk-modal-close").addEventListener("click", closeModal);
    if ($("bk-modal-backdrop")) $("bk-modal-backdrop").addEventListener("click", closeModal);
    if ($("bk-modal-home")) {
      $("bk-modal-home").addEventListener("click", function () {
        window.location.href = "/";
      });
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
