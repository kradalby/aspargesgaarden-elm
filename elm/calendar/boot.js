// Starts the booking calendar (Calendar.elm) on the months ui.BookingCalendar
// rendered. Elm draws off the page first and replaces the server's months
// only once it has read their data (its root carries data-booted): a visitor
// never gets less than the page without script.
document.querySelectorAll("[data-calendar]").forEach((node) => {
  let flags;
  try {
    flags = JSON.parse(node.dataset.calendar);
  } catch {
    return;
  }

  const holder = document.createElement("div");
  const mount = holder.appendChild(document.createElement("div"));
  const app = Elm.Calendar.init({ node: mount, flags });
  const drawn = holder.firstElementChild;
  if (!drawn || !drawn.hasAttribute("data-booted")) return;
  node.replaceWith(drawn);

  // en-CA writes YYYY-MM-DD.
  const oslo = new Intl.DateTimeFormat("en-CA", { timeZone: "Europe/Oslo" });
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) app.ports.today.send(oslo.format(new Date()));
  });

  // After Elm has drawn the answer.
  app.ports.answered.subscribe(() =>
    requestAnimationFrame(() =>
      document
        .getElementById("kalender-svar")
        ?.scrollIntoView({ block: "nearest" }),
    ),
  );
});
