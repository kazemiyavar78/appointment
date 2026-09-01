// waiting_queue.js — بروزرسانی زنده وضعیت صف انتظار بیمار + لرزش/صدا هنگام تغییر تعداد.
(function () {
  var root = document.getElementById("waiting-queue-live");
  if (!root) return;

  var wsPath = root.getAttribute("data-ws-path") || "";
  if (!wsPath) return;

  var patientEl = document.getElementById("wq-patient-name");
  var doctorEl = document.getElementById("wq-doctor-name");
  var timeEl = document.getElementById("wq-visit-time");
  var aheadEl = document.getElementById("wq-ahead-count");
  var hintEl = document.getElementById("wq-ahead-hint");
  var msgEl = document.getElementById("wq-status-msg");
  var lastUpdatedEl = document.getElementById("wq-last-updated");
  var aheadPanel = root.querySelector(".wq-ahead-panel");

  var lastAhead =
    parseInt(root.getAttribute("data-ahead-count") || "", 10);
  if (isNaN(lastAhead)) lastAhead = null;

  var initialUpdatedAt = parseInt(
    root.getAttribute("data-updated-at") || "0",
    10
  );
  if (initialUpdatedAt > 0) {
    setLastUpdated(initialUpdatedAt);
  }

  var audioCtx = null;
  var audioUnlocked = false;
  var reconnectDelay = 3000;
  var ws;
  var proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  var wsURL = proto + "//" + window.location.host + wsPath;

  function setMessage(text) {
    if (msgEl) msgEl.textContent = text || "";
  }

  function toFaDigits(str) {
    return String(str).replace(/\d/g, function (d) {
      return "۰۱۲۳۴۵۶۷۸۹"[d];
    });
  }

  function pad2(n) {
    return n < 10 ? "0" + n : String(n);
  }

  function setLastUpdated(unixSec) {
    if (!lastUpdatedEl) return;
    var sec = typeof unixSec === "number" ? unixSec : 0;
    var d = sec > 0 ? new Date(sec * 1000) : new Date();
    if (isNaN(d.getTime())) d = new Date();
    var timeStr =
      pad2(d.getHours()) + ":" + pad2(d.getMinutes()) + ":" + pad2(d.getSeconds());
    lastUpdatedEl.textContent =
      "آخرین بروزرسانی: " + toFaDigits(timeStr);
  }

  function unlockAudio() {
    if (audioUnlocked) return;
    try {
      var Ctx = window.AudioContext || window.webkitAudioContext;
      if (!Ctx) return;
      if (!audioCtx) audioCtx = new Ctx();
      if (audioCtx.state === "suspended") {
        audioCtx.resume();
      }
      audioUnlocked = true;
    } catch (e) {
      /* نادیده */
    }
  }

  function playTone(freq, durationMs, type) {
    try {
      unlockAudio();
      if (!audioCtx) return;
      var osc = audioCtx.createOscillator();
      var gain = audioCtx.createGain();
      osc.type = type || "sine";
      osc.frequency.value = freq;
      gain.gain.value = 0.0001;
      osc.connect(gain);
      gain.connect(audioCtx.destination);
      var now = audioCtx.currentTime;
      gain.gain.exponentialRampToValueAtTime(0.18, now + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.0001, now + durationMs / 1000);
      osc.start(now);
      osc.stop(now + durationMs / 1000 + 0.02);
    } catch (e) {
      /* نادیده */
    }
  }

  function vibrate(pattern) {
    try {
      if (navigator.vibrate) navigator.vibrate(pattern);
    } catch (e) {
      /* نادیده */
    }
  }

  function pulseAheadUI(isYourTurn) {
    if (aheadEl) {
      aheadEl.classList.remove("wq-pulse");
      // force reflow so animation restarts
      void aheadEl.offsetWidth;
      aheadEl.classList.add("wq-pulse");
    }
    if (aheadPanel) {
      aheadPanel.classList.remove("wq-shake", "wq-your-turn");
      void aheadPanel.offsetWidth;
      aheadPanel.classList.add("wq-shake");
      if (isYourTurn) aheadPanel.classList.add("wq-your-turn");
    }
  }

  function notifyAheadChange(prev, next) {
    if (prev === null || prev === next) return;
    var decreased = next < prev;
    var yourTurn = next === 0;
    if (!decreased && !yourTurn) return;

    pulseAheadUI(yourTurn);

    if (yourTurn) {
      vibrate([200, 80, 200, 80, 400]);
      playTone(660, 160, "triangle");
      setTimeout(function () {
        playTone(880, 220, "triangle");
      }, 180);
    } else if (decreased) {
      vibrate([80, 40, 120]);
      playTone(520, 120, "sine");
    }
  }

  function updateUI(data) {
    if (!data) return;
    if (data.patient_name && patientEl) patientEl.textContent = data.patient_name;
    if (data.doctor_name && doctorEl) doctorEl.textContent = data.doctor_name;
    if (data.visit_time && timeEl) timeEl.textContent = data.visit_time;

    if (typeof data.ahead_count === "number") {
      if (aheadEl) aheadEl.textContent = String(data.ahead_count);
      notifyAheadChange(lastAhead, data.ahead_count);
      lastAhead = data.ahead_count;
    }

    if (hintEl) {
      if (data.ahead_count === 0) {
        hintEl.textContent = "نوبت شماست! لطفاً به اتاق پزشک بروید.";
      } else {
        hintEl.textContent = "لطفاً در سالن منتظر بمانید.";
      }
    }

    if (typeof data.updated_at_unix === "number" && data.updated_at_unix > 0) {
      setLastUpdated(data.updated_at_unix);
    } else {
      setLastUpdated(0);
    }

    if (data.message) {
      setMessage(data.message);
    } else if (data.found) {
      setMessage("صف به‌روز است");
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

  // برخی مرورگرها فقط بعد از تعامل کاربر صدا را مجاز می‌دانند.
  document.addEventListener("pointerdown", unlockAudio, { once: true });
  document.addEventListener("keydown", unlockAudio, { once: true });

  connect();
})();
