// HTMX-Alpine.js Integration
// Ensures Alpine components are properly initialized after HTMX swaps
(function () {
  function initAlpine(el) {
    if (!window.Alpine || !el) return;
    // Initialize Alpine only within the swapped subtree
    window.Alpine.initTree(el);
  }

  // Fired on newly added content (best place to init widgets). htmx also
  // fires this once for document.body during its own initial htmx.process()
  // call on page load — that races with Alpine's own native auto-init
  // bootstrap, double-initializing the whole already-initialized page and
  // duplicating every x-for-rendered node on it (e.g. two entries per
  // combobox option). Skip that one redundant case; genuinely new subtrees
  // swapped in later are never document.body itself.
  document.addEventListener('htmx:load', e => {
    if (e.detail.elt === document.body) return;
    initAlpine(e.detail.elt);
  });

  // Out-of-band swaps (hx-swap-oob) get their own event
  document.addEventListener('htmx:oobAfterSwap', e => initAlpine(e.detail.elt));

  // Safety net for any swap target that didn't trigger htmx:load
  document.addEventListener('htmx:afterSwap', e => initAlpine(e.detail.target));

  // When navigating via htmx history (back/forward), re-init restored DOM
  document.addEventListener('htmx:historyRestore', e => initAlpine(document.body));
})();