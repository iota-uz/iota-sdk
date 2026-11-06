import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../modules/core/presentation/assets/js/alpine.js", import.meta.url), "utf8");
const start = source.indexOf("let fbPanel =");
const factorySource = source.slice(start, source.indexOf("\n});", start) + 4);
const factory = vm.runInNewContext(`${factorySource}; fbPanel`);

function panel({ checkboxes = [], selected = [] }) {
  const dispatched = [];
  const instance = factory({ field: "status", type: "reference", op: "is", chip: 3 });
  instance.$root = {
    querySelectorAll: (selector) => (selector === 'input[type="checkbox"][data-fb-value]' ? checkboxes : []),
    querySelector: (selector) => (selector === "select[multiple]" ? { selectedOptions: selected } : null),
  };
  instance.$dispatch = (name, detail) => dispatched.push({ name, detail });
  return { instance, dispatched };
}

test("checkbox list applies only the checked values", () => {
  const { instance, dispatched } = panel({
    checkboxes: [
      { value: "1", checked: true },
      { value: "2", checked: false },
      { value: "3", checked: true },
    ],
  });
  instance.apply();
  assert.equal(dispatched.length, 1);
  assert.equal(dispatched[0].name, "fb-apply");
  assert.deepEqual(JSON.parse(JSON.stringify(dispatched[0].detail)), {
    field: "status",
    op: "is",
    values: ["1", "3"],
    chip: 3,
  });
});

test("checkbox list with nothing checked applies nothing", () => {
  const { instance, dispatched } = panel({ checkboxes: [{ value: "1", checked: false }] });
  instance.apply();
  assert.deepEqual(dispatched, []);
});

test("combobox selection still applies and skips group headers", () => {
  const { instance, dispatched } = panel({
    selected: [{ value: "__group:Mandatory" }, { value: "EEIU" }],
  });
  instance.apply();
  assert.deepEqual(Array.from(dispatched[0].detail.values), ["EEIU"]);
});
