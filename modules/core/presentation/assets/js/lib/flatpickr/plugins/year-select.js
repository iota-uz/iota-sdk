export function getEventTarget(event) {
  try {
    if (typeof event.composedPath === "function") {
      const path = event.composedPath();
      return path[0];
    }
    return event.target;
  } catch (error) {
    return event.target;
  }
}

function clearNode(node) {
  while (node.firstChild) node.removeChild(node.firstChild);
}

export default function yearSelect({dateFormat = 'Y', altFormat = 'Y'} = {}) {
  return function(fp) {
    let yearContainer;
    fp.config.dateFormat = dateFormat;
    fp.config.altFormat = altFormat;

    function clearUnnecessaryDOMElements() {
      if (!fp.rContainer) return;

      clearNode(fp.rContainer);
      if (fp.monthNav) {
	fp.monthNav.remove();
      }
    }

    function build() {
      if (!fp.rContainer) return;
      yearContainer = fp._createElement("div", "flatpickr-yearSelect-years");
      yearContainer.tabIndex = "-1";

      buildYears();

      fp.rContainer.appendChild(yearContainer);
    }

    function buildYears() {
      if (!yearContainer) return;
      clearNode(yearContainer);
      let fragment = document.createDocumentFragment();
      for (let i = 0; i < 10; i++) {
	let date = new Date();
	date.setFullYear(date.getFullYear() - i);
	let year = fp.createDay(`flatpickr-yearSelect-year flatpickr-yearSelect-year-${date.getFullYear()}`, date, 0, 0);
	if (year.dateObj.getFullYear() === new Date().getFullYear()) {
	  year.classList.add("today");
	}
	year.textContent = year.dateObj.getFullYear();
	year.addEventListener("click", selectYear);
	fragment.appendChild(year);
      }
      yearContainer.appendChild(fragment);
    }

    function selectYear(e) {
      e.preventDefault();
      e.stopPropagation();
      let target = getEventTarget(e);
      if (!(target instanceof Element)) return;
      if (target.classList.contains("flatpickr-disabled")) return;
      if (target.classList.contains("notAllowed")) return;

      let selectedDates = [];
      if (fp.config.mode === "single") {
	selectedDates = [target.dateObj]
      } else if (fp.config.mode === "multiple") {
	selectedDates.push(target.dateObj);
      } else if (fp.config.mode === "range") {
	if (fp.selectedDates.length === 2) {
	  selectedDates = [target.dateObj];
	} else {
	  selectedDates = fp.selectedDates.concat([target.dateObj]);
	  selectedDates.sort((a, b) => a.getTime() - b.getTime());
	}
      }

      fp.setDate(selectedDates, true);

      if (fp.config.closeOnSelect) {
	let single = fp.config.mode === "single";
	let range =
	  fp.config.mode === "range" && fp.selectedDates.length === 2;
	if (single || range) fp.close();
      }
    }

    function bindEvents() {
      fp._bind(yearContainer, "mouseover", (e) => {
	if (fp.config.mode === "range") {
	  fp.onMouseOver(getEventTarget(e), "flatpickr-yearSelect-year")
	}
      })
    }

    function setCurrentlySelected() {
      if (!fp.rContainer) return;
      if (!fp.selectedDates.length) return;

      const currentlySelected = fp.rContainer.querySelectorAll(
	".flatpickr-yearSelect-year.selected"
      );

      for (let index = 0; index < currentlySelected.length; index++) {
	currentlySelected[index].classList.remove("selected");
      }
      for (let date of fp.selectedDates) {
	const year = fp.rContainer.querySelector(
	  `.flatpickr-yearSelect-year-${date.getFullYear()}`
	);

	if (year) {
	  year.classList.add("selected");
	}
      }
    }

    function closeHook() {
      if (fp.config?.mode === "range" && fp.selectedDates.length === 1) {
	fp.clear(false);
      }
      if (!fp.selectedDates.length) buildYears();
    }

    function destroyPluginInstance() {
      if (yearContainer != null) {
	const years = yearContainer.querySelectorAll(
	  ".flatpickr-yearSelect-year"
	);
	for (let i = 0; i < years.length; i++) {
	  years[index].removeEventListener("click", selectYear);
	}
      }
    }
    return {
      onParseConfig() {
	fp.config.enableTime = false;
      },
      onValueUpdate: setCurrentlySelected,
      onReady: [clearUnnecessaryDOMElements, build, bindEvents, setCurrentlySelected, () => {
	fp.config.onClose.push(closeHook);
	fp.loadedPlugins.push("yearSelect");
      }],
      onDestroy: [
	destroyPluginInstance,
	() => {
	  fp.config.onClose = fp.config.onClose.filter((hook) => hook !== closeHook);
	}
      ],
    }
  }
}
