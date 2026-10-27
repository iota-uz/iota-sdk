import { expect, test } from "@playwright/test";

// Falsely green if text alone renders: cross real navigation, built search, browser Mermaid and persisted theme boundaries.
test("static documentation preserves navigation, search, diagrams and theme", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/iota-sdk/");
  await expect(page.locator("main h1")).toHaveText("IOTA SDK Documentation");
  const nav = page.locator("#docs-nav");
  await nav.getByRole("button", { name: "Architecture", exact: true }).click();
  await nav.getByRole("link", { name: "Overview", exact: true }).last().click();
  await expect(page).toHaveURL(/\/iota-sdk\/architecture(?:\.html)?$/);
  await expect(page.locator("main h1")).toHaveText(/Architecture/);
  await expect(page.locator(".mermaid svg").first()).toBeVisible({
    timeout: 30000,
  });
  await page.locator("#theme-toggle").click();
  await page.getByRole("option", { name: "Dark", exact: true }).click();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.reload();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.locator("#docs-search").fill("tenant");
  const results = page.locator("#search-results a");
  await expect(results.first()).toBeVisible();
  expect(await results.first().getAttribute("href")).toContain("/iota-sdk/");
  await results.first().click();
  await expect(page.locator("main h1")).toBeVisible();
  expect(errors).toEqual([]);
});

test("narrow documentation navigation remains keyboard reachable", async ({
  page,
}) => {
  await page.setViewportSize({ width: 393, height: 852 });
  await page.goto("/iota-sdk/");
  const toggle = page.getByRole("button", { name: "Toggle navigation" });
  await toggle.focus();
  await page.keyboard.press("Enter");
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator("#docs-mobile-nav")).toBeVisible();
  const nav = page.locator("#docs-mobile-nav");
  await nav.getByRole("button", { name: "Architecture", exact: true }).click();
  await nav.getByRole("link", { name: "Overview", exact: true }).last().click();
  await expect(page.locator("main h1")).toHaveText(/Architecture/);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

// Falsely green if controls render without handlers: drive listboxes by keyboard and verify the actual clipboard.
test("native documentation choices and Markdown copy", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/iota-sdk/architecture");
  await page.locator("#theme-toggle").click();
  await page.getByRole("option", { name: "Light", exact: true }).press("Enter");
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await page.locator("#copy-menu-toggle").click();
  const menu = page.getByRole("listbox", { name: "Copy page options" });
  await expect(menu).toBeVisible();
  await menu.press("ArrowDown");
  await page
    .getByRole("option", {
      name: "Copy page Copy page as Markdown for LLMs",
      exact: true,
    })
    .press("Enter");
  await expect(menu).not.toBeVisible();
  const copied = await page.evaluate(() => navigator.clipboard.readText());
  expect(copied).toContain("##");
  expect(copied).toContain("```");
  await page.locator("#theme-toggle").click();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("listbox", { name: "Theme" })).not.toBeVisible();
  await expect(page.locator("#theme-toggle")).toBeFocused();
});

// Falsely green if the drawer only opens: choose a theme by keyboard and cross a real navigation boundary.
test("mobile theme choices preserve state", async ({ page }) => {
  await page.setViewportSize({ width: 393, height: 852 });
  await page.goto("/iota-sdk/");
  await page.locator("#nav-toggle").click();
  await page.locator("#mobile-theme-toggle").click();
  await page.getByRole("option", { name: "Dark", exact: true }).press("Enter");
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(page.locator("#mobile-theme-toggle")).toHaveText(/Dark/);
  await expect(page.locator("#mobile-theme-toggle")).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  await page.locator("#mobile-theme-toggle").click();
  await page.keyboard.press("Escape");
  await expect(page.locator("#mobile-theme-toggle")).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  await expect(page.locator("#mobile-theme-toggle")).toBeFocused();
  await page.locator("#nav-toggle").click();
  await expect(page.locator("#docs-mobile-nav")).toHaveAttribute("inert", "");
  await page.reload();
  await expect(page.locator("html")).toHaveClass(/dark/);
});

// Falsely green if the menu merely renders: follow its prefixed URL and verify the destination page.
test("module menu follows the SDK base path", async ({ page }) => {
  await page.goto("/iota-sdk/");
  await page.locator("#modules-toggle").click();
  const item = page.getByRole("menuitem", { name: "Core Module", exact: true });
  await expect(item).toHaveAttribute("href", "/iota-sdk/core");
  await item.click();
  await expect(page).toHaveURL(/\/iota-sdk\/core$/);
  await expect(page.locator("main h1")).toHaveText("Core Module");
});
