const base = document.body.dataset.docsBase ?? "";
const input = document.querySelector<HTMLInputElement>("#docs-search");
const results = document.querySelector<HTMLElement>("#search-results");
let generation = 0;
let searchModule: Promise<any> | undefined;
input?.addEventListener("input", async () => {
  const current = ++generation;
  const value = input.value.trim();
  if (!value) {
    if (results) results.hidden = true;
    return;
  }
  try {
    searchModule ??= import(/* @vite-ignore */ base + "/_pagefind/pagefind.js");
    const search = await searchModule;
    const found = await search.search(value);
    const rows = await Promise.all(
      found.results.slice(0, 8).map((row: any) => row.data()),
    );
    if (current !== generation || !results) return;
    results.replaceChildren();
    for (const row of rows) {
      const link = document.createElement("a");
      link.href = row.url;
      link.textContent = row.meta.title ?? row.url;
      results.append(link);
    }
    if (!rows.length) results.textContent = "No results";
    results.hidden = false;
  } catch {
    if (results) {
      results.textContent = "Search is available after the static build";
      results.hidden = false;
    }
  }
});
const themeLabel = document.querySelector("#theme-label");
const updateTheme = () => {
  const dark = document.documentElement.classList.contains("dark");
  document.documentElement.classList.toggle("light", !dark);
  document.documentElement.style.colorScheme = dark ? "dark" : "light";
  if (themeLabel)
    themeLabel.textContent =
      (localStorage.getItem("docs-theme") ??
        localStorage.getItem("theme") ??
        "system") === "system"
        ? "System"
        : dark
          ? "Dark"
          : "Light";
};
updateTheme();
const toggle = document.querySelector<HTMLButtonElement>("#nav-toggle");
toggle?.addEventListener("click", () => {
  const open = document.body.classList.toggle("nav-open");
  toggle.setAttribute("aria-expanded", String(open));
  toggle.querySelector("svg")?.classList.toggle("open", open);
});
let tabGroup = 0;
for (const container of document.querySelectorAll<HTMLElement>(
  "[data-doc-tabs]",
)) {
  const id = "docs-tabs-" + ++tabGroup;
  const panels = Array.from(
    container.querySelectorAll<HTMLElement>(":scope > [data-doc-tab-panel]"),
  );
  const labels = JSON.parse(container.dataset.labels ?? "[]") as string[];
  const tabs = document.createElement("div");
  tabs.setAttribute("role", "tablist");
  tabs.className =
    "nextra-scrollbar x:overflow-x-auto x:overscroll-x-contain x:overflow-y-hidden x:mt-4 x:flex x:w-full x:gap-2 x:border-b x:border-gray-200 x:pb-px x:dark:border-neutral-800 x:focus-visible:nextra-focus";
  tabs.setAttribute("aria-orientation", "horizontal");
  tabs.setAttribute("aria-label", "Documentation examples");
  const buttons = panels.map((panel, index) => {
    const button = document.createElement("button");
    button.textContent = labels[index] ?? String(index + 1);
    button.id = id + "-tab-" + index;
    panel.id = id + "-panel-" + index;
    button.setAttribute("role", "tab");
    button.setAttribute("aria-controls", panel.id);
    panel.setAttribute("role", "tabpanel");
    panel.setAttribute("aria-labelledby", button.id);
    panel.tabIndex = 0;
    panel.className = "x:rounded x:mt-[1.25em]";
    const heading = document.createElement("h3");
    heading.id = (labels[index] ?? String(index))
      .toLowerCase()
      .replaceAll(" ", "-");
    heading.style.cssText = "visibility:hidden;width:0;height:0";
    heading.textContent = labels[index] ?? "";
    panel.prepend(heading);
    tabs.append(button);
    return button;
  });
  const activate = (index: number, focus = false) => {
    panels.forEach((panel, other) => (panel.hidden = other !== index));
    buttons.forEach((button, other) => {
      button.setAttribute("aria-selected", String(other === index));
      button.tabIndex = other === index ? 0 : -1;
      button.className =
        other === index
          ? "x:whitespace-nowrap x:cursor-pointer x:rounded-t x:p-2 x:font-medium x:leading-5 x:transition-colors x:-mb-0.5 x:select-none x:border-b-2 x:border-current x:outline-none x:text-primary-600"
          : "x:whitespace-nowrap x:cursor-pointer x:rounded-t x:p-2 x:font-medium x:leading-5 x:transition-colors x:-mb-0.5 x:select-none x:border-b-2 x:border-transparent x:text-gray-600 x:dark:text-gray-200";
    });
    if (focus) buttons[index].focus();
  };
  buttons.forEach((button, index) => {
    button.addEventListener("click", () => activate(index));
    button.addEventListener("keydown", (event) => {
      let target: number | undefined;
      if (event.key === "ArrowRight") target = (index + 1) % buttons.length;
      if (event.key === "ArrowLeft")
        target = (index - 1 + buttons.length) % buttons.length;
      if (event.key === "Home") target = 0;
      if (event.key === "End") target = buttons.length - 1;
      if (target !== undefined) {
        event.preventDefault();
        activate(target, true);
      }
    });
  });
  activate(0);
  container.prepend(tabs);
}
const diagrams = document.querySelectorAll<HTMLElement>(".mermaid");
if (diagrams.length) {
  let queue = Promise.resolve();
  const charts = [...diagrams].map((source, index) => {
    const chart = source.textContent ?? "";
    const wrapper = document.createElement("div");
    wrapper.className = "mermaid";
    source.replaceWith(wrapper);
    return { wrapper, chart, id: "docs-diagram-" + index, visible: false };
  });
  const render = (entry: (typeof charts)[number]) => {
    queue = queue
      .then(async () => {
        const { default: mermaid } = await import("mermaid");
        mermaid.initialize({
          startOnLoad: false,
          fontFamily: "inherit",
          themeCSS: "margin: 1.5rem auto 0;",
          theme: document.documentElement.classList.contains("dark")
            ? "dark"
            : "default",
          securityLevel: "strict",
        });
        const { svg } = await mermaid.render(entry.id, entry.chart);
        entry.wrapper.innerHTML = svg;
      })
      .catch((error) => console.error("Documentation diagram failed", error));
  };
  const observer = new IntersectionObserver((entries) => {
    for (const observed of entries)
      if (observed.isIntersecting) {
        const entry = charts.find(
          (chart) => chart.wrapper === observed.target,
        )!;
        if (!entry.visible) {
          entry.visible = true;
          render(entry);
        }
      }
  });
  charts.forEach((entry) => observer.observe(entry.wrapper));
  new MutationObserver(() =>
    charts.filter((entry) => entry.visible).forEach(render),
  ).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  });
}

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    if (
      document.body.classList.contains("nav-open") &&
      !document.querySelector(".docs-choice-menu:not([hidden])")
    ) {
      document.body.classList.remove("nav-open");
      toggle?.setAttribute("aria-expanded", "false");
      toggle?.focus();
    }
    if (results) results.hidden = true;
  }
});

for (const button of document.querySelectorAll<HTMLButtonElement>(
  "[data-docs-toggle]",
))
  button.addEventListener("click", () => {
    const branch = button.nextElementSibling as HTMLElement;
    branch.hidden = !branch.hidden;
    button.setAttribute("aria-expanded", String(!branch.hidden));
    button
      .querySelector("svg")
      ?.classList.toggle("x:ltr:rotate-90", !branch.hidden);
  });
document
  .querySelector<HTMLButtonElement>("#language-switch")
  ?.addEventListener("click", (event) => {
    const href = (event.currentTarget as HTMLElement).dataset.href;
    if (href) location.href = href;
  });
document.querySelector("#copy-page")?.addEventListener("click", () => {
  void navigator.clipboard.writeText(
    JSON.parse(
      document.querySelector("#docs-page-source")?.textContent ?? '""',
    ),
  );
});
document.querySelector("#environment-toggle")?.addEventListener("click", () => {
  const menu = document.querySelector<HTMLElement>("#environment-menu");
  if (menu) menu.hidden = !menu.hidden;
});
for (const button of document.querySelectorAll<HTMLElement>(
  "[data-environment]",
))
  button.addEventListener("click", () => {
    localStorage.setItem("docs-environment", button.dataset.environment!);
    const label = document.querySelector("#environment-label");
    if (label) label.textContent = button.textContent;
    const menu = document.querySelector<HTMLElement>("#environment-menu");
    if (menu) menu.hidden = true;
    window.dispatchEvent(
      new CustomEvent("docs-environment-change", {
        detail: button.dataset.environment,
      }),
    );
  });

const themePath = document.querySelector("#theme-toggle svg path");
const sunPath = themePath?.getAttribute("d");
const moonPath =
  "M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z";
const updateThemeIcon = () => {
  if (themePath && sunPath) {
    themePath.setAttribute(
      "d",
      document.documentElement.classList.contains("dark") ? moonPath : sunPath,
    );
    if (document.documentElement.classList.contains("dark"))
      themePath.setAttribute("stroke-linejoin", "round");
    else themePath.removeAttribute("stroke-linejoin");
    if (document.documentElement.classList.contains("dark"))
      themePath.removeAttribute("stroke-linecap");
    else themePath.setAttribute("stroke-linecap", "round");
  }
};
updateThemeIcon();
document
  .querySelector("#theme-toggle")
  ?.addEventListener("click", updateThemeIcon);
const shortcut = document.querySelector("header kbd");
if (shortcut && !navigator.platform.includes("Mac"))
  shortcut.textContent = "CTRL K";

for (const heading of document.querySelectorAll<HTMLElement>(
  "main h2,main h3,main h4,main h5,main h6",
))
  if (
    heading.id &&
    heading.style.visibility !== "hidden" &&
    !heading.querySelector(".subheading-anchor")
  ) {
    const anchor = document.createElement("a");
    anchor.href = "#" + heading.id;
    anchor.className = "x:focus-visible:nextra-focus subheading-anchor";
    anchor.setAttribute("aria-label", "Permalink for this section");
    heading.append(anchor);
  }

const collapse = document.querySelector<HTMLButtonElement>("#sidebar-collapse");
collapse?.addEventListener("click", () => {
  const collapsed = document.body.classList.toggle("sidebar-collapsed");
  collapse.setAttribute("aria-expanded", String(!collapsed));
  collapse.title = collapsed ? "Expand sidebar" : "Collapse sidebar";
});
document.addEventListener("keydown", (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    input?.focus();
  }
});

document.querySelector("#modules-toggle")?.addEventListener("click", () => {
  const menu = document.querySelector<HTMLElement>("#modules-menu");
  if (menu) menu.hidden = !menu.hidden;
});

for (const button of document.querySelectorAll<HTMLButtonElement>(
  "[data-docs-code-control]",
))
  button.addEventListener("click", () => {
    const pre = button.closest("pre");
    if (!pre) return;
    if (button.title === "Copy code")
      void navigator.clipboard.writeText(
        pre.querySelector("code")?.textContent ?? "",
      );
    else
      pre.style.whiteSpace =
        pre.style.whiteSpace === "pre-wrap" ? "pre" : "pre-wrap";
  });

const themeMenu = document.querySelector<HTMLElement>("#theme-menu");
const themeButton = document.querySelector<HTMLButtonElement>("#theme-toggle");
let themeTrigger = themeButton;
function selectTheme(value: string) {
  const dark =
    value === "dark" ||
    (value === "system" && matchMedia("(prefers-color-scheme:dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
  localStorage.setItem("docs-theme", value);
  updateTheme();
  updateThemeIcon();
  for (const label of document.querySelectorAll<HTMLElement>(
    "[data-mobile-theme-label]",
  ))
    label.textContent = value[0].toUpperCase() + value.slice(1);
  for (const path of document.querySelectorAll<SVGPathElement>(
    "#mobile-theme-toggle svg path",
  )) {
    path.setAttribute("d", dark ? moonPath : sunPath!);
    if (dark) {
      path.setAttribute("stroke-linejoin", "round");
      path.removeAttribute("stroke-linecap");
    } else {
      path.removeAttribute("stroke-linejoin");
      path.setAttribute("stroke-linecap", "round");
    }
  }
  if (themeLabel)
    themeLabel.textContent = value[0].toUpperCase() + value.slice(1);
  updateSelectedChoices();
  if (themeMenu) themeMenu.hidden = true;
  themeTrigger?.setAttribute("aria-expanded", "false");
  themeTrigger?.focus();
}
function openThemeMenu(trigger: HTMLButtonElement) {
  themeTrigger = trigger;
  if (!themeMenu) return;
  themeMenu.hidden = !themeMenu.hidden;
  trigger.setAttribute("aria-expanded", String(!themeMenu.hidden));
  const rect = trigger.getBoundingClientRect();
  themeMenu.style.setProperty("--button-width", rect.width + "px");
  themeMenu.style.left = rect.left + "px";
  themeMenu.style.top =
    Math.max(4, scrollY + rect.top - themeMenu.offsetHeight - 10) + "px";
  themeMenu.querySelector<HTMLElement>('[aria-selected="true"]')?.focus();
}
themeButton?.addEventListener("click", () => openThemeMenu(themeButton));
for (const option of document.querySelectorAll<HTMLElement>(
  "[data-theme-choice]",
)) {
  option.addEventListener("click", () =>
    selectTheme(option.dataset.themeChoice!),
  );
  option.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      selectTheme(option.dataset.themeChoice!);
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const options = [
        ...themeMenu!.querySelectorAll<HTMLElement>("[data-theme-choice]"),
      ];
      options[
        (options.indexOf(option) +
          (event.key === "ArrowDown" ? 1 : options.length - 1)) %
          options.length
      ].focus();
    }
  });
}
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && themeMenu && !themeMenu.hidden) {
    themeMenu.hidden = true;
    themeTrigger?.setAttribute("aria-expanded", "false");
    themeTrigger?.focus();
  }
});

// Native listboxes retain keyboard, dismissal and focus behavior without a framework runtime.
const choiceMenus = [
  ...document.querySelectorAll<HTMLElement>(".docs-choice-menu"),
];
const focusedClasses = [
  "x:bg-primary-100",
  "x:text-primary-800",
  "x:dark:text-primary-500",
  "x:dark:bg-primary-500/10",
];
for (const menu of choiceMenus) {
  for (const option of menu.querySelectorAll<HTMLElement>("[role=option]")) {
    option.addEventListener(
      "focus",
      () => (
        option.classList.remove("x:text-gray-800", "x:dark:text-gray-100"),
        focusedClasses.forEach((c) => option.classList.add(c))
      ),
    );
    option.addEventListener(
      "blur",
      () => (
        option.classList.add("x:text-gray-800", "x:dark:text-gray-100"),
        focusedClasses.forEach((c) => option.classList.remove(c))
      ),
    );
    option.addEventListener("keydown", (event) => {
      const options = [...menu.querySelectorAll<HTMLElement>("[role=option]")];
      const index = options.indexOf(option);
      if (menu.id === "theme-menu") return;
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        options[
          (index + (event.key === "ArrowDown" ? 1 : options.length - 1)) %
            options.length
        ].focus();
      }
      if (event.key === "Home" || event.key === "End") {
        event.preventDefault();
        options[event.key === "Home" ? 0 : options.length - 1].focus();
      }
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        option.click();
      }
    });
  }
}
function closeChoices() {
  for (const menu of choiceMenus) menu.hidden = true;
  for (const button of document.querySelectorAll("[aria-haspopup=listbox]"))
    button.setAttribute("aria-expanded", "false");
}
document.addEventListener("click", (event) => {
  const target = event.target as Element;
  if (!target.closest(".docs-choice-menu,[aria-haspopup=listbox]"))
    closeChoices();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    const opened = choiceMenus.find((menu) => !menu.hidden);
    if (opened) {
      const trigger = document.querySelector<HTMLElement>(
        "[aria-haspopup=listbox][aria-expanded=true]",
      );
      closeChoices();
      if (trigger) {
        trigger.focus();
        return;
      }
      document
        .querySelector<HTMLElement>(
          opened.id === "copy-menu"
            ? "#copy-menu-toggle"
            : opened.id === "language-menu"
              ? "#language-switch"
              : "#theme-toggle",
        )
        ?.focus();
    }
  }
});
const copyToggle =
  document.querySelector<HTMLButtonElement>("#copy-menu-toggle");
const copyMenu = document.querySelector<HTMLElement>("#copy-menu");
copyToggle?.addEventListener("click", () => {
  if (!copyMenu) return;
  copyMenu.hidden = !copyMenu.hidden;
  copyToggle.setAttribute("aria-expanded", String(!copyMenu.hidden));
  const rect = copyToggle.getBoundingClientRect();
  copyMenu.style.left = Math.max(4, rect.right - copyMenu.offsetWidth) + "px";
  copyMenu.style.top = scrollY + rect.bottom + 10 + "px";
  copyMenu.focus();
});
for (const option of document.querySelectorAll<HTMLElement>(
  "[data-copy-choice]",
))
  option.addEventListener("click", () => {
    const value = option.dataset.copyChoice;
    if (value === "copy")
      document.querySelector<HTMLElement>("#copy-page")?.click();
    else {
      const url =
        value === "chatgpt"
          ? "https://chatgpt.com/?hints=search&prompt="
          : "https://claude.ai/new?q=";
      window.open(
        url +
          encodeURIComponent(
            "Read from " + location.href + " so I can ask questions about it.",
          ),
        "_blank",
        "noopener,noreferrer",
      );
    }
    closeChoices();
    copyToggle?.focus();
  });
const backToTop = document.querySelector<HTMLButtonElement>("#back-to-top");
function updateBackToTop() {
  if (!backToTop) return;
  const visible = scrollY > 100;
  backToTop.disabled = !visible;
  backToTop.setAttribute("aria-hidden", String(!visible));
  backToTop.classList.toggle("x:opacity-0", !visible);
  backToTop.toggleAttribute("data-disabled", !visible);
}
backToTop?.addEventListener("click", () =>
  window.scrollTo({
    top: 0,
    behavior: matchMedia("(prefers-reduced-motion: reduce)").matches
      ? "instant"
      : "smooth",
  }),
);
window.addEventListener("scroll", updateBackToTop, { passive: true });
updateBackToTop();
matchMedia("(prefers-color-scheme:dark)").addEventListener("change", () => {
  if (
    (localStorage.getItem("docs-theme") ??
      localStorage.getItem("theme") ??
      "system") === "system"
  )
    selectTheme("system");
});

function updateSelectedChoices() {
  const theme =
    localStorage.getItem("docs-theme") ??
    localStorage.getItem("theme") ??
    "system";
  for (const option of document.querySelectorAll<HTMLElement>(
    "[data-theme-choice],[data-language-href]",
  )) {
    const selected = option.dataset.themeChoice
      ? option.dataset.themeChoice === theme
      : option.getAttribute("aria-selected") === "true";
    option.setAttribute("aria-selected", String(selected));
    for (const c of [
      "x:flex",
      "x:items-center",
      "x:justify-between",
      "x:gap-3",
    ])
      option.classList.toggle(c, selected);
    option.querySelector("[data-choice-check]")?.remove();
    if (selected) {
      const check = document.createElementNS(
        "http://www.w3.org/2000/svg",
        "svg",
      );
      check.setAttribute("data-choice-check", "");
      check.setAttribute("viewBox", "0 0 20 20");
      check.setAttribute("fill", "currentColor");
      check.setAttribute("height", "1em");
      const path = document.createElementNS(
        "http://www.w3.org/2000/svg",
        "path",
      );
      path.setAttribute(
        "d",
        "M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z",
      );
      check.append(path);
      option.append(check);
    }
  }
}
updateSelectedChoices();
function observeChoiceButton(button: HTMLButtonElement) {
  new MutationObserver(() => {
    const open = button.getAttribute("aria-expanded") === "true";
    button.classList.toggle("x:text-gray-600", !open);
    button.classList.toggle("x:dark:text-gray-400", !open);
    for (const c of [
      "x:bg-gray-200",
      "x:text-gray-900",
      "x:dark:bg-primary-100/10",
      "x:dark:text-gray-50",
    ])
      button.classList.toggle(c, open);
  }).observe(button, { attributes: true, attributeFilter: ["aria-expanded"] });
}
for (const button of document.querySelectorAll<HTMLButtonElement>(
  "[aria-haspopup=listbox]",
))
  observeChoiceButton(button);

copyMenu?.addEventListener("keydown", (event) => {
  if (
    event.target === copyMenu &&
    (event.key === "ArrowDown" || event.key === "ArrowUp")
  ) {
    event.preventDefault();
    const options = [
      ...copyMenu.querySelectorAll<HTMLElement>("[role=option]"),
    ];
    options[event.key === "ArrowDown" ? 0 : options.length - 1]?.focus();
  }
});

const choicesPortal = document.createElement("div");
choicesPortal.id = "docs-choices-portal";
document.body.append(choicesPortal);
for (const menu of choiceMenus) {
  const wrapper = document.createElement("div");
  wrapper.append(menu);
  choicesPortal.append(wrapper);
}
let choicesLocked = false;
let previousOverflow = "";
let previousPadding = "";
function synchronizeChoiceScroll() {
  const opened =
    choiceMenus.some((menu) => !menu.hidden) ||
    Boolean(modulesMenu && !modulesMenu.hidden);
  if (opened && !choicesLocked) {
    previousOverflow = document.documentElement.style.overflow;
    previousPadding = document.documentElement.style.paddingRight;
    const gap = innerWidth - document.documentElement.clientWidth;
    document.documentElement.style.overflow = "hidden";
    document.documentElement.style.paddingRight = gap + "px";
    choicesLocked = true;
  } else if (!opened && choicesLocked) {
    document.documentElement.style.overflow = previousOverflow;
    document.documentElement.style.paddingRight = previousPadding;
    choicesLocked = false;
  }
}
for (const menu of choiceMenus)
  new MutationObserver(synchronizeChoiceScroll).observe(menu, {
    attributes: true,
    attributeFilter: ["hidden"],
  });

const modulesMenu = document.querySelector<HTMLElement>("#modules-menu");
const modulesButton =
  document.querySelector<HTMLButtonElement>("#modules-toggle");
modulesButton?.setAttribute("aria-haspopup", "menu");
modulesButton?.addEventListener("click", () => {
  if (!modulesMenu) return;
  const rect = modulesButton.getBoundingClientRect();
  modulesMenu.style.left =
    Math.round(
      Math.max(16, rect.left + (rect.width - modulesMenu.offsetWidth) / 2),
    ) + "px";
  modulesMenu.style.top = scrollY + rect.bottom + 10 + "px";
  modulesButton.setAttribute("aria-expanded", String(!modulesMenu.hidden));
  if (!modulesMenu.hidden) modulesMenu.focus();
});
modulesMenu?.addEventListener("keydown", (event) => {
  const options = [
    ...modulesMenu.querySelectorAll<HTMLElement>("[role=menuitem]"),
  ];
  const index = options.indexOf(document.activeElement as HTMLElement);
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    event.preventDefault();
    options[
      index < 0
        ? event.key === "ArrowDown"
          ? 0
          : options.length - 1
        : (index + (event.key === "ArrowDown" ? 1 : options.length - 1)) %
          options.length
    ]?.focus();
  }
  if (event.key === "Escape") {
    modulesMenu.hidden = true;
    modulesButton?.setAttribute("aria-expanded", "false");
    modulesButton?.focus();
  }
});
document.addEventListener("mousedown", (event) => {
  if (
    modulesMenu &&
    !modulesMenu.contains(event.target as Node) &&
    !modulesButton?.contains(event.target as Node)
  ) {
    modulesMenu.hidden = true;
    modulesButton?.setAttribute("aria-expanded", "false");
  }
});

if (modulesMenu)
  new MutationObserver(synchronizeChoiceScroll).observe(modulesMenu, {
    attributes: true,
    attributeFilter: ["hidden"],
  });

if (modulesMenu) {
  const wrapper = document.createElement("div");
  wrapper.append(modulesMenu);
  choicesPortal.append(wrapper);
  modulesMenu.classList.add("x:nextra-focus");
}

const mobileNav = document.querySelector<HTMLElement>("#docs-mobile-nav");
const mobileContent = mobileNav?.firstElementChild as HTMLElement | undefined;
if (mobileContent) {
  mobileContent.className =
    "x:p-4 x:overflow-y-auto nextra-scrollbar nextra-mask";
  const desktopSearch = document.querySelector<HTMLElement>(".nextra-search");
  if (desktopSearch) {
    const wrapper = document.createElement("div");
    wrapper.className = "x:px-4 x:pt-4";
    const search = desktopSearch.cloneNode(true) as HTMLElement;
    const field = search.querySelector<HTMLInputElement>("input")!;
    field.id = "docs-mobile-search";
    field.addEventListener("input", () => {
      if (input) {
        input.value = field.value;
        input.dispatchEvent(new Event("input"));
      }
    });
    wrapper.append(search);
    mobileNav!.prepend(wrapper);
  }

  const footer = document.createElement("div");
  footer.className =
    "nextra-sidebar-footer x:border-t nextra-border x:flex x:items-center x:gap-2 x:py-4 x:mx-4 x:mt-auto";
  if (themeButton) {
    const mobileTheme = themeButton.cloneNode(true) as HTMLButtonElement;
    mobileTheme.id = "mobile-theme-toggle";
    observeChoiceButton(mobileTheme);
    mobileTheme.classList.add("x:grow");
    const label = mobileTheme.querySelector<HTMLElement>("#theme-label");
    if (label) {
      label.removeAttribute("id");
      label.dataset.mobileThemeLabel = "";
    }
    mobileTheme.addEventListener("click", () => openThemeMenu(mobileTheme));
    footer.append(mobileTheme);
  }
  mobileNav!.append(footer);
}
// Keep the closed mobile drawer out of the keyboard and accessibility navigation.
const syncMobileNavigation = () => {
  const open = document.body.classList.contains("nav-open");
  mobileNav?.toggleAttribute("inert", !open);
  toggle?.setAttribute("aria-expanded", String(open));
  toggle?.querySelector("svg")?.classList.toggle("open", open);
};
syncMobileNavigation();
new MutationObserver(syncMobileNavigation).observe(document.body, {
  attributes: true,
  attributeFilter: ["class"],
});

// Switching native choices dismisses the other popup before the target handler opens its menu.
document.addEventListener(
  "click",
  (event) => {
    const trigger = (event.target as Element).closest<HTMLButtonElement>(
      "button[aria-haspopup=listbox]",
    );
    if (!trigger) return;
    const id = trigger.id.includes("theme")
      ? "theme-menu"
      : trigger.id.includes("language")
        ? "language-menu"
        : "copy-menu";
    for (const menu of choiceMenus) if (menu.id !== id) menu.hidden = true;
    for (const button of document.querySelectorAll<HTMLButtonElement>(
      "[aria-haspopup=listbox]",
    ))
      if (button !== trigger) button.setAttribute("aria-expanded", "false");
  },
  true,
);
