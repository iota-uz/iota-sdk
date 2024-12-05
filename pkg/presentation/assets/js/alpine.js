import "./alpine.lib.min.js";
import "./alpine-focus.min.js";

let relativeFormat = () => ({
  format(dateStr = new Date().toISOString(), locale = "ru") {
    let date = new Date(dateStr);
    let timeMs = date.getTime();
    let delta = Math.round((timeMs - Date.now()) / 1000);
    let cutoffs = [
      60,
      3600,
      86400,
      86400 * 7,
      86400 * 30,
      86400 * 365,
      Infinity,
    ];
    let units = ["second", "minute", "hour", "day", "week", "month", "year"];
    let unitIdx = cutoffs.findIndex((cutoff) => cutoff > Math.abs(delta));
    let divisor = unitIdx ? cutoffs[unitIdx - 1] : 1;
    let rtf = new Intl.RelativeTimeFormat(locale, {numeric: "auto"});
    return rtf.format(Math.floor(delta / divisor), units[unitIdx]);
  },
});

let passwordVisibility = () => ({
  toggle(e) {
    let inputId = e.target.value;
    let input = document.getElementById(inputId);
    if (input) {
      if (e.target.checked) input.setAttribute("type", "text");
      else input.setAttribute("type", "password");
    }
  },
});

let dialogEvents = {
  closing: new Event("closing"),
  closed: new Event("closed"),
  opening: new Event("opening"),
  opened: new Event("opened"),
  removed: new Event("removed"),
};

async function animationsComplete(el) {
  return await Promise.allSettled(
    el.getAnimations().map((animation) => animation.finished)
  );
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

        let focusTarget = dialog.querySelector("[autofocus]");
        let dialogBtn = dialog.querySelector("button");
        if (focusTarget) focusTarget.focus();
        else if (dialogBtn) dialogBtn.focus();

        dialog.dispatchEvent(dialogEvents.opening);
        await animationsComplete(dialog);
        dialog.dispatchEvent(dialogEvents.opened);
      }
    });
  }),
  deleteObserver: new MutationObserver((mutations) => {
    mutations.forEach((mutation) => {
      mutation.removedNodes.forEach((removed) => {
        if (removed.nodeName === "DIALOG") {
          removed.removeEventListener("click", this.lightDismiss);
          removed.removeEventListener("close", this.close);
          removed.dispatchEvent(dialogEvents.removed);
        }
      });
    });
  }),
  lightDismiss({target: dialog}) {
    if (dialog.nodeName === "DIALOG") {
      dialog.close("dismiss");
    }
  },
  async close({target: dialog}) {
    dialog.setAttribute("inert", "");
    dialog.dispatchEvent(dialogEvents.closing);
    await animationsComplete(dialog);
    dialog.dispatchEvent(dialogEvents.closed);
    this.open = false;
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
    },
  },
});

let combobox = (searchable = false) => ({
  open: false,
  openedWithKeyboard: false,
  options: [],
  selectedIndex: null,
  activeIndex: null,
  selectedIndices: new Set(),
  multiple: false,
  value: "",
  observer: null,
  searchable,
  setIndex(index) {
    if (!index) return;
    let indexInt = Number(index);
    this.activeIndex = indexInt;
    if (this.multiple) {
      this.options[index].toggleAttribute("selected");
      if (this.selectedIndices.has(indexInt)) {
        this.selectedIndices.delete(indexInt);
      } else {
        this.selectedIndices.add(indexInt);
      }
    } else {
      let [selectedIndex] = this.selectedIndices;
      for (let i = 0, len = this.options.length; i < len; i++) {
        if (i === selectedIndex) this.options[i].toggleAttribute("selected");
        else this.options[i].removeAttribute("selected");
      }
      if (
        this.selectedIndices.size > 0 &&
        !this.selectedIndices.has(indexInt)
      ) {
        this.selectedIndices.clear();
      }
      if (this.selectedIndices.has(indexInt)) {
        this.selectedIndices.delete(indexInt);
      } else this.selectedIndices.add(indexInt);
    }
    this.generateValue();
    this.open = false;
    this.openedWithKeyboard = false;
  },
  toggle() {
    this.open = !this.open;
  },
  generateValue() {
    let values = [];
    for (let i of this.selectedIndices.values()) {
      values.push(this.options[i].textContent);
    }
    this.value = values.join(", ");
  },
  onInput(e) {
    for (let i = 0, len = this.options.length; i < len; i++) {
      let option = this.options[i];
      if (option.textContent.toLowerCase().startsWith(e.target.value.toLowerCase())) {
        this.activeIndex = i;
      }
    }
    if (!this.open) this.open = true;
  },
  highlightMatchingOption(pressedKey) {
    for (let i = 0, len = this.options.length; i < len; i++) {
      let option = this.options[i];
      if (option.textContent.toLowerCase().startsWith(pressedKey.toLowerCase())) {
        this.activeIndex = i;
      }
    }
    let allOptions = this.$refs.list.querySelectorAll(".combobox-option");
    if (this.activeIndex !== null) {
      allOptions[this.activeIndex]?.focus();
    }
  },
  select: {
    ["x-init"]() {
      this.options = this.$el.querySelectorAll("option");
      this.multiple = this.$el.multiple;
      for (let i = 0, len = this.options.length; i < len; i++) {
        let option = this.options[i];
        if (option.selected) {
          if (this.selectedIndices > 0 && !this.multiple) continue;
          this.selectedIndices.add(i);
        }
      }
      this.generateValue();
      this.observer = new MutationObserver(() => {
        this.options = this.$el.querySelectorAll("option");
      });
      this.observer.observe(this.$el, {
        childList: true
      });
    },
  },
});

let checkboxes = () => ({
  children: [],
  onParentChange(e) {
    this.children.forEach(c => c.checked = e.target.checked);
  },
  onChange() {
    let allChecked = this.children.every((c) => c.checked);
    let someChecked = this.children.some((c) => c.checked);
    this.$refs.parent.checked = allChecked;
    this.$refs.parent.indeterminate = !allChecked && allChecked !== someChecked;
  },
  init() {
    this.children = Array.from(this.$el.querySelectorAll("input[type='checkbox']:not(.parent)"));
    this.onChange();
  },
  destroy() {
    this.children = [];
  }
});

document.addEventListener("alpine:init", () => {
  Alpine.data("relativeformat", relativeFormat);
  Alpine.data("passwordVisibility", passwordVisibility);
  Alpine.data("dialog", dialog);
  Alpine.data("combobox", combobox);
  Alpine.data("checkboxes", checkboxes);
});
