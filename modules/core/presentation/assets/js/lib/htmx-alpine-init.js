// HTMX-Alpine.js Integration
// Ensures Alpine components are properly initialized after HTMX swaps
(function () {
  function initAlpine(el) {
    if (!window.Alpine || !el) return;
    // Initialize Alpine only within the swapped subtree
    window.Alpine.initTree(el);
  }

  // Fired on newly added content (best place to init widgets)
  document.addEventListener('htmx:load', e => initAlpine(e.detail.elt));

  // Out-of-band swaps (hx-swap-oob) get their own event
  document.addEventListener('htmx:oobAfterSwap', e => initAlpine(e.detail.elt));

  // Safety net for any swap target that didn't trigger htmx:load
  document.addEventListener('htmx:afterSwap', e => initAlpine(e.detail.target));

  // When navigating via htmx history (back/forward), re-init restored DOM
  document.addEventListener('htmx:historyRestore', e => initAlpine(document.body));
})();