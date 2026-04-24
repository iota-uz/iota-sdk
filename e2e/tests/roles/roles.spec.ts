import { test, expect, type Page } from '@playwright/test';
import { login, logout, waitForAlpine } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

/**
 * Submit the #delete-form via htmx programmatically.
 * Works around Playwright's hit-test issue with `<dialog>` top-layer in headless Chromium
 * where the sticky bottom action bar intercepts pointer events on the confirm button.
 */
async function submitDeleteFormViaHtmx(page: Page): Promise<void> {
  const response = await page.evaluate(async () => {
    const form = document.getElementById('delete-form');
    let endpoint = '';
    const params = new URLSearchParams();

    if (form instanceof HTMLFormElement && form.getAttribute('hx-delete')) {
      endpoint = form.getAttribute('hx-delete') || '';
      for (const [key, value] of new FormData(form).entries()) {
        params.append(key, String(value));
      }
    } else {
      endpoint = window.location.pathname;
      const csrfInput = document.querySelector<HTMLInputElement>(
        'input[name="gorilla.csrf.Token"]',
      );
      if (csrfInput?.value) {
        params.append('gorilla.csrf.Token', csrfInput.value);
      }
    }

    if (!endpoint) throw new Error('delete endpoint not found');

    const res = await fetch(endpoint, {
      method: 'DELETE',
      credentials: 'same-origin',
      headers: {
        'HX-Request': 'true',
        'X-Requested-With': 'XMLHttpRequest',
        'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
      },
      body: params.toString(),
    });

    const body = await res.text().catch(() => '');

    return {
      endpoint,
      ok: res.ok,
      status: res.status,
      redirect:
        res.headers.get('Hx-Redirect') ||
        res.headers.get('HX-Redirect') ||
        (res.redirected ? res.url : null),
      location: res.headers.get('Location'),
      body: body.slice(0, 500),
    };
  });
  expect(
    response.ok,
    `DELETE ${response.endpoint} failed with status ${response.status}; location=${
      response.location ?? 'none'
    }; body=${response.body}`,
  ).toBeTruthy();
  if (response.redirect) {
    await page.goto(response.redirect, { waitUntil: 'domcontentloaded' });
  }
}

async function ensureRolesListVisible(page: Page): Promise<void> {
  await expect(page).toHaveURL(/\/roles$/, { timeout: 15000 });
  await expect(page.locator('tbody')).toBeVisible({ timeout: 15000 });
}

async function setCheckboxState(
  locator: ReturnType<Page['locator']>,
  checked: boolean,
): Promise<void> {
  await locator.evaluate((element, nextChecked) => {
    if (!(element instanceof HTMLInputElement)) {
      throw new Error('expected checkbox input');
    }

    element.checked = nextChecked;
    element.dispatchEvent(new Event('change', { bubbles: true }));
  }, checked);
}

async function setMultiSelectByLabel(
  locator: ReturnType<Page['locator']>,
  labels: string[],
): Promise<void> {
  await locator.evaluate((element, nextLabels) => {
    if (!(element instanceof HTMLSelectElement)) {
      throw new Error('expected select element');
    }

    const labelSet = new Set(nextLabels);
    let matched = 0;

    for (const option of Array.from(element.options)) {
      const shouldSelect = labelSet.has(option.text.trim());
      option.selected = shouldSelect;
      if (shouldSelect) {
        matched++;
      }
    }

    if (matched !== labelSet.size) {
      throw new Error(
        `expected to match ${labelSet.size} select options, matched ${matched}`,
      );
    }

    element.dispatchEvent(new Event('input', { bubbles: true }));
    element.dispatchEvent(new Event('change', { bubbles: true }));
  }, labels);
}

test.describe('role management flows', () => {
  // Tests MUST run serially - some tests depend on data created by previous tests
  test.describe.configure({ mode: 'serial' });
  const userReadPermissionID = '13f011c8-1107-4957-ad19-70cfc167a775';
  const userUpdatePermissionID = '1c351fd3-9a2b-40b9-80b1-11ba81e645c8';

  const saveRoleButton = (page: Page) =>
    page
      .getByRole('button', { name: /save/i })
      .or(page.locator('[data-test-id="save-role-btn"], #save-btn'));

  // Reset database once for entire suite
  test.beforeAll(async ({ request }) => {
    await resetTestDatabase(request, { reseedMinimal: false });
    await seedScenario(request, 'comprehensive');
  });

  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
  });

  test.afterEach(async ({ page }) => {
    await logout(page);
  });

  test('complete role lifecycle: create, edit, and delete', async ({
    page,
  }) => {
    // Login as admin user
    await login(page, 'test@gmail.com', 'TestPass123!');

    // Navigate to roles page
    await page.goto('/roles');
    await expect(page).toHaveURL(/\/roles$/);

    // Wait for roles page to be ready and new button to be visible and clickable
    const newRoleBtn = page.locator('[data-test-id="new-role-btn"]');
    await expect(newRoleBtn).toBeVisible({ timeout: 20000 });
    await expect(newRoleBtn).toBeEnabled();

    // Count initial roles
    const initialRoleCount = await page
      .locator('tbody tr:not(.hidden)')
      .count();

    // Click new role button
    await newRoleBtn.click();
    await expect(page).toHaveURL(/\/roles\/new$/);

    // Verify form elements are present
    await expect(
      page.locator('[data-test-id="role-name-input"]'),
    ).toBeVisible();
    await expect(
      page.locator('[data-test-id="role-description-input"]'),
    ).toBeVisible();
    // Save button (by role or test-id); may be in sticky footer below fold
    const saveBtn = saveRoleButton(page).first();
    await saveBtn.waitFor({ state: 'attached', timeout: 15000 });
    await saveBtn.scrollIntoViewIfNeeded();
    await expect(saveBtn).toBeVisible();

    // Fill in role details
    const testRoleName = 'Test Editor Role';
    const testRoleDescription = 'Can view and edit users';
    await page.locator('[data-test-id="role-name-input"]').fill(testRoleName);
    await page
      .locator('[data-test-id="role-description-input"]')
      .fill(testRoleDescription);

    // Verify input values were set correctly
    await expect(page.locator('[data-test-id="role-name-input"]')).toHaveValue(
      testRoleName,
    );
    await expect(
      page.locator('[data-test-id="role-description-input"]'),
    ).toHaveValue(testRoleDescription);

    // Wait for permission UI to load
    await waitForAlpine(page);

    // Find and click on a permission toggle (first available permission set)
    // The permission sets are rendered as switches in the form
    const firstPermissionSwitch = page
      .locator('input[type="checkbox"][name^="Permissions"]')
      .first();
    if (await firstPermissionSwitch.isVisible()) {
      await firstPermissionSwitch.check();
      // Verify the checkbox was checked
      await expect(firstPermissionSwitch).toBeChecked();
    }

    // Save the role (scroll into view again in case viewport changed)
    await saveBtn.scrollIntoViewIfNeeded();
    await saveBtn.click();

    // Wait for redirect back to roles list
    await ensureRolesListVisible(page);

    // Verify role appears in list
    const createdRoleRow = page
      .locator('tbody tr')
      .filter({ hasText: testRoleName });
    await expect(createdRoleRow).toBeVisible();

    // Verify role count increased
    const newRoleCount = await page.locator('tbody tr:not(.hidden)').count();
    expect(newRoleCount).toBe(initialRoleCount + 1);

    // Edit the role - find the edit button for our new role
    await createdRoleRow.locator('a').first().click();
    await expect(page).toHaveURL(/\/roles\/\d+$/);

    // Verify the saved values are loaded in the edit form
    await expect(page.locator('[data-test-id="role-name-input"]')).toHaveValue(
      testRoleName,
    );
    await expect(
      page.locator('[data-test-id="role-description-input"]'),
    ).toHaveValue(testRoleDescription);

    // Verify delete button is present on edit page
    await expect(
      page.locator('[data-test-id="delete-role-btn"]'),
    ).toBeVisible();

    // Update role name
    const updatedRoleName = 'Updated Editor Role';
    await page
      .locator('[data-test-id="role-name-input"]')
      .fill(updatedRoleName);
    await expect(page.locator('[data-test-id="role-name-input"]')).toHaveValue(
      updatedRoleName,
    );

    // Save changes
    await saveRoleButton(page).first().click();
    await ensureRolesListVisible(page);

    // Verify name was updated in the list
    await expect(
      page.locator('tbody tr').filter({ hasText: updatedRoleName }),
    ).toBeVisible();
    await expect(
      page.locator('tbody tr').filter({ hasText: testRoleName }),
    ).not.toBeVisible();

    // Delete the role
    const updatedRoleRow = page
      .locator('tbody tr')
      .filter({ hasText: updatedRoleName });
    await updatedRoleRow.locator('a').first().click();
    await expect(page).toHaveURL(/\/roles\/\d+$/);

    // Verify we're on the correct edit page
    await expect(page.locator('[data-test-id="role-name-input"]')).toHaveValue(
      updatedRoleName,
    );

    // Click delete button
    await page.locator('[data-test-id="delete-role-btn"]').click();

    // Submit the delete via the underlying HTMX form. The dialog visibility
    // can lag in headless Chromium, but the hidden form is the actual action surface.
    await submitDeleteFormViaHtmx(page);

    // Wait for redirect back to roles list
    await ensureRolesListVisible(page);

    // Verify role was deleted from list
    await expect(
      page.locator('tbody tr').filter({ hasText: updatedRoleName }),
    ).not.toBeVisible();

    // Verify role count returned to initial
    const finalRoleCount = await page.locator('tbody tr:not(.hidden)').count();
    expect(finalRoleCount).toBe(initialRoleCount);
  });

  test('permission UI shows only modules with resources (regression test for #498)', async ({
    page,
  }) => {
    await login(page, 'test@gmail.com', 'TestPass123!');

    // Navigate to create role page
    await page.goto('/roles/new', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/roles\/new$/);

    // Wait for Alpine.js and permission UI to load
    await waitForAlpine(page);

    // Verify that module tabs exist (at least one should be visible)
    // Module tabs are rendered with tab buttons
    const moduleTabs = page.locator(
      '[role="tablist"] button, [data-tab-value]',
    );
    const tabCount = await moduleTabs.count();

    // There should be at least one module tab (Core module at minimum)
    expect(tabCount).toBeGreaterThanOrEqual(1);

    // Get all module content containers that have the test ID
    const moduleContentContainers = page.locator(
      '[data-test-id^="module-content-"]',
    );

    // Verify the first/default tab has content (not empty)
    // The first tab should already be selected and showing content
    // This verifies that modules with empty ResourceGroups are not displayed (fix for #498)

    // Wait for module content to be visible
    const defaultModuleContent = moduleContentContainers.first();
    await expect(defaultModuleContent).toBeVisible({
      timeout: 5000,
    });

    // The content should have at least one permission-related element
    const permissionInputsInTab = defaultModuleContent.locator(
      'input[name^="Permissions"]',
    );
    const inputCount = await permissionInputsInTab.count();

    // The visible module tab MUST have at least one permission checkbox
    // If it has zero, that's the bug from #498 (empty module being displayed)
    expect(
      inputCount,
      'Default module tab should have permission inputs',
    ).toBeGreaterThan(0);

    // Verify the total permission count across all modules
    const allPermissionInputs = page.locator('input[name^="Permissions"]');
    const totalPermissionCount = await allPermissionInputs.count();

    // Should have meaningful permissions (at least basic CRUD for users/roles)
    expect(totalPermissionCount).toBeGreaterThanOrEqual(4);
  });

  test('system roles are protected from modification', async ({ page }) => {
    await login(page, 'test@gmail.com', 'TestPass123!');

    // Navigate to roles page
    await page.goto('/roles', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/roles$/);

    // Verify the roles table is visible and has content
    const rolesTable = page.locator('tbody');
    await expect(rolesTable).toBeVisible();

    // Count total roles in the table
    const roleRows = page.locator('tbody tr:not(.hidden)');
    const totalRoles = await roleRows.count();
    expect(totalRoles).toBeGreaterThan(0);

    // Find a system role row - system roles typically have disabled edit buttons
    // The Administrator role is usually a system role
    const systemRoleRow = page
      .locator('tbody tr')
      .filter({ hasText: /Administrator|System/i })
      .first();

    // Check if there's a system role
    if (await systemRoleRow.isVisible()) {
      // Verify the system role row contains expected content
      await expect(systemRoleRow).toContainText(/Administrator|System/i);

      // The edit button should be disabled for system roles
      const editButton = systemRoleRow.locator('a, button').first();
      await expect(editButton).toBeVisible();

      // Check if button is disabled using multiple possible indicators
      const isDisabled =
        (await editButton.isDisabled()) ||
        (await editButton.getAttribute('aria-disabled')) === 'true' ||
        (await editButton.getAttribute('disabled')) !== null;

      // System roles MUST have their edit buttons disabled
      expect(isDisabled, 'System role edit button should be disabled').toBe(
        true,
      );

      // Additionally verify the button has disabled styling or state
      const buttonClasses = (await editButton.getAttribute('class')) || '';
      const hasDisabledAppearance =
        isDisabled ||
        buttonClasses.includes('disabled') ||
        buttonClasses.includes('opacity') ||
        buttonClasses.includes('cursor-not-allowed');

      expect(
        hasDisabledAppearance,
        'System role button should have disabled appearance',
      ).toBe(true);
    } else {
      // If no system role exists in the test data, this test is skipped
      // This is acceptable as the test depends on seed data
      test.skip();
    }
  });

  test('user with limited role can only access permitted resources (RBAC flow)', async ({
    page,
    request,
  }) => {
    const limitedRoleName = 'Limited Reader Role';
    const limitedUserEmail = 'limited@test.com';
    const limitedUserPassword = 'TestPass123!';

    // Login as admin to create a limited role and user
    await login(page, 'test@gmail.com', 'TestPass123!');

    // Create a limited role with only User.Read permission
    await page.goto('/roles/new', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/roles\/new$/);

    await page
      .locator('[data-test-id="role-name-input"]')
      .fill(limitedRoleName);
    await page
      .locator('[data-test-id="role-description-input"]')
      .fill('Can only view users');

    // Verify inputs were set
    await expect(page.locator('[data-test-id="role-name-input"]')).toHaveValue(
      limitedRoleName,
    );

    // Wait for permissions to load
    await waitForAlpine(page);

    // Find and enable only the User Read permission
    const coreModuleContent = page.locator(
      '[data-test-id="module-content-Core"]',
    );
    await expect(coreModuleContent).toBeVisible();

    const userReadCheckbox = coreModuleContent
      .locator(`input[name="Permissions[${userReadPermissionID}]"]`)
      .first();
    await expect(userReadCheckbox).toHaveCount(1);
    await setCheckboxState(userReadCheckbox, true);
    await expect(userReadCheckbox).toBeChecked();

    const userUpdateCheckbox = coreModuleContent
      .locator(`input[name="Permissions[${userUpdatePermissionID}]"]`)
      .first();
    await expect(userUpdateCheckbox).toHaveCount(1);
    await expect(userUpdateCheckbox).not.toBeChecked();

    // Save the role
    await saveRoleButton(page).first().click();
    await page.waitForURL(/\/roles$/);

    // Verify role was created and appears in the table
    const createdRoleRow = page
      .locator('tbody tr')
      .filter({ hasText: limitedRoleName });
    await expect(createdRoleRow).toBeVisible();

    // Verify role row contains the role name
    await expect(createdRoleRow).toContainText(limitedRoleName);

    // Create a new user with this limited role
    await page.goto('/users/new');
    await expect(page).toHaveURL(/\/users\/new$/);

    // Fill in user details
    await page.locator('[name=FirstName]').fill('Limited');
    await page.locator('[name=LastName]').fill('User');
    await page.locator('[name=Email]').fill(limitedUserEmail);
    await page.locator('[name=Phone]').fill('+998901112233');
    await page.locator('[name=Password]').fill(limitedUserPassword);
    // Select first enabled option (index 0 might be a disabled placeholder)
    const languageSelect = page.locator('[name=Language]');
    const enabledOptions = languageSelect.locator('option:not([disabled])');
    const firstEnabledValue = await enabledOptions
      .first()
      .getAttribute('value');
    if (firstEnabledValue) {
      await languageSelect.selectOption(firstEnabledValue);
    }

    // Verify user form fields
    await expect(page.locator('[name=FirstName]')).toHaveValue('Limited');
    await expect(page.locator('[name=LastName]')).toHaveValue('User');
    await expect(page.locator('[name=Email]')).toHaveValue(limitedUserEmail);

    // Select the Limited Reader Role directly on the underlying multi-select.
    const roleSelect = page.locator('select[name="RoleIDs"]');
    await expect(roleSelect).toHaveCount(1);
    await setMultiSelectByLabel(roleSelect, [limitedRoleName]);

    // Save the user
    await page.locator('[id=save-btn]').click();

    // Wait for redirect to users page or handle login redirect
    // The newly created user might not have sufficient permissions causing a redirect to login
    await page.waitForURL(/\/(users|login)$/);

    // Check where we ended up
    const currentUrl = page.url();
    if (currentUrl.includes('/login')) {
      // If redirected to login, there was likely a session or permission issue
      // Re-login as admin to continue the test
      await login(page, 'test@gmail.com', 'TestPass123!');
      await page.goto('/users');
    }

    // Verify user was created in the list
    const createdUserRow = page
      .locator('tbody tr')
      .filter({ hasText: 'Limited User' });
    await expect(createdUserRow).toBeVisible();
    await expect(createdUserRow).toContainText('Limited');

    // Logout admin
    await logout(page);

    // Login as the limited user
    await login(page, limitedUserEmail, limitedUserPassword);

    // Verify login was successful - should be redirected to dashboard or main page
    // (not stuck on login page)
    await expect(page).not.toHaveURL(/\/login/);

    // The limited user should have explicit read access to users.
    await page.goto('/users', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/users$/);
    await expect(page.locator('table')).toBeVisible();

    // And they should not have access to a different resource like roles.
    const rolesResponse = await page.goto('/roles', {
      waitUntil: 'domcontentloaded',
    });
    const rolesTableVisible = await page
      .locator('tbody')
      .isVisible()
      .catch(() => false);
    const forbiddenVisible = await page
      .getByText(/forbidden/i)
      .isVisible()
      .catch(() => false);
    const redirectedAwayFromRoles = !/\/roles$/.test(page.url());
    const forbiddenStatus = rolesResponse?.status() === 403;

    expect(
      rolesTableVisible,
      'limited role should not be able to view the roles list',
    ).toBe(false);
    expect(
      forbiddenStatus || forbiddenVisible || redirectedAwayFromRoles,
      `limited role should receive a forbidden response or redirect away from roles; status=${
        rolesResponse?.status() ?? 'none'
      } url=${page.url()}`,
    ).toBe(true);

    // Clean up: Login back as admin and delete the test role and user
    await logout(page);
    await login(page, 'test@gmail.com', 'TestPass123!');

    // Delete the limited user
    await page.goto('/users');
    const limitedUserRow = page
      .locator('tbody tr')
      .filter({ hasText: 'Limited User' });
    if (await limitedUserRow.isVisible()) {
      await limitedUserRow
        .locator('a[href*="/users/"][href$="/edit"]')
        .first()
        .click();
      await expect(page).toHaveURL(/\/users\/\d+\/edit$/);
      await expect(page.locator('#delete-user-btn')).toBeEnabled();

      // Trigger htmx delete directly — this is cleanup, not testing the dialog
      await submitDeleteFormViaHtmx(page);
      await page.waitForURL(/\/users$/);
    }

    // Verify user was deleted
    await expect(
      page.locator('tbody tr').filter({ hasText: 'Limited User' }),
    ).not.toBeVisible();

    // Delete the limited role
    await page.goto('/roles');
    const limitedRoleRow = page
      .locator('tbody tr')
      .filter({ hasText: limitedRoleName });
    if (await limitedRoleRow.isVisible()) {
      await limitedRoleRow.locator('a').first().click();
      await page.locator('[data-test-id="delete-role-btn"]').click();

      // Wait for dialog and trigger htmx delete directly
      const confirmDialog = page.locator(
        '[data-test-id="delete-confirmation-dialog"]',
      );
      await expect(confirmDialog).toBeVisible();
      await submitDeleteFormViaHtmx(page);

      // Wait for redirect back to roles list
      await page.waitForURL(/\/roles$/);
    }

    // Verify role was deleted
    await expect(
      page.locator('tbody tr').filter({ hasText: limitedRoleName }),
    ).not.toBeVisible();
  });
});
