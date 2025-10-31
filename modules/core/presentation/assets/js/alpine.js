import "./lib/alpine.lib.min.js";
import "./lib/alpine-focus.min.js";
import "./lib/alpine-anchor.min.js";
import "./lib/alpine-mask.min.js";
import Sortable from "./lib/alpine-sort.js";

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
  allOptions: [],
  activeIndex: null,
  selectedIndices: new Set(),
  selectedValues: new Map(),
  activeValue: null,
  multiple: false,
  observer: null,
  searchQuery: '',
  searchable,
  setValue(value) {
    if (value == null || !(this.open || this.openedWithKeyboard)) return;
    let index, option
    for (let i = 0, len = this.allOptions.length; i < len; i++) {
      let o = this.allOptions[i];
      if (o.value === value) {
        index = i;
        option = o;
      }
    }
    if (index == null || index > this.allOptions.length - 1) return;
    if (this.multiple) {
      this.allOptions[index].toggleAttribute("selected");
      if (this.selectedValues.has(value)) {
        this.selectedValues.delete(value);
      } else {
        this.selectedValues.set(value, {
          value,
          label: option.textContent,
        });
      }
    } else {
      for (let i = 0, len = this.allOptions.length; i < len; i++) {
        let option = this.allOptions[i];
        if (option.value === value) this.allOptions[i].toggleAttribute("selected");
        else this.allOptions[i].removeAttribute("selected");
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
  onSearch(e) {
    if (!this.open) this.open = true
    this.searchQuery = e.target.value.toLowerCase();
    this.options = Array.from(this.allOptions).filter((o) => {
      return o.textContent.toLowerCase().includes(this.searchQuery);
    });
    if (this.options.length > 0) {
      let option = this.options[0];
      this.activeValue = option.value;
    }
    if (!this.searchQuery) {
      this.options = this.$el.querySelectorAll("option");
    }
  },
  highlightMatchingOption(pressedKey) {
    this.setActiveIndex(pressedKey);
    this.setActiveValue(pressedKey);
    let allOptions = this.$refs.list.querySelectorAll(".combobox-option");
    if (this.activeIndex !== null) {
      allOptions[this.activeIndex]?.focus();
    }
  },
  removeSelectedValue(value) {
    if (!this.selectedValues.has(value)) return;
    this.selectedValues.delete(value);

    const select = this.$refs.select;
    if (select) {
      for (const option of select.options) {
        if (option.value === value) {
          option.removeAttribute("selected");
          // select.removeChild(option); // TODO: Why removed???
          break;
        }
      }
    }
    select?.dispatchEvent(new Event("change"));
  },
  select: {
    ["x-init"]() {
      this.options = this.$el.querySelectorAll("option");
      this.allOptions = this.options;
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

let filtersDropdown = () => ({
  open: false,
  selected: [],
  init() {
    this.selected = Array.from(this.$el.querySelectorAll('input[type=checkbox]:checked'))
      .map(el => el.value);
  },
  toggleValue(val) {
    const index = this.selected.indexOf(val);
    if (index === -1) {
      this.selected.push(val);
    } else {
      this.selected.splice(index, 1);
    }
  }
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
    const list = document.getElementById(this.$id('spotlight'));
    const count = list.childElementCount;
    this.highlightedIndex = (this.highlightedIndex + 1) % count;

    this.$nextTick(() => {
      const item = list.children[this.highlightedIndex];
      if (item) {
        item.scrollIntoView({block: 'nearest', behavior: 'smooth'});
      }
    });
  },

  highlightPrevious() {
    const list = document.getElementById(this.$id('spotlight'));
    const count = list.childElementCount;
    this.highlightedIndex = (this.highlightedIndex - 1 + count) % count;

    this.$nextTick(() => {
      const item = list.children[this.highlightedIndex];
      if (item) {
        item.scrollIntoView({block: 'nearest', behavior: 'smooth'});
      }
    });
  },
  goToLink() {
    const item = document.getElementById(this.$id('spotlight')).children[this.highlightedIndex];
    if (item) {
      item.children[0].click();
    }
  }
});

let datePicker = ({
  locale = 'ru',
  mode = 'single',
  dateFormat = 'Y-m-d',
  labelFormat = 'F j, Y',
  minDate = '',
  maxDate = '',
  selectorType = 'day',
  selected = [],
} = {}) => ({
  selected: [],
  localeMap: {
    ru: {
      path: '/ru.js',
      key: 'ru'
    },
    uz: {
      path: '/uz.js',
      key: 'uz_latn'
    },
  },
  async init() {
    mode = mode || 'single';
    selectorType = selectorType || 'day';
    labelFormat = labelFormat || 'F j, Y';
    dateFormat = dateFormat || 'z';

    let {default: flatpickr} = await import("./lib/flatpickr/index.js");
    let found = this.localeMap[locale];
    if (found) {
      let {default: localeData} = await import(`./lib/flatpickr/locales/${found.path}`);
      flatpickr.localize(localeData[found.key]);
    }
    let plugins = [];
    if (selectorType === 'month') {
      let {default: monthSelect} = await import('./lib/flatpickr/plugins/month-select.js');
      plugins.push(monthSelect({
        altFormat: labelFormat,
        dateFormat: dateFormat,
        shortHand: true,
      }))
    } else if (selectorType === 'week') {
      let {default: weekSelect} = await import('./lib/flatpickr/plugins/week-select.js');
      plugins.push(weekSelect())
    } else if (selectorType === 'year') {
      let {default: yearSelect} = await import('./lib/flatpickr/plugins/year-select.js');
      plugins.push(yearSelect())
    }
    if (selected) {
      this.selected = selected;
    }
    let self = this;
    flatpickr(this.$refs.input, {
      altInput: true,
      static: true,
      altInputClass: "form-control-input input outline-none w-full",
      altFormat: labelFormat,
      dateFormat: dateFormat,
      mode,
      minDate: minDate || null,
      maxDate: maxDate || null,
      defaultDate: selected,
      plugins,
      onChange(selected = []) {
        let formattedDates = selected.map((s) => flatpickr.formatDate(s, dateFormat));
        if (!formattedDates.length) return;
        if (mode === 'single') {
          self.selected = [formattedDates[0]];
        } else if (mode === 'range') {
          if (formattedDates.length === 2) self.selected = formattedDates;
        } else {
          self.selected = formattedDates;
        }
        // Dispatch custom event for HTMX integration
        self.$nextTick(() => {
          self.$el.dispatchEvent(new CustomEvent('date-selected', {
            bubbles: true,
            detail: {selected: self.selected}
          }));
        });
      },
    });
  }
})

let navTabs = (defaultValue = '') => ({
  activeTab: defaultValue,
  backgroundStyle: {left: 0, width: 0, opacity: 0},
  restoreHandler: null,

  init() {
    this.$nextTick(() => this.updateBackground());

    // Listen for restore-tab event on document since it bubbles up
    this.restoreHandler = (event) => {
      if (event.detail && event.detail.value) {
        this.activeTab = event.detail.value;
        this.$nextTick(() => this.updateBackground());
      }
    };

    document.addEventListener('restore-tab', this.restoreHandler);
  },

  destroy() {
    if (this.restoreHandler) {
      document.removeEventListener('restore-tab', this.restoreHandler);
    }
  },

  setActiveTab(tabValue) {
    this.activeTab = tabValue;
    this.$nextTick(() => this.updateBackground());
    // Emit event for parent components to handle
    this.$dispatch('tab-changed', {value: tabValue});
  },

  updateBackground() {
    const tabsContainer = this.$refs.tabsContainer;
    if (!tabsContainer) return;

    const activeButton = tabsContainer.querySelector(`button[data-tab-value="${this.activeTab}"]`);
    if (activeButton) {
      this.backgroundStyle = {
        left: activeButton.offsetLeft,
        width: activeButton.offsetWidth,
        opacity: 1
      };
    }
  },

  isActive(tabValue) {
    return this.activeTab === tabValue;
  },

  getTabClasses(tabValue) {
    return this.isActive(tabValue)
      ? 'text-slate-900'
      : 'text-gray-500 hover:text-slate-300';
  }
})

let sidebar = () => ({
  isCollapsed: localStorage.getItem('sidebar-collapsed') === 'true',
  storedTab: localStorage.getItem('sidebar-active-tab') || null,

  toggle() {
    this.isCollapsed = !this.isCollapsed;
    localStorage.setItem('sidebar-collapsed', this.isCollapsed.toString());
  },

  handleTabChange(event) {
    // Save the selected tab to localStorage
    if (event.detail && event.detail.value) {
      localStorage.setItem('sidebar-active-tab', event.detail.value);
      this.storedTab = event.detail.value;
    }
  },

  getStoredTab() {
    return this.storedTab;
  },

  initSidebar() {
    // Apply initial state class to prevent flash
    this.$nextTick(() => {
      if (this.isCollapsed) {
        this.$el.classList.add('sidebar-collapsed');
      }

      // Only restore tab if there are multiple tab buttons rendered
      if (this.storedTab && this.$el.querySelector('[role="tablist"]')) {
        // Wait a bit for navTabs to initialize
        setTimeout(() => {
          this.$dispatch('restore-tab', {value: this.storedTab});
        }, 100);
      }
    });
  }
})

let disableFormElementsWhen = (query) => ({
  matches: window.matchMedia(query).matches,
  media: null,
  onChange() {
    this.matches = window.matchMedia(query).matches;
    this.disableAllFormElements();
  },
  disableAllFormElements() {
    let elements = this.$el.querySelectorAll('input,select,textarea');
    for (let element of elements) {
      element.disabled = this.matches;
    }
  },
  init() {
    this.media = window.matchMedia(query);
    this.media.addEventListener('change', this.onChange.bind(this));
    this.disableAllFormElements();
  },
  destroy() {
    if (this.media == null) return;
    this.media.removeEventListener('change', this.onChange.bind(this));
    this.media = null;
  }
})

let editableTableRows = ({rows, emptyRow} = {rows: [], emptyRow: ''}) => ({
  emptyRow,
  rows,
  addRow() {
    this.rows.push({id: Math.random().toString(32).slice(2), html: this.emptyRow})
  },
  removeRow(id) {
    this.rows = this.rows.filter((row) => row.id !== id);
  }
});

let kanban = () => ({
  col: {
    key: '',
    oldIndex: 0,
    newIndex: 0
  },
  card: {
    key: '',
    newCol: '',
    oldCol: '',
    oldIndex: 0,
    newIndex: 0,
  },
  changeCol(col) {
    this.col = col;
  },
  changeCard(card) {
    this.card = card;
  }
})

let moneyInput = (config = {}) => ({
  displayValue: '',
  amountInCents: config.value || 0,
  min: config.min ?? null,
  max: config.max ?? null,
  decimal: config.decimal || '.',
  thousand: config.thousand || ',',
  precision: config.precision || 2,
  conversionRate: config.conversionRate || 0,
  convertTo: config.convertTo || '',
  convertedAmount: 0,
  validationError: '',

  // Helper to calculate divisor (reduces code duplication)
  getDivisor() {
    return Math.pow(10, this.precision);
  },

  // Convert cents to formatted display value
  centsToDisplay(cents) {
    return (cents / this.getDivisor()).toFixed(this.precision);
  },

  // Parse display value to cents
  displayToCents(value) {
    // Remove all non-numeric characters except decimal point and minus sign
    const cleaned = value.replace(/[^0-9.-]/g, '');

    // Handle edge cases: multiple decimals, multiple minus signs
    const parts = cleaned.split(this.decimal);
    let normalized = parts[0] || '0';
    if (parts.length > 1) {
      // Take only the first decimal part
      normalized += '.' + parts[1];
    }

    // Handle negative sign (should only be at start)
    const isNegative = normalized.startsWith('-');
    const absoluteValue = normalized.replace(/-/g, '');
    const finalValue = (isNegative ? '-' : '') + absoluteValue;

    const floatValue = parseFloat(finalValue) || 0;
    return Math.round(floatValue * this.getDivisor());
  },

  init() {
    this.displayValue = this.centsToDisplay(this.amountInCents);
    this.updateConversion();

    // Watch amountInCents for external changes (e.g., from calculator scripts)
    // Only update displayValue if it doesn't match the current amountInCents
    // This prevents circular updates during user input
    this.$watch('amountInCents', (value) => {
      const expectedDisplay = this.centsToDisplay(value);
      // Only update if displayValue differs (accounting for formatting)
      if (this.displayToCents(this.displayValue) !== value) {
        this.displayValue = expectedDisplay;
        this.updateConversion();
      }
    });
  },

  onInput(event) {
    this.amountInCents = this.displayToCents(event.target.value);
    this.validateMinMax();
    this.updateConversion();

    // Dispatch custom event so parent scopes can react to user input
    const hiddenInput = event.target.closest('[x-data]').querySelector('input[type="hidden"]');
    if (hiddenInput) {
      hiddenInput.dispatchEvent(new CustomEvent('money-changed', {
        bubbles: true,
        detail: { amountInCents: this.amountInCents }
      }));
    }
  },

  validateMinMax() {
    this.validationError = '';

    if (this.min !== null && this.amountInCents < this.min) {
      const minDisplay = this.centsToDisplay(this.min);
      this.validationError = `Minimum amount is ${minDisplay}`;
    }

    if (this.max !== null && this.amountInCents > this.max) {
      const maxDisplay = this.centsToDisplay(this.max);
      this.validationError = `Maximum amount is ${maxDisplay}`;
    }
  },

  updateConversion() {
    if (this.conversionRate > 0) {
      const floatValue = this.amountInCents / this.getDivisor();
      this.convertedAmount = floatValue * this.conversionRate;
    } else {
      this.convertedAmount = 0;
    }
  },

  // Format conversion amount for display (handles negative values)
  formatConversion() {
    if (this.convertedAmount === 0) return '0';

    const absValue = Math.abs(this.convertedAmount);
    const sign = this.convertedAmount < 0 ? '-' : '';
    return sign + absValue.toFixed(this.precision);
  }
});

document.addEventListener("alpine:init", () => {
  Alpine.data("relativeformat", relativeFormat);
  Alpine.data("passwordVisibility", passwordVisibility);
  Alpine.data("dialog", dialog);
  Alpine.data("combobox", combobox);
  Alpine.data("filtersDropdown", filtersDropdown);
  Alpine.data("checkboxes", checkboxes);
  Alpine.data("spotlight", spotlight);
  Alpine.data("dateFns", dateFns);
  Alpine.data("datePicker", datePicker);
  Alpine.data("navTabs", navTabs);
  Alpine.data("sidebar", sidebar);
  Alpine.data("disableFormElementsWhen", disableFormElementsWhen);
  Alpine.data("editableTableRows", editableTableRows);
  Alpine.data("kanban", kanban);
  Alpine.data("moneyInput", moneyInput);
  Sortable(Alpine);
});
