// بروزرسانی زنده وضعیت نوبت، حلقه پیشرفت، صدا/لرزش و ثبت Web Push.
(function () {
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(function () {});
  }

  var root = document.getElementById("waiting-queue-live");
  if (!root) return;

  var wsPath = root.getAttribute("data-ws-path") || "";
  var pushPath = root.getAttribute("data-push-path") || "";
  var vapidKey = root.getAttribute("data-vapid-key") || "";
  var csrfToken = root.getAttribute("data-csrf-token") || "";

  var patientEl = document.getElementById("wq-patient-name");
  var doctorEl = document.getElementById("wq-doctor-name");
  var timeEl = document.getElementById("wq-visit-time");
  var aheadEl = document.getElementById("wq-ahead-count");
  var ringLabel = document.getElementById("wq-ring-label");
  var ringFg = document.getElementById("wq-ring-fg");
  var banner = document.getElementById("wq-status-banner");
  var connDot = document.getElementById("wq-conn-dot");
  var connText = document.getElementById("wq-conn-text");
  var confirmBtn = document.getElementById("wq-confirm");
  var soundToggle = document.getElementById("wq-sound-toggle");
  var pushToggle = document.getElementById("wq-push-toggle");
  var iosGuide = document.getElementById("wq-ios-guide");
  var pushMsg = document.getElementById("wq-push-msg");

  var RING_CIRCUMFERENCE = 703.7;
  var RING_MAX = 20;
  var STATES = ["calm", "approaching", "urgent", "now"];
  var STATE_TEXT = {
    calm: "می‌توانید در محیط دیگری منتظر بمانید، به‌موقع خبر می‌دهیم",
    approaching: "نوبت شما نزدیک می‌شود — بهتر است در دسترس باشید",
    urgent: "نوبت شما خیلی نزدیک است، لطفاً به مطب مراجعه کنید",
    now: "نوبت شماست",
  };

  var lastAhead = parseInt(root.getAttribute("data-ahead-count") || "", 10);
  if (isNaN(lastAhead)) lastAhead = null;
  var soundEnabled = false;
  var audioCtx = null;
  var lastUpdateAt = Date.now();
  var reconnectDelay = 3000;
  var ws;
  var proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  var wsURL = wsPath ? proto + "//" + window.location.host + wsPath : "";

  function faDigits(value) {
    return String(value).replace(/\d/g, function (d) {
      return "۰۱۲۳۴۵۶۷۸۹"[d];
    });
  }

  // یک نفر هم فوری است: ۰ الان، ۱ تا ۳ فوری، ۴ تا ۱۰ نزدیک، بیشتر از ۱۰ آرام.
  function stateFor(n) {
    if (n <= 0) return "now";
    if (n <= 3) return "urgent";
    if (n <= 10) return "approaching";
    return "calm";
  }

  function setState(name) {
    for (var i = 0; i < STATES.length; i++) {
      document.body.classList.remove("wq-state-" + STATES[i]);
      root.classList.remove("wq-state-" + STATES[i]);
    }
    document.body.classList.add("wq-state-" + name);
    root.classList.add("wq-state-" + name);
    if (banner && STATE_TEXT[name]) banner.textContent = STATE_TEXT[name];
  }

  function paintRing(n) {
    var clamped = Math.max(0, Math.min(n, RING_MAX));
    if (ringFg) {
      ringFg.style.strokeDashoffset = (RING_CIRCUMFERENCE * (clamped / RING_MAX)).toFixed(1);
    }
    if (aheadEl) aheadEl.textContent = n <= 0 ? "حالا" : faDigits(n);
    if (ringLabel) ringLabel.textContent = n <= 0 ? "" : "نفر جلوتر از شما";
    setState(stateFor(n));
  }

  function unlockAudio() {
    try {
      var Ctx = window.AudioContext || window.webkitAudioContext;
      if (!Ctx) return;
      if (!audioCtx) audioCtx = new Ctx();
      if (audioCtx.state === "suspended") audioCtx.resume();
    } catch (e) {}
  }

  function playTone(freq, start, duration) {
    if (!audioCtx) return;
    var osc = audioCtx.createOscillator();
    var gain = audioCtx.createGain();
    osc.type = "sine";
    osc.frequency.value = freq;
    gain.gain.setValueAtTime(0.001, audioCtx.currentTime + start);
    gain.gain.linearRampToValueAtTime(0.25, audioCtx.currentTime + start + 0.02);
    gain.gain.exponentialRampToValueAtTime(0.001, audioCtx.currentTime + start + duration);
    osc.connect(gain);
    gain.connect(audioCtx.destination);
    osc.start(audioCtx.currentTime + start);
    osc.stop(audioCtx.currentTime + start + duration + 0.02);
  }

  // هر کم شدن عدد، اگر صدا روشن باشد، بوق و لرزش دارد. رسیدن به صفر طولانی‌تر است.
  function notifyDecrease(prev, next) {
    if (!soundEnabled || prev === null || next >= prev) return;
    var strong = next === 0;
    try {
      unlockAudio();
      if (strong) {
        playTone(880, 0, 0.18);
        playTone(880, 0.24, 0.18);
        playTone(1046, 0.48, 0.28);
      } else {
        playTone(880, 0, 0.18);
        playTone(880, 0.24, 0.18);
      }
    } catch (e) {}
    try {
      if (navigator.vibrate) {
        navigator.vibrate(strong ? [220, 90, 220, 90, 320] : [180, 80, 180]);
      }
    } catch (e2) {}
  }

  function applyAhead(n) {
    notifyDecrease(lastAhead, n);
    lastAhead = n;
    paintRing(n);
  }

  function updateUI(data) {
    if (!data) return;
    if (data.patient_name && patientEl) patientEl.textContent = data.patient_name;
    if (data.doctor_name && doctorEl) doctorEl.textContent = data.doctor_name;
    if (data.visit_time && timeEl) timeEl.textContent = data.visit_time;
    if (typeof data.ahead_count === "number") applyAhead(data.ahead_count);
    lastUpdateAt = Date.now();
    paintConnection();
  }

  function paintConnection() {
    var secs = Math.round((Date.now() - lastUpdateAt) / 1000);
    var open = ws && ws.readyState === 1;
    var closed = ws && (ws.readyState === 2 || ws.readyState === 3);
    var stale = open && secs > 15;
    if (connDot) {
      connDot.classList.toggle("wq-dot-off", closed || stale);
      connDot.classList.toggle("wq-dot-live", !closed && !stale);
    }
    if (!connText) return;
    if (closed) {
      connText.textContent = "اتصال قطع شد — در حال تلاش مجدد...";
      return;
    }
    if (!open) {
      connText.textContent = "در حال برقراری ارتباط…";
      return;
    }
    connText.textContent =
      secs < 2 ? "بروزرسانی زنده — همین الان" : "آخرین بروزرسانی: " + faDigits(secs) + " ثانیه پیش";
  }

  function connect() {
    if (!wsURL) return;
    try {
      ws = new WebSocket(wsURL);
    } catch (e) {
      setTimeout(connect, reconnectDelay);
      return;
    }
    ws.onopen = function () {
      lastUpdateAt = Date.now();
      paintConnection();
    };
    ws.onmessage = function (ev) {
      try {
        updateUI(JSON.parse(ev.data));
      } catch (e) {}
    };
    ws.onclose = function () {
      paintConnection();
      setTimeout(connect, reconnectDelay);
    };
    ws.onerror = function () {
      paintConnection();
    };
  }

  if (soundToggle) {
    soundToggle.addEventListener("click", function () {
      soundEnabled = !soundEnabled;
      soundToggle.setAttribute("data-on", soundEnabled ? "true" : "false");
      soundToggle.textContent = soundEnabled ? "صدا و لرزش فعال است" : "فعال‌سازی صدا و لرزش هشدار";
      if (soundEnabled) unlockAudio();
    });
  }

  if (confirmBtn) {
    confirmBtn.addEventListener("click", function () {
      confirmBtn.textContent = "ثبت شد";
      confirmBtn.disabled = true;
    });
  }

  function isIOS() {
    var ua = navigator.userAgent || "";
    return /iPad|iPhone|iPod/.test(ua) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
  }

  function isStandalone() {
    return window.navigator.standalone === true || window.matchMedia("(display-mode: standalone)").matches;
  }

  function setPushMessage(text) {
    if (pushMsg) pushMsg.textContent = text || "";
  }

  function urlBase64ToUint8Array(base64String) {
    var padding = "=".repeat((4 - (base64String.length % 4)) % 4);
    var base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
    var raw = window.atob(base64);
    var output = new Uint8Array(raw.length);
    for (var i = 0; i < raw.length; i++) output[i] = raw.charCodeAt(i);
  return output;
  }

  function subscribePush() {
    if (!pushPath || !vapidKey || !("serviceWorker" in navigator) || !("PushManager" in window)) {
      setPushMessage("اعلان پس‌زمینه در این مرورگر در دسترس نیست");
      return Promise.resolve();
    }
    return navigator.serviceWorker.ready.then(function (reg) {
      return reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(vapidKey),
      });
    }).then(function (sub) {
      var json = sub.toJSON();
      return fetch(pushPath, {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          csrf_token: csrfToken,
          endpoint: json.endpoint,
          keys: json.keys || {},
        }),
      });
    }).then(function (res) {
      return res.json().then(function (body) {
        if (!res.ok || !body.ok) {
          setPushMessage((body && body.message) || "فعال‌سازی اعلان انجام نشد");
          return;
        }
        if (pushToggle) {
          pushToggle.setAttribute("data-on", "true");
          pushToggle.textContent = "اعلان پس‌زمینه فعال است";
        }
        setPushMessage(body.message || "اعلان پس‌زمینه فعال شد");
      });
    }).catch(function () {
      setPushMessage("فعال‌سازی اعلان انجام نشد");
    });
  }

  function setupPush() {
    var iosNeedsInstall = isIOS() && !isStandalone();
    if (iosNeedsInstall) {
      if (pushToggle) pushToggle.hidden = true;
      if (iosGuide) iosGuide.hidden = false;
      return;
    }
    if (!vapidKey || !("Notification" in window) || !("PushManager" in window)) {
      if (pushToggle) pushToggle.hidden = true;
      setPushMessage("اعلان پس‌زمینه در این مرورگر در دسترس نیست");
      return;
    }
    if (Notification.permission === "granted") {
      subscribePush();
      return;
    }
    if (!pushToggle) return;
    pushToggle.addEventListener("click", function () {
      Notification.requestPermission().then(function (permission) {
        if (permission !== "granted") {
          setPushMessage("اجازه اعلان داده نشد");
          return;
        }
        subscribePush();
      });
    });
  }

  var initial = lastAhead === null ? 0 : lastAhead;
  paintRing(initial);
  lastAhead = initial;
  setupPush();
  paintConnection();
  setInterval(paintConnection, 1000);
  connect();
})();
