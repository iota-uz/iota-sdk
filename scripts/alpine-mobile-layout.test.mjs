import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../modules/core/presentation/assets/js/alpine.js", import.meta.url), "utf8");

function component(name, globals = {}) {
  const start = source.indexOf(`let ${name} =`);
  const end = source.indexOf("\n});", start);
  // navTabs follows the file's older semicolon-free convention.
  const legacyEnd = source.indexOf("\n})\n", start);
  const stop = legacyEnd >= 0 && legacyEnd < end ? legacyEnd + 3 : end + 4;
  return vm.runInNewContext(`${source.slice(start, stop)}; ${name}`, globals);
}

test("workspace indicator measures again after hidden mobile navigation becomes visible", () => {
  let resized;
  let disconnected = false;
  const button = { offsetLeft: 0, offsetWidth: 0 };
  const container = { querySelector: () => button };
  const state = component("navTabs", {
    ResizeObserver: class {
      constructor(callback) { resized = callback; }
      observe(target) { assert.equal(target, container); }
      disconnect() { disconnected = true; }
    },
    document: { addEventListener() {}, removeEventListener() {} },
  })("erp");
  Object.assign(state, { $refs: { tabsContainer: container }, $nextTick: (callback) => callback() });
  state.init();
  assert.equal(state.backgroundStyle.width, 0);
  button.offsetLeft = 6;
  button.offsetWidth = 120;
  resized();
  assert.equal(state.backgroundStyle.left, 6);
  assert.equal(state.backgroundStyle.width, 120);
  assert.equal(state.backgroundStyle.opacity, 1);
  state.destroy();
  assert.equal(disconnected, true);
});

test("empty table state uses visible scrollport width at mobile and desktop sizes", () => {
  let resized;
  const scrollport = { clientWidth: 340, isConnected: true };
  const element = { style: {}, closest: () => ({ parentElement: scrollport }) };
  const state = component("emptyStateCentering", {
    ResizeObserver: class {
      constructor(callback) { resized = callback; }
      observe() {}
      disconnect() {}
    },
  })();
  Object.assign(state, { $el: element, $nextTick: (callback) => callback() });
  state.init();
  for (const width of [310, 340, 380, 1230]) {
    scrollport.clientWidth = width;
    resized();
    assert.equal(element.style.width, `${width}px`);
  }
});
