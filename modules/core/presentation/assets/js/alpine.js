import "./lib/alpine.lib.min.js";
import "./lib/alpine-focus.min.js";
import "./lib/alpine-anchor.min.js";
import "./lib/alpine-mask.min.js";
import Sortable from "./lib/alpine-sort.js";

let dateTimeFormat = () => ({
  format(dateStr = new Date().toISOString(), locale = document.documentElement.lang || "ru") {
    const date = new Date(dateStr);
    return date.toLocaleString(locale, {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      timeZoneName: 'short'
    });
  }
})

let relativeFormat = () => ({
  format(dateStr = new Date().toISOString(), locale = document.documentElement.lang || "ru") {
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
  formatter: new Intl.DateTimeFormat(document.documentElement.lang || "ru", {
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
      else this.$el.close();
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

let combobox = (searchable = false, canCreateNew = false) => ({
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
  canCreateNew,
  setValue(value) {
    if (!this.options.length && this.canCreateNew && this.searchQuery.length) {
      this.$refs.createOption.click();
      return;
    }
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
    this.searchQuery = '';
    this.options = [...this.allOptions];
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
    let searchValue = e.target.value.trim();
    this.options = this.allOptions.filter((o) => {
      return o.textContent.toLowerCase().includes(searchValue.toLowerCase());
    });
    if (this.options.length > 0) {
      let option = this.options[0];
      this.activeValue = option.value;
    }
    if (!searchValue) {
      this.options = this.allOptions;
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
      this.options = Array.from(this.$el.querySelectorAll("option"));
      this.allOptions = [...this.options];
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
        this.options = Array.from(this.$el.querySelectorAll("option"));
        this.allOptions = [...this.options];
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
    // Use [checked] attribute selector instead of :checked pseudo-selector
    // because Alpine's :checked binding may clear the property before init() runs
    this.selected = Array.from(this.$el.querySelectorAll('input[type=checkbox][checked]'))
      .map(el => el.value);
  },
  toggleValue(val) {
    const index = this.selected.indexOf(val);
    if (index === -1) {
      this.selected.push(val);
    } else {
      this.selected.splice(index, 1);
    }
    // Dispatch custom event after Alpine state is updated
    // We use 'filter-changed' custom event instead of 'change' to avoid race condition
    // where HTMX collects form data before Alpine updates checkbox state
    this.$nextTick(() => {
      this.$el.dispatchEvent(new CustomEvent('filter-changed', {bubbles: true}));
    });
  },
  clearAll() {
    this.selected = [];
    this.$el.querySelectorAll('input[type=checkbox]').forEach(cb => cb.checked = false);
    this.$nextTick(() => {
      this.$el.dispatchEvent(new CustomEvent('filter-changed', {bubbles: true}));
    });
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
        if (!formattedDates.length) {
          self.selected = [];
          self.$nextTick(() => {
            self.$el.dispatchEvent(new CustomEvent('date-selected', {
              bubbles: true,
              detail: {selected: self.selected}
            }));
          });
          return;
        }
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

// Helper function to determine sidebar initial state with 3-state priority
// Make globally available for use in templates
window.initSidebarCollapsed = function() {
  // Priority 1: Check server hint (overrides localStorage)
  const el = document.querySelector('[data-sidebar-state]');
  const serverState = el?.dataset.sidebarState;

  if (serverState === 'collapsed') {
    return true;
  } else if (serverState === 'expanded') {
    return false;
  }

  // Priority 2: Fall back to localStorage (only when serverState is 'auto' or missing)
  const stored = localStorage.getItem('sidebar-collapsed');
  if (stored !== null) {
    return stored === 'true';
  }

  // Priority 3: Default to expanded
  return false;
}

let createAnchoredOverlayPositioner = ({gap = 8, minTop = 8} = {}) => ({
  rightStart(anchorEl) {
    if (!anchorEl) return null;
    const rect = anchorEl.getBoundingClientRect();
    return {
      left: rect.right + gap,
      top: Math.max(minTop, rect.top),
    };
  },
});

let sidebarShell = () => ({
  isCollapsed: initSidebarCollapsed(),
  storedTab: localStorage.getItem('sidebar-active-tab') || null,

  toggle() {
    this.isCollapsed = !this.isCollapsed;
    localStorage.setItem('sidebar-collapsed', this.isCollapsed.toString());
  },

  handleSidebarClick(event) {
    const interactive = 'a, button, input, summary, [role="button"], .btn';
    if (event.target.closest(interactive)) return;
    this.toggle();
    this.$dispatch('sidebar-toggle');
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

  initSidebarShell() {
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
  },
})

let sidebarNavigation = () => ({
  collapsedMenus: [],
  outsideClickHandler: null,
  escapeHandler: null,
  overlayPositioner: createAnchoredOverlayPositioner({gap: 8, minTop: 8}),

  onCollapsedGroupTrigger(event) {
    const trigger = event.currentTarget;
    if (!trigger) return;

    const groupId = trigger.dataset.groupId;
    const depth = Number(trigger.dataset.depth || 0);
    if (!groupId || Number.isNaN(depth)) return;

    this.openCollapsedMenu(trigger, groupId, depth);
  },

  openCollapsedMenu(anchorEl, groupId, depth) {
    if (!this.isCollapsed) return;

    const current = this.collapsedMenus[depth];
    if (current && current.id === groupId) {
      this.collapsedMenus = this.collapsedMenus.slice(0, depth);
      return;
    }

    const position = this.overlayPositioner.rightStart(anchorEl);
    if (!position) return;

    this.collapsedMenus = this.collapsedMenus.slice(0, depth);
    this.collapsedMenus[depth] = {
      id: groupId,
      left: position.left,
      top: position.top,
    };
  },

  closeCollapsedMenus() {
    this.collapsedMenus = [];
  },

  isCollapsedMenuOpen(groupId, depth) {
    return this.isCollapsed && this.collapsedMenus[depth]?.id === groupId;
  },

  isCollapsedMenuOpenFor(el) {
    if (!el) return false;
    const groupId = el.dataset.groupId;
    const depth = Number(el.dataset.depth || 0);
    if (!groupId || Number.isNaN(depth)) return false;
    return this.isCollapsedMenuOpen(groupId, depth);
  },

  collapsedMenuStyleFor(el) {
    if (!el) return {};
    const groupId = el.dataset.groupId;
    const depth = Number(el.dataset.depth || 0);
    if (!groupId || Number.isNaN(depth)) return {};
    if (!this.isCollapsedMenuOpen(groupId, depth)) {
      return {};
    }
    const menu = this.collapsedMenus[depth];
    return {
      left: `${menu.left}px`,
      top: `${menu.top}px`,
    };
  },

  handleCollapsedMenuOutsideClick(event) {
    if (!this.isCollapsed || this.collapsedMenus.length === 0) return;

    const target = event.target;
    if (
      target.closest('[data-sidebar-collapsed-menu="true"]') ||
      target.closest('[data-sidebar-collapsed-group-trigger="true"]')
    ) {
      return;
    }

    this.closeCollapsedMenus();
  },

  handleCollapsedMenuEscape(event) {
    if (event.key === 'Escape') {
      this.closeCollapsedMenus();
    }
  },

  initSidebarNavigation() {
    this.outsideClickHandler = this.handleCollapsedMenuOutsideClick.bind(this);
    this.escapeHandler = this.handleCollapsedMenuEscape.bind(this);
    document.addEventListener('click', this.outsideClickHandler);
    document.addEventListener('keydown', this.escapeHandler);
    this.$watch('isCollapsed', () => {
      this.closeCollapsedMenus();
    });
    this.$watch('storedTab', () => {
      this.closeCollapsedMenus();
    });
  },

  destroy() {
    if (this.outsideClickHandler) {
      document.removeEventListener('click', this.outsideClickHandler);
      this.outsideClickHandler = null;
    }
    if (this.escapeHandler) {
      document.removeEventListener('keydown', this.escapeHandler);
      this.escapeHandler = null;
    }
  },
})

let disableFormElementsWhen = (query) => ({
  matches: window.matchMedia(query).matches,
  media: null,
  observer: null,
  changeHandler: null,
  onChange() {
    this.matches = window.matchMedia(query).matches;
    this.disableAllFormElements();
  },
  disableAllFormElements() {
    let elements = this.$el.querySelectorAll('input,select,textarea,button');
    for (let element of elements) {
      element.disabled = this.matches;
    }
  },
  init() {
    this.media = window.matchMedia(query);
    this.changeHandler = this.onChange.bind(this);
    this.media.addEventListener('change', this.changeHandler);
    this.observer = new MutationObserver(() => this.disableAllFormElements());
    this.observer.observe(this.$el, { childList: true, subtree: true });
    this.disableAllFormElements();
  },
  destroy() {
    if (this.media == null) return;
    if (this.changeHandler) {
      this.media.removeEventListener('change', this.changeHandler);
      this.changeHandler = null;
    }
    if (this.observer) {
      this.observer.disconnect();
      this.observer = null;
    }
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
        detail: {amountInCents: this.amountInCents}
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

let dateRangeButtons = ({formID, hiddenStartID, hiddenEndID} = {}) => ({
  formatDate(d) {
    const year = d.getFullYear();
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  },
  updateDateRange(startDate, endDate) {
    const startStr = this.formatDate(startDate);
    const endStr = this.formatDate(endDate);

    document.getElementById(hiddenStartID).value = startStr;
    document.getElementById(hiddenEndID).value = endStr;

    const fpElements = document.querySelectorAll('.flatpickr-input');
    fpElements.forEach(fp => {
      if (fp._flatpickr) {
        fp._flatpickr.setDate([startDate, endDate], true);
      }
    });

    const form = document.getElementById(formID);
    if (form) {
      // Small delay to ensure flatpickr has updated the form inputs
      setTimeout(() => {
        if (typeof htmx !== 'undefined') {
          htmx.trigger(form, 'dateRangeChange');
        } else {
          const event = new Event('change', {bubbles: true});
          form.dispatchEvent(event);
        }
      }, 50);
    }
  },
  applyDays(days) {
    const today = new Date();
    const endDate = new Date(today.getFullYear(), today.getMonth(), today.getDate());
    const startDate = new Date(today.getFullYear(), today.getMonth(), today.getDate() - (days - 1));
    this.updateDateRange(startDate, endDate);
  },
  applyMonths(months) {
    const today = new Date();
    const endDate = new Date(today.getFullYear(), today.getMonth(), today.getDate());
    const startDate = new Date(today.getFullYear(), today.getMonth() - months, today.getDate());
    this.updateDateRange(startDate, endDate);
  },
  applyFiscalYear() {
    const today = new Date();
    const endDate = new Date(today.getFullYear(), today.getMonth(), today.getDate());
    const startDate = new Date(today.getFullYear(), 0, 1);
    this.updateDateRange(startDate, endDate);
  },
  applyCurrentMonth() {
    const today = new Date();
    const endDate = new Date(today.getFullYear(), today.getMonth(), today.getDate());
    const startDate = new Date(today.getFullYear(), today.getMonth(), 1);
    this.updateDateRange(startDate, endDate);
  },
  applyAllTime() {
    // Clear hidden fields
    const hiddenStart = document.getElementById(hiddenStartID);
    const hiddenEnd = document.getElementById(hiddenEndID);
    if (hiddenStart) hiddenStart.value = '';
    if (hiddenEnd) hiddenEnd.value = '';

    // Clear all flatpickr instances
    const fpElements = document.querySelectorAll('.flatpickr-input');
    fpElements.forEach(fp => {
      if (fp._flatpickr) {
        fp._flatpickr.clear();
      }
    });

    // Trigger form update
    const form = document.getElementById(formID);
    if (form) {
      setTimeout(() => {
        if (typeof htmx !== 'undefined') {
          htmx.trigger(form, 'dateRangeChange');
        } else {
          const event = new Event('change', {bubbles: true});
          form.dispatchEvent(event);
        }
      }, 50);
    }
  }
});

// Shared CSS classes for toggle switch visual states
const TOGGLE_CLASSES = {
  checked: "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-brand-600 after:translate-x-full after:border-white",
  unchecked: "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-gray-200 after:border-gray-300"
};

/**
 * Creates Alpine.js data for managing a permission set (group of related permissions)
 * Each set has a unique ID to prevent cross-contamination when toggling
 */
function createPermissionSetData(allChecked, someChecked, permissionIds, setId) {

  return {
    // Component state
    expanded: false,
    allChecked: allChecked,
    someChecked: someChecked,
    permissionIds: permissionIds,
    setId: setId,

    // Initialize component
    init() {
      this.$nextTick(() => this.updateState());
    },

    // Toggle all permissions in this set on/off
    toggleAll() {
      const newState = !this.allChecked;
      this.setAllPermissions(newState);
    },

    // Set all permissions to a specific state
    setAllPermissions(checked) {
      this.allChecked = checked;
      this.someChecked = false;

      // Update each permission checkbox
      const checkboxes = this.getPermissionCheckboxes();
      checkboxes.forEach(checkbox => {
        if (checkbox) {
          checkbox.checked = checked;
          checkbox.dispatchEvent(new Event('change', {bubbles: true}));
        }
      });

      this.updateVisualToggle();
    },

    // Get all checkbox elements for this permission set
    getPermissionCheckboxes() {
      return this.permissionIds.map(permId =>
        document.querySelector(`#${this.setId}-perm-${permId}`)
      );
    },

    // Update component state based on checkbox states
    updateState() {
      const checkboxes = this.getPermissionCheckboxes();
      const checkedCount = checkboxes.filter(cb => cb?.checked).length;
      const totalCount = checkboxes.filter(cb => cb !== null).length;

      this.allChecked = totalCount > 0 && checkedCount === totalCount;
      this.someChecked = checkedCount > 0 && checkedCount < totalCount;

      this.updateVisualToggle();
    },

    // Update the visual toggle switch appearance
    updateVisualToggle() {
      const toggleVisual = this.$el.querySelector('[id^="toggle-visual-"]');
      if (toggleVisual) {
        toggleVisual.className = this.allChecked
          ? TOGGLE_CLASSES.checked
          : TOGGLE_CLASSES.unchecked;
      }
    }
  };
}

/**
 * Creates Alpine.js data for managing an entire resource group (contains multiple permission sets)
 * Handles the resource-level "select all" functionality
 */
function createPermissionFormData(allChecked, someChecked, permissionIds) {
  return {
    // Component state
    allChecked: allChecked,
    someChecked: someChecked,
    permissionIds: permissionIds,

    // Initialize component
    init() {
      this.$nextTick(() => this.updateState());
    },

    // Toggle all permissions in this resource group on/off
    toggleAll() {
      const newState = !this.allChecked;
      this.setAllPermissions(newState);
      this.updateNestedPermissionSets(newState);
    },

    // Set all permissions to a specific state
    setAllPermissions(checked) {
      this.allChecked = checked;

      // Update all permission checkboxes in this resource
      const checkboxes = this.getPermissionCheckboxes();
      checkboxes.forEach(checkbox => {
        checkbox.checked = checked;
        checkbox.dispatchEvent(new Event('change', {bubbles: true}));
      });

      this.updateVisualToggle();
    },

    // Get all checkbox elements for this resource group
    getPermissionCheckboxes() {
      return this.permissionIds
        .map(permId => document.querySelector(`input[name="Permissions[${permId}]"]`))
        .filter(checkbox => checkbox !== null);
    },

    // Update nested permission set components to reflect new state
    updateNestedPermissionSets(checked) {
      const nestedComponents = this.$el.querySelectorAll('[x-data]');

      nestedComponents.forEach(el => {
        // Skip self and check if it's an Alpine component with our data structure
        if (el === this.$el) return;

        const alpineData = el._x_dataStack?.[0];
        if (alpineData && typeof alpineData.allChecked !== 'undefined') {
          alpineData.allChecked = checked;
          alpineData.someChecked = false;
          alpineData.updateState?.();
        }
      });
    },

    // Update component state based on checkbox states
    updateState() {
      const checkboxes = this.getPermissionCheckboxes();
      const checkedCount = checkboxes.filter(cb => cb.checked).length;
      const totalCount = checkboxes.length;

      this.allChecked = totalCount > 0 && checkedCount === totalCount;
      this.someChecked = checkedCount > 0 && checkedCount < totalCount;

      this.updateVisualToggle();
    },

    // Update the visual toggle switch appearance
    updateVisualToggle() {
      const toggleVisual = this.$el.querySelector('[id^="toggle-visual-"]');
      if (toggleVisual) {
        toggleVisual.className = this.allChecked
          ? TOGGLE_CLASSES.checked
          : TOGGLE_CLASSES.unchecked;
      }
    }
  };
}

let fillerRows = (rowHeight = 49) => ({
  _resizeHandler: null,
  _settleHandler: null,
  init() {
    this.$nextTick(() => this.fillGrid());
    this._resizeHandler = this._debounce(() => this.fillGrid(), 150);
    window.addEventListener('resize', this._resizeHandler);
    this._settleHandler = () => {
      this.$nextTick(() => this.fillGrid());
    };
    document.addEventListener('htmx:afterSettle', this._settleHandler);
  },
  destroy() {
    if (this._resizeHandler) window.removeEventListener('resize', this._resizeHandler);
    if (this._settleHandler) document.removeEventListener('htmx:afterSettle', this._settleHandler);
  },
  fillGrid() {
    const wrapper = this.$el;
    const tbody = wrapper.querySelector('tbody#table-body');
    const thead = wrapper.querySelector('thead');
    if (!tbody || !thead) return;
    tbody.querySelectorAll('.grid-filler').forEach(r => r.remove());
    const scEl = wrapper.querySelector('[x-ref=sc]');
    if (tbody.querySelector('tr:not(.hidden) > td[colspan]')) {
      if (scEl) scEl.style.overflowY = 'hidden';
      return;
    }
    if (scEl) scEl.style.overflowY = '';
    const ths = thead.querySelector('tr').children;
    const colCount = ths.length;
    const stickyInfo = [];
    for (let i = 0; i < colCount; i++) {
      const cs = getComputedStyle(ths[i]);
      stickyInfo.push({
        isSticky: cs.position === 'sticky',
        right: cs.right !== 'auto' && cs.right !== '' ? cs.right : null,
        left: cs.left !== 'auto' && cs.left !== '' ? cs.left : null
      });
    }
    const wrapperH = scEl ? scEl.clientHeight : wrapper.offsetHeight;
    const theadH = thead.offsetHeight;
    let dataH = 0;
    for (const row of tbody.children) {
      if (!row.classList.contains('grid-filler') && !row.classList.contains('hidden')) {
        dataH += row.offsetHeight;
      }
    }
    const emptySpace = wrapperH - theadH - dataH;
    const count = Math.max(0, Math.floor(emptySpace / rowHeight));
    const frag = document.createDocumentFragment();
    for (let i = 0; i < count; i++) {
      const tr = document.createElement('tr');
      tr.className = 'grid-filler';
      for (let j = 0; j < colCount; j++) {
        const td = document.createElement('td');
        if (stickyInfo[j].isSticky && stickyInfo[j].right !== null) {
          td.className = 'grid-sticky-right';
        }
        tr.appendChild(td);
      }
      frag.appendChild(tr);
    }
    tbody.appendChild(frag);
  },
  _debounce(fn, ms) {
    let t;
    return (...a) => { clearTimeout(t); t = setTimeout(() => fn.apply(this, a), ms); };
  }
});

let tableConfig = (id) => ({
  get key() {
    return "iota-table-config-" + (id || (window.location.origin + window.location.pathname))
  },
  columns: [],
  fixedColumns: [],
  grid: { verticalLines: true, horizontalLines: true },
  table: null,
  rootEl: null,
  observer: null,
  toggleGridVertical() {
    this.grid.verticalLines = !this.grid.verticalLines;
    this.save();
    this.applyGridClasses();
  },
  toggleGridHorizontal() {
    this.grid.horizontalLines = !this.grid.horizontalLines;
    this.save();
    this.applyGridClasses();
  },
  applyGridClasses() {
    let el = this.rootEl;
    if (!el) return;
    el.classList.toggle('table-grid-no-vertical', !this.grid.verticalLines);
    el.classList.toggle('table-grid-no-horizontal', !this.grid.horizontalLines);
  },
  toggleColumn(colKey) {
    let col = this.columns.find(c => c.key === colKey);
    if (!col) return;
    col.visible = !col.visible;
    this.save();
    this.applyConfiguration();
  },

  moveColumn(fromIndex, toIndex, sync = false) {
    if (fromIndex < 0 || toIndex < 0 || fromIndex >= this.columns.length || toIndex >= this.columns.length) {
      return;
    }
    if (fromIndex === toIndex) return;
    let [col] = this.columns.splice(fromIndex, 1);
    this.columns.splice(toIndex, 0, col);
    if (sync) {
      this.fixedColumns = this.columns;
    }
    this.save();
    this.applyConfiguration();
  },

  reorderRow(row) {
    let cells = Array.from(row.children);
    let cellMap = new Map();

    cells.forEach((cell, index) => {
      let key = cell.dataset.col || `col-${index}`;
      cellMap.set(key, cell);
    });

    let fragment = document.createDocumentFragment();

    this.columns.forEach(col => {
      let cell = cellMap.get(col.key);
      if (cell) {
        if (!col.visible) {
          cell.style.display = 'none';
        } else {
          cell.style.display = '';
        }
        fragment.appendChild(cell);
      }
    });

    cellMap.forEach((cell, key) => {
      if (!this.columns.find(c => c.key === key)) {
        fragment.appendChild(cell);
      }
    });

    row.innerHTML = '';
    row.appendChild(fragment);
  },

  applyConfiguration() {
    if (!this.table) return;

    let tHead = this.table.querySelector("thead");
    if (tHead) {
      let headerRows = tHead.querySelectorAll("tr");
      headerRows.forEach(row => this.reorderRow(row));
    }

    let tBodies = this.table.querySelectorAll("tbody");
    tBodies.forEach(tbody => {
      let rows = tbody.querySelectorAll("tr");
      rows.forEach(row => this.reorderRow(row));
    });

    let tFoot = this.table.querySelector("tfoot");
    if (tFoot) {
      let footerRows = tFoot.querySelectorAll("tr");
      footerRows.forEach(row => this.reorderRow(row));
    }
  },

  syncConfiguration(domColumns) {
    let savedConfig = window.localStorage.getItem(this.key);

    if (!savedConfig) {
      savedConfig = this.save();
    }

    try {
      let saved = JSON.parse(savedConfig);
      let savedColumns = saved.columns || [];
      let savedColumnMap = new Map(savedColumns.map(c => [c.key, c]));
      let mergedColumns = [];

      if (saved.grid) {
        this.grid.verticalLines = saved.grid.verticalLines !== false;
        this.grid.horizontalLines = saved.grid.horizontalLines !== false;
      }

      savedColumns.forEach(savedCol => {
        let domCol = domColumns.find(c => c.key === savedCol.key);
        if (domCol) {
          mergedColumns.push({
            ...domCol,
            sticky: savedCol.sticky != undefined ? savedCol.sticky : domCol.sticky,
            visible: savedCol.visible != undefined ? savedCol.visible : true,
          });
        }
      });

      domColumns.forEach(domCol => {
        if (!savedColumnMap.has(domCol.key)) {
          mergedColumns.push(domCol);
        }
      });

      return mergedColumns;
    } catch (e) {
      console.error('Failed to parse saved table config:', e);
      return domColumns;
    }
  },

  extractColumnsFromDOM() {
    let table = this.$el.querySelector("table");
    if (!table) return [];

    let headerRow = table.querySelector("thead tr");
    if (!headerRow) return [];

    let columns = [];
    let headerCells = headerRow.querySelectorAll("th");

    headerCells.forEach((th, index) => {
      let key = th.dataset.col || `col-${index}`;
      let sticky = th.dataset.colSticky != undefined;
      columns.push({
        key,
        label: th.textContent.trim(),
        sticky,
      });
    });

    return columns;
  },


  resetConfiguration() {
    window.localStorage.removeItem(this.key);
    window.location.reload();
  },

  save() {
    let config = JSON.stringify({key: this.key, columns: this.columns, grid: this.grid});
    window.localStorage.setItem(this.key, config);
    return config;
  },

  init() {
    this.rootEl = this.$el;
    this.table = this.$el.querySelector("table");
    if (!this.table) return;

    let tBodies = this.table.querySelectorAll("tbody");
    if (!tBodies.length) return;

    if (!this.columns || this.columns.length === 0) {
      this.columns = this.extractColumnsFromDOM();
    }

    this.columns = this.syncConfiguration(this.columns);
    this.fixedColumns = [...this.columns];
    this.applyConfiguration();
    this.applyGridClasses();

    this.observer = new MutationObserver((mutations) => {
      for (let mutation of mutations) {
        if (mutation.type === "childList") {
          for (let node of mutation.addedNodes) {
            if (node.tagName === "TR") {
              this.reorderRow(node);
            }
          }
        }
      }
    });

    for (let body of tBodies) {
      this.observer.observe(body, {childList: true});
    }
  },

  destroy() {
    if (this.observer) this.observer.disconnect();
  },
})

document.addEventListener("alpine:init", () => {
  Alpine.data("dateTimeFormat", dateTimeFormat)
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
  Alpine.data("sidebarShell", sidebarShell);
  Alpine.data("sidebarNavigation", sidebarNavigation);
  Alpine.data("disableFormElementsWhen", disableFormElementsWhen);
  Alpine.data("editableTableRows", editableTableRows);
  Alpine.data("kanban", kanban);
  Alpine.data("moneyInput", moneyInput);
  Alpine.data("dateRangeButtons", dateRangeButtons);
  Alpine.data("createPermissionFormData", createPermissionFormData);
  Alpine.data("createPermissionSetData", createPermissionSetData);
  Alpine.data("fillerRows", fillerRows);
  Alpine.data("tableConfig", tableConfig);
  Sortable(Alpine);
});
