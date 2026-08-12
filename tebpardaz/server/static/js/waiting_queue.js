// waiting_queue.js — بروزرسانی زنده وضعیت صف انتظار بیمار (هر ۵ ثانیه از کش سرور).
(function () {
  var root = document.getElementById("waiting-queue-live");
  if (!root) return;

  // مسیر کامل path-based؛ مثلاً /123/0012345678/ws/waiting-queue
  var wsPath = root.getAttribute("data-ws-path") || "";
  if (!wsPath) return;

  var patientEl = document.getElementById("wq-patient-name");
  var doctorEl = document.getElementById("wq-doctor-name");
  var timeEl = document.getElementById("wq-visit-time");
  var aheadEl = document.getElementById("wq-ahead-count");
  var hintEl = document.getElementById("wq-ahead-hint");
  var msgEl = document.getElementById("wq-status-msg");

  var proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  var wsURL = proto + "//" + window.location.host + wsPath;

  var ws;
  var reconnectDelay = 3000;

  function setMessage(text) {
    if (msgEl) msgEl.textContent = text || "";
  }

  function updateUI(data) {
    if (!data) return;
    if (data.patient_name && patientEl) patientEl.textContent = data.patient_name;
    if (data.doctor_name && doctorEl) doctorEl.textContent = data.doctor_name;
    if (data.visit_time && timeEl) timeEl.textContent = data.visit_time;
    if (typeof data.ahead_count === "number" && aheadEl) {
      aheadEl.textContent = String(data.ahead_count);
    }
    if (hintEl) {
      if (data.ahead_count === 0) {
        hintEl.textContent = "نوبت شماست! لطفاً به اتاق پزشک بروید.";
      } else {
        hintEl.textContent = "لطفاً در سالن منتظر بمانید.";
      }
    }
    if (data.message) {
      setMessage(data.message);
    } else if (data.found) {
      setMessage("آخرین بروزرسانی: همین الان");
    }
  }

  function connect() {
    try {
      ws = new WebSocket(wsURL);
    } catch (e) {
      setMessage("اتصال برقرار نشد؛ دوباره تلاش می‌شود…");
      setTimeout(connect, reconnectDelay);
      return;
    }

    ws.onopen = function () {
      setMessage("متصل شد؛ در حال دریافت اطلاعات…");
    };

    ws.onmessage = function (ev) {
      try {
        var data = JSON.parse(ev.data);
        updateUI(data);
      } catch (e) {
        /* نادیده */
      }
    };

    ws.onclose = function () {
      setMessage("اتصال قطع شد؛ دوباره وصل می‌شویم…");
      setTimeout(connect, reconnectDelay);
    };

    ws.onerror = function () {
      setMessage("خطا در ارتباط");
    };
  }

  connect();
})();
