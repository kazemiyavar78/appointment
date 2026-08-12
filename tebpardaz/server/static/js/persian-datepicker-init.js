/**
 * Initializes .persian-datepicker-input fields with babakhani persian-datepicker.
 * Optional data attributes:
 *   data-min-age / data-max-age  — birth date (years)
 *   data-min-days / data-max-days — relative day window from today
 */
(function () {
  "use strict";

  function parseIntAttr(el, name, fallback) {
    var raw = el.getAttribute(name);
    if (raw == null || raw === "") return fallback;
    var n = parseInt(raw, 10);
    return isNaN(n) ? fallback : n;
  }

  function hasAttr(el, name) {
    var raw = el.getAttribute(name);
    return raw != null && raw !== "";
  }

  function buildOptions(el) {
    var opts = {
      format: "YYYY/MM/DD",
      autoClose: true,
      initialValue: false,
      observer: true,
      calendar: {
        persian: {
          locale: "fa",
        },
      },
      toolbox: {
        todayButton: { enabled: false },
        calendarSwitch: { enabled: false },
      },
    };

    if (hasAttr(el, "data-min-age") || hasAttr(el, "data-max-age")) {
      var minAge = parseIntAttr(el, "data-min-age", 1);
      var maxAge = parseIntAttr(el, "data-max-age", 120);
      if (minAge < 0) minAge = 0;
      if (maxAge < minAge) maxAge = minAge;
      opts.maxDate = new persianDate().subtract("year", minAge).valueOf();
      opts.minDate = new persianDate().subtract("year", maxAge).valueOf();
      opts.toolbox.todayButton.enabled = false;
    } else if (hasAttr(el, "data-min-days") || hasAttr(el, "data-max-days")) {
      var minDays = parseIntAttr(el, "data-min-days", 0);
      var maxDays = parseIntAttr(el, "data-max-days", 15);
      opts.minDate = new persianDate().add("day", minDays).valueOf();
      opts.maxDate = new persianDate().add("day", maxDays).valueOf();
      opts.toolbox.todayButton.enabled = true;
    }

    return opts;
  }

  function initPersianDatepickers(root) {
    if (typeof window.jQuery === "undefined" || typeof window.persianDate === "undefined") {
      return;
    }
    var $ = window.jQuery;
    var scope = root ? $(root) : $(document);
    scope.find(".persian-datepicker-input").each(function () {
      var el = this;
      if ($(el).data("pd-initialized")) return;
      $(el).persianDatepicker(buildOptions(el));
      $(el).data("pd-initialized", true);
    });
  }

  window.initPersianDatepickers = initPersianDatepickers;

  function onReady(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  onReady(function () {
    initPersianDatepickers(document);
  });
})();
