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
    // The value select (multi or single) is found by excluding the operator
    // select, which a real panel also renders.
    querySelector: (selector) => (selector === "select:not([data-fb-op])" ? { selectedOptions: selected } : null),
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

test("single-value select applies its one selected value", () => {
  const { instance, dispatched } = panel({ selected: [{ value: "EEIU" }] });
  instance.apply();
  assert.equal(dispatched.length, 1);
  assert.deepEqual(Array.from(dispatched[0].detail.values), ["EEIU"]);
});

test("an empty value select applies nothing", () => {
  const { instance, dispatched } = panel({ selected: [{ value: "" }] });
  instance.apply();
  assert.deepEqual(dispatched, []);
});

function flagPanel(value, focused, op = "is") {
  const dispatched = [];
  const instance = factory({ field: "overdue", type: "bool", op, chip: 0, value });
  instance.$root = { ownerDocument: { activeElement: focused } };
  instance.$dispatch = (name, detail) => dispatched.push({ name, detail });
  return { instance, dispatched };
}

test("flag editor knows the current value and switches it", () => {
  const { instance, dispatched } = flagPanel("true");
  assert.equal(instance.value, "true");
  instance.applyBool("false");
  assert.equal(instance.value, "false");
  assert.deepEqual(JSON.parse(JSON.stringify(dispatched[0].detail)), { field: "overdue", op: "is", values: ["false"], chip: 0 });
});

test("a flag chip stored as is-not keeps its operator when edited", () => {
  const { instance, dispatched } = flagPanel("false", undefined, "isnot");
  instance.applyBool("true");
  assert.equal(instance.value, "true");
  assert.deepEqual(JSON.parse(JSON.stringify(dispatched[0].detail)), { field: "overdue", op: "isnot", values: ["false"], chip: 0 });
});

test("Enter on a focused Yes / No button applies that value", () => {
  const { instance, dispatched } = flagPanel("true", { dataset: { fbBool: "false" } });
  instance.apply();
  assert.deepEqual(Array.from(dispatched[0].detail.values), ["false"]);
  const idle = flagPanel("true", { dataset: {} });
  idle.instance.apply();
  assert.deepEqual(idle.dispatched, []);
});

test("a page with filter chips restored from history is reloaded", () => {
  const begin = source.indexOf("// Back to a page with filter chips.");
  const snippet = source.slice(begin, source.indexOf("// fbPanel is the polymorphic", begin));
  for (const [hasBuilder, want] of [[true, 1], [false, 0]]) {
    const listeners = {};
    let reloads = 0;
    const document = {
      addEventListener: (name, fn) => { listeners[name] = fn; },
      querySelector: (selector) => (hasBuilder && selector === '[x-data="filterBuilder"]' ? {} : null),
    };
    vm.runInNewContext(snippet, { document, window: { location: { reload: () => { reloads++; } } } });
    listeners["htmx:historyRestore"]();
    assert.equal(reloads, want);
  }
});
