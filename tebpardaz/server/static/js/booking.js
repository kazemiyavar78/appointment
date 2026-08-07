(function () {
  "use strict";

  var STEPS = [1, 2, 3, 4];

  function $(id) {
    return document.getElementById(id);
  }

  function resetSteps() {
    STEPS.forEach(function (n) {
      var li = document.querySelector('.booking-step[data-step="' + n + '"]');
      if (!li) return;
      li.classList.remove("is-loading", "is-done", "is-error");
      var icon = li.querySelector(".booking-step-icon");
      if (icon) {
        icon.textContent = String(n);
        icon.className =
          "booking-step-icon flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-gray-300 text-xs text-ink-muted";
      }
    });
  }

  function setStep(step, status) {
    var li = document.querySelector('.booking-step[data-step="' + step + '"]');
    if (!li) return;
    li.classList.remove("is-loading", "is-done", "is-error");
    var icon = li.querySelector(".booking-step-icon");
    if (!icon) return;

    if (status === "loading") {
      li.classList.add("is-loading");
      icon.className =
        "booking-step-icon flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-brand text-xs text-brand";
      icon.innerHTML =
        '<span class="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-brand border-t-transparent"></span>';
    } else if (status === "done") {
      li.classList.add("is-done");
      icon.className =
        "booking-step-icon flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-emerald-500 text-xs text-white";
      icon.innerHTML =
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4"><path fill-rule="evenodd" d="M16.704 4.153a.75.75 0 01.143 1.052l-8 10.5a.75.75 0 01-1.127.075l-4.5-4.5a.75.75 0 011.06-1.06l3.894 3.893 7.48-9.817a.75.75 0 011.05-.143z" clip-rule="evenodd"/></svg>';
    } else if (status === "error") {
      li.classList.add("is-error");
      icon.className =
        "booking-step-icon flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-red-500 text-xs text-white";
      icon.textContent = "!";
    }
  }

  function showResult(ok, message) {
    var el = $("booking-result");
    if (!el) return;
    el.classList.remove("hidden", "bg-emerald-50", "text-emerald-800", "border", "border-emerald-200", "bg-red-50", "text-red-700", "border-red-200");
    el.classList.add("border");
    if (ok) {
      el.classList.add("bg-emerald-50", "text-emerald-800", "border-emerald-200");
    } else {
      el.classList.add("bg-red-50", "text-red-700", "border-red-200");
    }
    el.textContent = message || (ok ? "نوبت ثبت شد" : "خطا در ثبت نوبت");
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
    };
  }

  function init() {
    var root = $("booking-root");
    var form = $("booking-form");
    var progress = $("booking-progress");
    var submitBtn = $("booking-submit");
    if (!root || !form) return;

    var path = root.getAttribute("data-ws-url") || "";
    var busy = false;

    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      if (busy) return;
      if (!form.reportValidity()) return;

      busy = true;
      if (submitBtn) submitBtn.disabled = true;
      if (progress) progress.classList.remove("hidden");
      resetSteps();
      var resultEl = $("booking-result");
      if (resultEl) {
        resultEl.classList.add("hidden");
        resultEl.textContent = "";
      }

      var socket;
      try {
        socket = new WebSocket(wsURL(path));
      } catch (e) {
        showResult(false, "امکان برقراری ارتباط وجود ندارد");
        busy = false;
        if (submitBtn) submitBtn.disabled = false;
        return;
      }

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
        if (data.step) {
          setStep(data.step, data.status || "loading");
        }
        if (data.done) {
          var ok = !!data.ok;
          if (!ok) {
            document.querySelectorAll(".booking-step.is-loading").forEach(function (li) {
              var step = parseInt(li.getAttribute("data-step"), 10);
              if (step) setStep(step, "error");
            });
          }
          showResult(ok, data.message || "");
          busy = false;
          if (submitBtn && !ok) submitBtn.disabled = false;
          if (ok && submitBtn) submitBtn.disabled = true;
          try {
            socket.close();
          } catch (e) {}
        }
      });

      socket.addEventListener("error", function () {
        showResult(false, "خطا در ارتباط با سرور");
        busy = false;
        if (submitBtn) submitBtn.disabled = false;
      });

      socket.addEventListener("close", function () {
        if (busy) {
          showResult(false, "ارتباط قطع شد");
          busy = false;
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
