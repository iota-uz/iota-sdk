import "./alpine.lib.min.js";

document.addEventListener("alpine:init", () => {
  Alpine.data("relativeformat", () => ({
    format(dateStr = new Date().toISOString(), locale = "ru") {
      let date = new Date(dateStr);
      let timeMs = date.getTime();
      let delta = Math.round((timeMs - Date.now()) / 1000);
      let cutoffs = [60, 3600, 86400, 86400 * 7, 86400 * 30, 86400 * 365, Infinity];
      let units = ["second", "minute", "hour", "day", "week", "month", "year"];
      let unitIdx = cutoffs.findIndex((cutoff) => cutoff > Math.abs(delta));
      let divisor = unitIdx ? cutoffs[unitIdx - 1] : 1;
      let rtf = new Intl.RelativeTimeFormat(locale, {numeric: "auto"});
      return rtf.format(Math.floor(delta / divisor), units[unitIdx]);
    }
  }));
});