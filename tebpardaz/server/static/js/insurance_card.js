/**
 * مدیریت کلیک و لمس روی کارت‌های بیمه برای نمایش توضیحات در موبایل و دسکتاپ
 */
document.addEventListener("DOMContentLoaded", function () {
  var cards = document.querySelectorAll(".insurance-card");
  if (!cards.length) return;

  // بستن کارت‌های باز با کلیک در هر جای صفحه بیرون از کارت
  document.addEventListener("click", function (e) {
    cards.forEach(function (card) {
      if (!card.contains(e.target)) {
        card.classList.remove("is-active");
      }
    });
  });

  // تاگل کردن وضعیت کارت با کلیک روی آن
  cards.forEach(function (card) {
    card.addEventListener("click", function (e) {
      var wasActive = card.classList.contains("is-active");
      cards.forEach(function (c) {
        c.classList.remove("is-active");
      });
      if (!wasActive) {
        card.classList.add("is-active");
      }
    });

    // پشتیبانی از کلیدهای کیبورد (Enter و Space)
    card.addEventListener("keydown", function (e) {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        var wasActive = card.classList.contains("is-active");
        cards.forEach(function (c) {
          c.classList.remove("is-active");
        });
        if (!wasActive) {
          card.classList.add("is-active");
        }
      }
    });
  });
});
