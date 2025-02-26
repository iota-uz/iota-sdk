import "./lib/alpine.lib.min.js";
import "./lib/alpine-focus.min.js";
import "./lib/alpine-anchor.min.js";

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
    let rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
    return rtf.format(Math.floor(delta / divisor), units[unitIdx]);
  },
});

let dateFns = () => ({
  formatter: new Intl.DateTimeFormat("ru", {
    year: "numeric",
    month: "numeric",
    day: "numeric",
    hour: "numeric",
    minute: "numeric",
    second: "numeric"
  }),
  now() {
    return this.formatter.format(new Date());
  },
  startOfDay(days = 0) {
    let date = new Date();
    date.setDate(date.getDate() - days);
    date.setHours(0, 0, 0, 0);
    return date.toISOString();
  },
  endOfDay(days = 0) {
    let date = new Date();
    date.setDate(date.getDate() - days);
    date.setHours(24, 0, 0, 0);
    return date.toISOString();
  },
  startOfWeek(factor = 0) {
    let date = new Date();
    let firstDay = (date.getDate() - date.getDay() + 1) - factor * 7
    console.log("FIRST DAY: ", firstDay, "FACTOR: ", factor)
    date.setDate(firstDay)
    date.setHours(0, 0, 0, 0);
    return new Date(date).toISOString();
  },
  endOfWeek(factor = 0) {
    let date = new Date();
    let firstDay = (date.getDate() - date.getDay() + 1) - factor * 7
    let lastDay = firstDay + 7
    date.setDate(lastDay);
    date.setHours(0, 0, 0, 0);
    return new Date(date.setDate(lastDay)).toISOString();
  },
  startOfMonth(months = 0) {
    let date = new Date();
    let newDate = new Date(date.getFullYear(), date.getMonth() - months, 1);
    newDate.setHours(0, 0, 0, 0);
    return newDate.toISOString();
  },
  endOfMonth(months = 0) {
    let date = new Date();
    let newDate = new Date(date.getFullYear(), date.getMonth() + months + 1, 0);
    newDate.setHours(24, 0, 0, 0);
    return newDate.toISOString();
  }
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

let dialog = (initialState) => ({
  open: initialState || false,
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
  lightDismiss({ target: dialog }) {
    if (dialog.nodeName === "DIALOG") {
      dialog.close("dismiss");
    }
  },
  async close({ target: dialog }) {
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
  activeIndex: null,
  selectedIndices: new Set(),
  selectedValues: new Map(),
  activeValue: null,
  multiple: false,
  observer: null,
  searchable,
  setValue(value) {
    if (value == null || !(this.open || this.openedWithKeyboard)) return;
    let index, option
    for (let i = 0, len = this.options.length; i < len; i++) {
      let o = this.options[i];
      if (o.value === value) {
        index = i;
        option = o;
      }
    }
    if (index == null || index > this.options.length - 1) return;
    if (this.multiple) {
      this.options[index].toggleAttribute("selected");
      if (this.selectedValues.has(value)) {
        this.selectedValues.delete(value);
      } else {
        this.selectedValues.set(value, {
          value,
          label: option.textContent,
        });
      }
    } else {
      for (let i = 0, len = this.options.length; i < len; i++) {
        let option = this.options[i];
        if (option.value === value) this.options[i].toggleAttribute("selected");
        else this.options[i].removeAttribute("selected");
      }
      if (
        this.selectedValues.size > 0 &&
        !this.selectedValues.has(value)
      ) {
        this.selectedValues.clear();
      }
      if (this.selectedValues.has(value)) {
        this.selectedValues.delete(value);
      } else this.selectedValues.set(value, {
        value,
        label: option.textContent,
      });
    }
    this.open = false;
    this.openedWithKeyboard = false;
    if (this.selectedValues.size === 0) {
      this.$refs.select.value = "";
    }
    this.$refs.select.dispatchEvent(new Event("change"));
    this.activeValue = value;
    if (this.$refs.input) {
      this.$refs.input.value = "";
      this.$refs.input.focus();
    }
  },
  toggle() {
    this.open = !this.open;
  },
  setActiveIndex(value) {
    for (let i = 0, len = this.options.length; i < len; i++) {
      let option = this.options[i];
      if (option.textContent.toLowerCase().startsWith(value.toLowerCase())) {
        this.activeIndex = i;
      }
    }
  },
  setActiveValue(value) {
    for (let i = 0, len = this.options.length; i < len; i++) {
      let option = this.options[i];
      if (option.textContent.toLowerCase().startsWith(value.toLowerCase())) {
        this.activeValue = option.value;
        return option;
      }
    }
  },
  onInput() {
    if (!this.open) this.open = true;
  },
  highlightMatchingOption(pressedKey) {
    this.setActiveIndex(pressedKey);
    this.setActiveValue(pressedKey);
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
          this.activeIndex = i;
          this.activeValue = option.value;
          if (this.selectedValues.size > 0 && !this.multiple) continue;
          this.selectedValues.set(option.value, {
            label: option.textContent,
            value: option.value,
          })
        }
      }
      this.observer = new MutationObserver(() => {
        this.options = this.$el.querySelectorAll("option");
        if (this.$refs.input) {
          this.setActiveIndex(this.$refs.input.value);
          this.setActiveValue(this.$refs.input.value);
        }
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

let spotlight = () => ({
  isOpen: false,
  highlightedIndex: 0,

  handleShortcut(event) {
    if ((event.ctrlKey || event.metaKey) && event.key === 'k') {
      event.preventDefault();
      this.open();
    }
  },

  open() {
    this.isOpen = true;
    this.$nextTick(() => {
      const input = this.$refs.input;
      if (input) {
        setTimeout(() => input.focus(), 50);
      }
    });
  },

  close() {
    this.isOpen = false;
    this.highlightedIndex = 0;
  },

  highlightNext() {
    const itemsCount = document.getElementById(this.$id('spotlight')).childElementCount
    this.highlightedIndex = (this.highlightedIndex + 1) % itemsCount;
  },

  highlightPrevious() {
    const itemsCount = document.getElementById(this.$id('spotlight')).childElementCount
    this.highlightedIndex = (this.highlightedIndex - 1 + itemsCount) % itemsCount;
  },

  goToLink() {
    const item = document.getElementById(this.$id('spotlight')).children[this.highlightedIndex];
    if (item) {
      item.children[0].click();
    }
  }
});

document.addEventListener("alpine:init", () => {
  Alpine.data("relativeformat", relativeFormat);
  Alpine.data("passwordVisibility", passwordVisibility);
  Alpine.data("dialog", dialog);
  Alpine.data("combobox", combobox);
  Alpine.data("checkboxes", checkboxes);
  Alpine.data("spotlight", spotlight);
  Alpine.data("dateFns", dateFns);
});
