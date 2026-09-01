document.addEventListener("DOMContentLoaded", function () {
  var cards = document.querySelectorAll(".sp-card");
  if (!cards.length) return;

  var canHover = window.matchMedia("(hover: hover) and (pointer: fine)").matches;

  document.addEventListener("click", function (e) {
    if (canHover) return;
    cards.forEach(function (card) {
      if (!card.contains(e.target)) {
        card.classList.remove("is-flipped");
      }
    });
  });

  cards.forEach(function (card) {
    card.addEventListener("click", function (e) {
      if (canHover || e.target.closest("a")) return;
      card.classList.toggle("is-flipped");
    });
  });
});
