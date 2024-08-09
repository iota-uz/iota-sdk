import "./alpine.lib.min.js";

let relativeFormat = () => ({
  format(dateStr = new Date().toISOString(), locale = "ru") {
    let date = new Date(dateStr)
    let timeMs = date.getTime();
    let delta = Math.round((timeMs - Date.now()) / 1000);
    let cutoffs = [60, 3600, 86400, 86400 * 7, 86400 * 30, 86400 * 365, Infinity];
    let units = ["second", "minute", "hour", "day", "week", "month", "year"];
    let unitIdx = cutoffs.findIndex((cutoff) => cutoff > Math.abs(delta));
    let divisor = unitIdx ? cutoffs[unitIdx - 1] : 1;
    let rtf = new Intl.RelativeTimeFormat(locale, {numeric: "auto"});
    return rtf.format(Math.floor(delta / divisor), units[unitIdx]);
  }
});

let passwordVisibility = () => ({
  toggle(e) {
    let inputId = e.target.value;
    let input = document.getElementById(inputId);
    if (input) {
    	if (e.target.checked) input.setAttribute("type", "text")
    	else input.setAttribute("type", "password")
    }
  }
});

let dialogEvents = {
  closing: new Event("closing"),
  closed: new Event("closed"),
  opening: new Event("opening"),
  opened: new Event("opened"),
  removed: new Event("removed"),
}

async function animationsComplete(el) {
  return await Promise.allSettled(el.getAnimations().map(animation => animation.finished));
}

let dialog = () => ({
  open: false,
  toggle() {
    this.open = !this.open;
  },
  attrsObserver: new MutationObserver((mutations) => {
    mutations.forEach(async (mutation) => {
      if (mutation.attributeName === "open") {
        let dialog = mutation.target;
        let isOpen = dialog.hasAttribute("open");
        if (!isOpen) return;

        dialog.removeAttribute("inert");

        let focusTarget = dialog.querySelector('[autofocus]');
        let dialogBtn = dialog.querySelector("button");
        if (focusTarget) focusTarget.focus();
        else if (dialogBtn) dialogBtn.focus();

        dialog.dispatchEvent(dialogEvents.opening);
        await animationsComplete(dialog);
        dialog.dispatchEvent(dialogEvents.opened);
      }
    });
  }),
  deleteObserver: new MutationObserver((mutations, observer) => {
    mutations.forEach((mutation) => {
      mutation.removedNodes.forEach((removed) => {
        if (removed.nodeName === "DIALOG") {
          removed.removeEventListener("click", this.lightDismiss);
          removed.removeEventListener("close", this.close);
          removed.dispatchEvent(dialogEvents.removed)
        }
      });
    });
  }),
  lightDismiss({target: dialog}) {
    if (dialog.nodeName === "DIALOG") {
      dialog.close("dismiss")
    }
  },
  async close({target: dialog}) {
    dialog.setAttribute("inert", "");
    dialog.dispatchEvent(dialogEvents.closing);
    await animationsComplete(dialog);
    dialog.dispatchEvent(dialogEvents.closed);
  },
  dialog: {
    ["x-effect"]() {
      if (this.open) this.$el.showModal();
    },
    async ["x-init"]() {
      this.attrsObserver.observe(this.$el, {
        attributes: true,
      });
      this.deleteObserver.observe(document.body, {
        attributes: false,
        subtree: false,
        childList: true,
      });
      await animationsComplete(this.$el);
    },
    ["@click"](e) {
      this.lightDismiss(e);
    },
    ["@close"](e) {
      this.close(e);
    }
  }
});

document.addEventListener("alpine:init", () => {
  Alpine.data("relativeformat", relativeFormat);
  Alpine.data("passwordVisibility", passwordVisibility);
  Alpine.data("dialog", dialog);
});