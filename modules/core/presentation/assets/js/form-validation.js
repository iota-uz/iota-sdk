/*
 * Replaces the browser's built-in constraint-validation messages ("Please fill
 * out this field.") with the application's translated ones. The messages come
 * from data-* attributes on this script's own tag, rendered by layouts.Base.
 */
(function () {
  var script = document.currentScript;
  if (!script) return;
  var messages = {
    required: script.dataset.required,
    email: script.dataset.email,
    url: script.dataset.url,
    format: script.dataset.format,
  };

  function messageFor(el) {
    var v = el.validity;
    if (!v) return "";
    if (v.valueMissing) return messages.required;
    if (v.typeMismatch) {
      if (el.type === "email") return messages.email;
      if (el.type === "url") return messages.url;
      return messages.format;
    }
    if (v.patternMismatch) return messages.format;
    return "";
  }

  document.addEventListener(
    "invalid",
    function (event) {
      var el = event.target;
      if (!el || typeof el.setCustomValidity !== "function") return;
      var message = messageFor(el);
      if (message) el.setCustomValidity(message);
    },
    true
  );

  // A required radio group fires "invalid" on every radio, so picking one must
  // clear the message on the whole group, not only on the selected radio.
  function controlsToReset(el) {
    if (el.type === "radio" && el.form && el.name) {
      return Array.prototype.filter.call(el.form.elements, function (control) {
        return control.type === "radio" && control.name === el.name;
      });
    }
    return [el];
  }

  function reset(event) {
    var el = event.target;
    if (!el || typeof el.setCustomValidity !== "function") return;
    controlsToReset(el).forEach(function (control) {
      control.setCustomValidity("");
    });
  }
  document.addEventListener("input", reset, true);
  document.addEventListener("change", reset, true);
})();
