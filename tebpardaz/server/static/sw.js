// سرویس‌ورکر صفحه وضعیت نوبت: اعلان پس‌زمینه را نشان می‌دهد و بقیه درخواست‌ها را دست نمی‌زند.

self.addEventListener("install", function () {
  self.skipWaiting();
});

self.addEventListener("activate", function (event) {
  event.waitUntil(self.clients.claim());
});

// گوش دادن خالی به fetch تا صفحه قابل نصب بماند، بدون اینکه پاسخ شبکه عوض شود.
self.addEventListener("fetch", function () {});

self.addEventListener("push", function (event) {
  var data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch (e) {
    data = { body: event.data ? event.data.text() : "" };
  }
  var title = data.title || "وضعیت نوبت";
  var options = {
    body: data.body || "",
    tag: data.tag || "waiting-queue",
    renotify: true,
    vibrate: data.vibrate || [180, 80, 180],
    dir: "rtl",
    lang: "fa",
    data: { url: data.url || "/waiting-queue" },
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", function (event) {
  event.notification.close();
  var target = "/waiting-queue";
  if (event.notification.data && event.notification.data.url) {
    target = event.notification.data.url;
  }
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then(function (windows) {
      for (var i = 0; i < windows.length; i++) {
        if (windows[i].url.indexOf(target) !== -1 && "focus" in windows[i]) {
          return windows[i].focus();
        }
      }
      if (self.clients.openWindow) {
        return self.clients.openWindow(target);
      }
    })
  );
});
