/*
 * Theme selection for the authenticated layout and the login page.
 *
 * The stored choice is the raw one ("system" | "light" | "dark"), but the
 * class applied to <html> is always the RESOLVED "light" or "dark":
 * Tailwind's class strategy never reacts to a literal "system" class, so
 * applying it verbatim left the app in light mode while OS dark mode was on
 * (and while embedded dashboards resolved the preference themselves).
 *
 * While "system" stays selected, a matchMedia listener re-resolves on OS
 * preference changes so the app follows the OS live without a reload.
 */
(function () {
  "use strict";

  var KEY = "iota-theme";
  var VALID = ["system", "light", "dark"];

  var root = document.documentElement;
  var media =
    window.matchMedia &&
    window.matchMedia("(prefers-color-scheme: dark)");

  function resolvedClass(choice) {
    if (choice === "light" || choice === "dark") {
      return choice;
    }
    return media && media.matches ? "dark" : "light";
  }

  function currentChoice() {
    var saved;
    try {
      saved = window.localStorage.getItem(KEY);
    } catch (e) {
      return "system";
    }
    return VALID.indexOf(saved) !== -1 ? saved : "system";
  }

  function apply(choice) {
    root.classList.remove("light", "dark", "system");
    root.classList.add(resolvedClass(choice));
  }

  function persist(choice) {
    try {
      window.localStorage.setItem(KEY, choice);
    } catch (e) {
      /* storage unavailable (private mode): keep the session-only choice */
    }
  }

  function syncRadio(choice) {
    var input = document.getElementById("theme-" + choice);
    if (input) {
      input.checked = true;
    }
  }

  var choice = currentChoice();
  syncRadio(choice);
  apply(choice);

  window.iotaTheme = {
    change: function (value) {
      choice = VALID.indexOf(value) !== -1 ? value : "system";
      persist(choice);
      syncRadio(choice);
      apply(choice);
    },
    onRadioChange: function (input) {
      this.change(input.value);
    },
  };

  if (media) {
    var onOSChange = function () {
      // choice (not currentChoice()): when storage is unavailable an
      // explicit light/dark selection lives only in this variable, and an
      // OS change must not override it.
      if (choice === "system") {
        apply("system");
      }
    };
    if (media.addEventListener) {
      media.addEventListener("change", onOSChange);
    } else if (media.addListener) {
      media.addListener(onOSChange);
    }
  }
})();
