import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../modules/core/presentation/assets/js/alpine.js", import.meta.url), "utf8");
const start = source.indexOf("let dateFns =");
const factorySource = source.slice(start, source.indexOf("\n});", start) + 4);

function weekAt(instant, timezone, factor = 0) {
  const previousTimezone = process.env.TZ;
  process.env.TZ = timezone;
  try {
    class FixedDate extends Date {
      constructor(...args) {
        super(...(args.length ? args : [instant]));
      }
      static now() { return Date.parse(instant); }
    }
    const factory = vm.runInNewContext(`${factorySource}; dateFns`, {
      Date: FixedDate,
      Intl,
      document: { documentElement: { lang: "ru" } },
    });
    const dates = factory();
    return [dates.startOfWeek(factor), dates.endOfWeek(factor)];
  } finally {
    if (previousTimezone === undefined) delete process.env.TZ;
    else process.env.TZ = previousTimezone;
  }
}

for (const scenario of [
  { name: "Sunday stays in the current Monday-based week", now: "2026-10-25T07:00:00Z", from: "2026-10-18T19:00:00.000Z", to: "2026-10-25T19:00:00.000Z" },
  { name: "previous week on Sunday is exactly one week earlier", now: "2026-10-25T07:00:00Z", factor: 1, from: "2026-10-11T19:00:00.000Z", to: "2026-10-18T19:00:00.000Z" },
  { name: "Monday end crosses October once", now: "2026-10-26T07:00:00Z", from: "2026-10-25T19:00:00.000Z", to: "2026-11-01T19:00:00.000Z" },
  { name: "first day of month starts in previous month", now: "2026-09-01T07:00:00Z", from: "2026-08-30T19:00:00.000Z", to: "2026-09-06T19:00:00.000Z" },
  { name: "Sunday at March boundary remains in February week", now: "2026-03-01T07:00:00Z", from: "2026-02-22T19:00:00.000Z", to: "2026-03-01T19:00:00.000Z" },
  { name: "week spans year boundary", now: "2027-01-01T07:00:00Z", from: "2026-12-27T19:00:00.000Z", to: "2027-01-03T19:00:00.000Z" },
  { name: "spring DST uses seven local calendar days", now: "2026-03-29T10:00:00Z", timezone: "Europe/Berlin", from: "2026-03-22T23:00:00.000Z", to: "2026-03-29T22:00:00.000Z" },
  { name: "autumn DST uses seven local calendar days", now: "2026-10-25T11:00:00Z", timezone: "Europe/Berlin", from: "2026-10-18T22:00:00.000Z", to: "2026-10-25T23:00:00.000Z" },
]) {
  test(scenario.name, () => {
    assert.deepEqual(weekAt(scenario.now, scenario.timezone || "Asia/Tashkent", scenario.factor), [scenario.from, scenario.to]);
  });
}
