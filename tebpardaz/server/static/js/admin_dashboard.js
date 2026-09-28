(function () {
  var root = document.getElementById("dash-root");
  if (!root) return;

  var dataNode = document.getElementById("dashboard-data");
  var chartEl = document.getElementById("dash-chart");
  var tooltip = document.getElementById("dash-chart-tooltip");
  var loading = document.getElementById("dash-loading");
  var banner = document.getElementById("dash-banner");
  var modal = document.getElementById("dash-kpi-modal");
  var snapshot = readSnapshot();

  bindToolbar();
  bindKpis(root);
  if (snapshot) {
    drawChart(snapshot.daily || []);
  }

  function readSnapshot() {
    if (!dataNode) return null;
    try {
      return JSON.parse(dataNode.textContent || "{}");
    } catch (err) {
      showBanner("خواندن داده داشبورد ممکن نشد.");
      return null;
    }
  }

  function bindToolbar() {
    root.querySelectorAll(".dash-range").forEach(function (el) {
      el.addEventListener("click", function (e) {
        if (e.metaKey || e.ctrlKey || e.shiftKey) return;
        e.preventDefault();
        loadRange(el.getAttribute("data-range") || "today", el.getAttribute("href"));
      });
    });
    var refresh = document.getElementById("dash-refresh");
    if (refresh) {
      refresh.addEventListener("click", function () {
        loadRange(root.getAttribute("data-range") || "today", null, true);
      });
    }
  }

  function bindKpis(scope) {
    (scope || document).querySelectorAll("[data-kpi-id]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        openKpi(btn);
      });
    });
  }

  function openKpi(btn) {
    if (!modal || typeof modal.showModal !== "function") {
      var url = btn.getAttribute("data-detail-url");
      if (url) window.location.href = url;
      return;
    }
    document.getElementById("dash-modal-title").textContent = btn.getAttribute("data-label") || "";
    document.getElementById("dash-modal-value").textContent = btn.getAttribute("data-value-text") || "—";
    document.getElementById("dash-modal-prev").textContent =
      "مقدار دوره قبل: " + (btn.getAttribute("data-previous-text") || "—");
    document.getElementById("dash-modal-change").textContent = btn.getAttribute("data-change-text") || "";
    document.getElementById("dash-modal-hint").textContent = btn.getAttribute("data-hint") || "";
    var link = document.getElementById("dash-modal-link");
    link.href = btn.getAttribute("data-detail-url") || "/admin/bookings";
    modal.showModal();
  }

  function loadRange(rangeKey, fallbackHref, isRefresh) {
    var clinicId = root.getAttribute("data-clinic-id") || "0";
    var url = "/admin/dashboard/data?range=" + encodeURIComponent(rangeKey);
    if (clinicId && clinicId !== "0") {
      url += "&clinic_id=" + encodeURIComponent(clinicId);
    }
    setBusy(true);
    hideBanner();
    fetch(url, { headers: { Accept: "application/json" } })
      .then(function (res) {
        if (!res.ok) throw new Error("http");
        return res.json();
      })
      .then(function (payload) {
        if (!payload || !payload.ok || !payload.data) {
          throw new Error(payload && payload.message ? payload.message : "empty");
        }
        applySnapshot(payload.data, rangeKey);
        if (!isRefresh) {
          var next = "/admin?range=" + encodeURIComponent(rangeKey);
          if (clinicId && clinicId !== "0") next += "&clinic_id=" + encodeURIComponent(clinicId);
          history.replaceState({}, "", next);
        }
      })
      .catch(function () {
        showBanner("خطا در بروزرسانی داشبورد. در حال تلاش برای بارگذاری کامل صفحه...");
        if (fallbackHref) {
          window.location.href = fallbackHref;
        }
      })
      .finally(function () {
        setBusy(false);
      });
  }

  function applySnapshot(data, rangeKey) {
    snapshot = data;
    root.setAttribute("data-range", rangeKey || data.range || "today");
    var updated = document.getElementById("dash-updated");
    if (updated) updated.textContent = data.updated_text || "—";
    var rangeLabel = document.getElementById("dash-range-label");
    if (rangeLabel) rangeLabel.textContent = data.range_label || "";
    root.querySelectorAll(".dash-range").forEach(function (el) {
      el.classList.toggle("is-active", el.getAttribute("data-range") === (rangeKey || data.range));
    });
    renderKpis(data.kpis || []);
    renderList('[data-section="top-pages"]', data.top_pages, "در این بازه بازدیدی ثبت نشده است.");
    renderList('[data-section="top-doctors"]', data.top_doctors, "بازدید صفحه نوبت پزشک ثبت نشده است.");
    renderList('[data-section="top-specialties"]', data.top_specialties, "بازدید تخصص در این بازه ثبت نشده است.");
    renderList('[data-section="top-services"]', data.top_services, "خدمت متناظری از بازدید پزشکان به‌دست نیامد.");
    renderList('[data-section="open-doctors"]', (data.booking && data.booking.open_doctors) || [], "ظرفیت زنده‌ای از مراکز دریافت نشده است.");
    renderStatus(data.booking || {});
    renderHours(data.peak_hours || []);
    renderInsights(data.insights || []);
    drawChart(data.daily || []);
  }

  function renderKpis(kpis) {
    var wrap = root.querySelector(".dash-kpis");
    if (!wrap) return;
    wrap.innerHTML = kpis
      .map(function (k) {
        return (
          '<button type="button" class="dash-kpi ' +
          toneClass(k.sentiment) +
          '" data-kpi-id="' +
          esc(k.id) +
          '" data-detail-url="' +
          esc(k.detail_url) +
          '" data-label="' +
          esc(k.label) +
          '" data-value-text="' +
          esc(k.value_text) +
          '" data-previous-text="' +
          esc(k.previous_text) +
          '" data-change-text="' +
          esc(k.change_text) +
          '" data-hint="' +
          esc(k.hint) +
          '"><span class="dash-kpi-icon"><i class="' +
          esc(k.icon) +
          '"></i></span><span class="dash-kpi-label">' +
          esc(k.label) +
          '</span><strong class="dash-kpi-value">' +
          esc(k.value_text) +
          '</strong><span class="dash-kpi-change"><i class="' +
          arrowClass(k.direction) +
          '"></i><span>' +
          esc(k.change_text) +
          "</span></span></button>"
        );
      })
      .join("");
    bindKpis(wrap);
  }

  function renderList(sel, rows, emptyText) {
    var card = root.querySelector(sel);
    if (!card) return;
    var head = card.querySelector(".dash-card-head");
    var body = namedListHtml(rows, emptyText);
    card.innerHTML = (head ? head.outerHTML : "") + body;
  }

  function namedListHtml(rows, emptyText) {
    if (!rows || !rows.length) {
      return emptyHtml(emptyText);
    }
    return (
      '<ul class="dash-rank">' +
      rows
        .map(function (row) {
          var label =
            '<span class="dash-rank-label">' +
            esc(row.label) +
            (row.sub ? "<small>" + esc(row.sub) + "</small>" : "") +
            "</span><strong>" +
            esc(row.text) +
            "</strong>";
          if (row.url) {
            return "<li><a href=\"" + esc(row.url) + "\">" + label + "</a></li>";
          }
          return "<li>" + label + "</li>";
        })
        .join("") +
      "</ul>"
    );
  }

  function renderStatus(booking) {
    var el = document.getElementById("dash-status");
    if (!el) return;
    el.innerHTML =
      statusItem("موفق", booking.confirmed_text, "is-positive", "/admin/bookings?status=confirmed") +
      statusItem("ناموفق", booking.failed_text, "is-negative", "/admin/bookings?status=failed") +
      statusItem("لغوشده", booking.cancelled_text, "is-warn", "/admin/bookings?status=cancelled") +
      statusItem("در انتظار", booking.pending_text, "is-neutral", "/admin/bookings?status=pending");
  }

  function statusItem(label, value, tone, href) {
    return (
      '<a class="dash-status ' +
      tone +
      '" href="' +
      href +
      '"><span>' +
      label +
      "</span><strong>" +
      esc(value || "۰") +
      "</strong></a>"
    );
  }

  function renderHours(hours) {
    var el = document.getElementById("dash-hours");
    if (!el) return;
    if (!hours.length) {
      el.outerHTML = emptyHtml("توزیع ساعتی برای این بازه وجود ندارد.");
      return;
    }
    el.innerHTML = hours
      .map(function (h) {
        var cls = "dash-hour";
        if (!h.count) cls += " is-empty";
        else if (h.share >= 70) cls += " is-peak";
        var pct = !h.count ? 4 : Math.max(8, h.share || 0);
        return (
          '<div class="' +
          cls +
          '" title="' +
          esc(h.label) +
          " — " +
          esc(h.text) +
          '"><span class="dash-hour-bar" style="--h:' +
          pct +
          '"></span><span class="dash-hour-label">' +
          esc(h.label) +
          "</span></div>"
        );
      })
      .join("");
  }

  function renderInsights(lines) {
    var box = document.getElementById("dash-insights");
    var card = root.querySelector('[data-section="insights"]');
    if (!card) return;
    var head = card.querySelector(".dash-card-head");
    if (!lines.length) {
      card.innerHTML = (head ? head.outerHTML : "") + emptyHtml("خلاصه‌ای برای این بازه ساخته نشد.");
      return;
    }
    card.innerHTML =
      (head ? head.outerHTML : "") +
      '<ul class="dash-insight-list" id="dash-insights">' +
      lines.map(function (line) { return "<li>" + esc(line) + "</li>"; }).join("") +
      "</ul>";
  }

  function emptyHtml(message) {
    return (
      '<div class="dash-empty"><i class="fa-regular fa-folder-open"></i><p>' +
      esc(message) +
      "</p></div>"
    );
  }

  function drawChart(days) {
    if (!chartEl) return;
    var wrap = document.getElementById("dash-chart-wrap");
    if (!days.length) {
      if (wrap) {
        wrap.innerHTML = emptyHtml("برای این بازه روندی ثبت نشده است.");
      }
      return;
    }
    if (!document.getElementById("dash-chart")) {
      wrap.innerHTML =
        '<svg id="dash-chart" class="dash-chart" viewBox="0 0 640 240" role="img" aria-label="نمودار مقایسه‌ای بازدید و نوبت"></svg>' +
        '<div id="dash-chart-tooltip" class="dash-tooltip" hidden></div>' +
        '<div class="dash-legend"><span><i class="dash-dot is-visit"></i> بازدید</span><span><i class="dash-dot is-appt"></i> نوبت ثبت‌شده</span></div>';
      chartEl = document.getElementById("dash-chart");
      tooltip = document.getElementById("dash-chart-tooltip");
    }
    var w = 640;
    var h = 240;
    var pad = { t: 18, r: 18, b: 36, l: 40 };
    var visits = days.map(function (d) { return d.visits || 0; });
    var appts = days.map(function (d) { return d.appointments || 0; });
    var maxV = Math.max.apply(null, visits.concat([1]));
    var maxA = Math.max.apply(null, appts.concat([1]));
    var innerW = w - pad.l - pad.r;
    var innerH = h - pad.t - pad.b;
    var n = days.length;
    function x(i) {
      if (n === 1) return pad.l + innerW / 2;
      return pad.l + (i * innerW) / (n - 1);
    }
    function y(val, max) {
      return pad.t + innerH - (val / max) * innerH;
    }
    function polyline(values, max, color) {
      return values
        .map(function (v, i) {
          return x(i) + "," + y(v, max);
        })
        .join(" ");
    }
    var grid = "";
    for (var g = 0; g < 4; g++) {
      var gy = pad.t + (innerH * g) / 3;
      grid +=
        '<line x1="' +
        pad.l +
        '" y1="' +
        gy +
        '" x2="' +
        (w - pad.r) +
        '" y2="' +
        gy +
        '" stroke="rgba(255,255,255,0.08)" />';
    }
    var labels = days
      .map(function (d, i) {
        if (n > 10 && i % 3 !== 0 && i !== n - 1) return "";
        return (
          '<text x="' +
          x(i) +
          '" y="' +
          (h - 10) +
          '" text-anchor="middle" fill="#94a3b8" font-size="11">' +
          esc(d.label || "") +
          "</text>"
        );
      })
      .join("");
    var hit = days
      .map(function (_, i) {
        return (
          '<rect data-i="' +
          i +
          '" x="' +
          (x(i) - 8) +
          '" y="' +
          pad.t +
          '" width="16" height="' +
          innerH +
          '" fill="transparent" />'
        );
      })
      .join("");
    chartEl.innerHTML =
      grid +
      '<polyline fill="none" stroke="#22d3ee" stroke-width="2.5" points="' +
      polyline(visits, maxV, "#22d3ee") +
      '" />' +
      '<polyline fill="none" stroke="#a78bfa" stroke-width="2.5" points="' +
      polyline(appts, maxA, "#a78bfa") +
      '" />' +
      labels +
      hit;
    chartEl.querySelectorAll("rect[data-i]").forEach(function (rect) {
      rect.addEventListener("mousemove", function (ev) {
        var i = parseInt(rect.getAttribute("data-i"), 10);
        var d = days[i];
        if (!d || !tooltip) return;
        tooltip.hidden = false;
        tooltip.innerHTML =
          "<div>" +
          esc(d.label) +
          "</div><div>بازدید: " +
          esc(d.visits_text) +
          "</div><div>نوبت: " +
          esc(d.appointments_text) +
          "</div>";
        var box = chartEl.getBoundingClientRect();
        tooltip.style.left = ev.clientX - box.left + 12 + "px";
        tooltip.style.top = ev.clientY - box.top + 8 + "px";
      });
      rect.addEventListener("mouseleave", function () {
        if (tooltip) tooltip.hidden = true;
      });
    });
  }

  function setBusy(on) {
    root.classList.toggle("is-busy", on);
    root.setAttribute("data-loading", on ? "1" : "0");
    if (loading) loading.hidden = !on;
  }

  function showBanner(msg) {
    if (!banner) return;
    banner.hidden = false;
    banner.classList.add("is-error");
    banner.textContent = msg;
  }

  function hideBanner() {
    if (!banner) return;
    banner.hidden = true;
    banner.textContent = "";
  }

  function toneClass(s) {
    if (s === "positive") return "is-positive";
    if (s === "negative") return "is-negative";
    return "is-neutral";
  }

  function arrowClass(d) {
    if (d === "up") return "fa-solid fa-arrow-trend-up";
    if (d === "down") return "fa-solid fa-arrow-trend-down";
    return "fa-solid fa-minus";
  }

  function esc(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }
})();
