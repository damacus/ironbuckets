// @ts-check
const { test, expect } = require('@playwright/test');

/**
 * User Management E2E Tests
 *
 * Tests the user creation, validation, and modal behavior.
 */

test.describe('User Management Journey', () => {
  test.describe.configure({ mode: 'serial' });
  const ADMIN_USER = process.env.ADMIN_USER || 'minioadmin';
  const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || 'minioadmin';
  const APP_URL = process.env.APP_URL || 'http://localhost:8080';

  /**
   * Helper to open the Add User modal via HTMX
   * Playwright's click() doesn't always trigger HTMX events reliably
   */
  async function openAddUserModal(page) {
    await expect(page.locator('button[hx-get="/users/create"]')).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /Add User/i }).click();

    // Wait for modal to appear
    await page.waitForSelector('#user-modal', { state: 'visible', timeout: 5000 });
  }

  test.beforeEach(async ({ page }) => {
    // Clear cookies to ensure fresh session
    await page.context().clearCookies();

    // Handle any dialogs that might be open
    page.on('dialog', dialog => dialog.dismiss());

    // Login as admin
    await page.goto(`${APP_URL}/login`);
    await page.waitForLoadState('domcontentloaded');

    // Fill in credentials
    await page.fill('input[name="accessKey"]', ADMIN_USER);
    await page.fill('input[name="secretKey"]', ADMIN_PASSWORD);

    // Click submit button
    await page.click('button[type="submit"]');

    // Wait for redirect to dashboard
    await page.waitForURL(`${APP_URL}/`, { timeout: 10000 });
  });

  test('should create new user, logout, and login as new user', async ({ page }) => {
    const newUsername = `testuser-${Date.now()}`;
    const newPassword = 'SecurePassword123!';

    // Step 1: Navigate to Users page
    await ensurePageContent(page, `${APP_URL}/users`, 'button[hx-get="/users/create"]');

    // Step 2: Open "Add User" modal
    await openAddUserModal(page);
    await expect(page.locator('#user-modal h3')).toContainText('Add New User');

    // Step 3: Fill in user details (use specific selectors within modal)
    await page.fill('#user-modal input[name="accessKey"]', newUsername);
    await page.fill('#user-modal input[name="secretKey"]', newPassword);
    await page.selectOption('#user-modal select[name="policy"]', 'readwrite');

    // Step 4: Submit form
    await page.click('#user-modal button[type="submit"]');

    // Wait for redirect back to users page (modal closes automatically)
    await page.waitForURL(`${APP_URL}/users`, { timeout: 5000 });

    // Step 5: Verify modal is closed and new user appears in list
    await expect(page.locator('#user-modal')).not.toBeVisible();
    await expect(page.locator(`text=${newUsername}`)).toBeVisible();

    // Step 6: Logout as admin
    await page.click('a[href="/logout"]');
    await expect(page).toHaveURL(`${APP_URL}/login`);

    // Step 7: Login as newly created user
    await page.waitForLoadState('networkidle');
    await page.fill('input[name="accessKey"]', newUsername);
    await page.fill('input[name="secretKey"]', newPassword);
    await page.click('button[type="submit"]');

    // Wait for redirect to dashboard
    await page.waitForURL(`${APP_URL}/`, { timeout: 10000 });

    // Step 8: Verify successful login
    await expect(page.locator('text=Overview')).toBeVisible();
    await expect(page.locator('a[href="/buckets"]')).toBeVisible();
  });

  test('should handle user creation validation errors', async ({ page }) => {
    // Navigate to Users page
    await ensurePageContent(page, `${APP_URL}/users`, 'button[hx-get="/users/create"]');

    // Open modal
    await openAddUserModal(page);

    // Try to submit empty form
    await page.click('#user-modal button[type="submit"]');

    // HTML5 validation should prevent submission - modal should still be visible
    await expect(page.locator('#user-modal')).toBeVisible();

    // Check that the required field has required attribute
    const accessKeyInput = page.locator('#user-modal input[name="accessKey"]');
    await expect(accessKeyInput).toHaveAttribute('required', '');
  });

  test('should close modal on cancel', async ({ page }) => {
    // Navigate to Users page
    await ensurePageContent(page, `${APP_URL}/users`, 'button[hx-get="/users/create"]');

    // Open modal
    await openAddUserModal(page);

    // Click cancel button
    await page.click('#user-modal button:has-text("Cancel")');

    // Modal should be removed from DOM
    await expect(page.locator('#user-modal')).not.toBeVisible();
  });

  test('should close modal when clicking backdrop', async ({ page }) => {
    // Navigate to Users page
    await ensurePageContent(page, `${APP_URL}/users`, 'button[hx-get="/users/create"]');

    // Open modal
    await openAddUserModal(page);

    // Click backdrop - must click on the backdrop itself, not the inner modal content
    // The modal backdrop is the #user-modal div itself, which is full-screen
    // The inner content is centered, so clicking at coordinates (10, 10) hits the backdrop
    const modal = page.locator('#user-modal');
    const box = await modal.boundingBox();
    if (!box) throw new Error('Modal bounding box not found');

    // Click in the top-left corner of the backdrop (outside the centered inner modal)
    await page.mouse.click(box.x + 10, box.y + 10);

    // Modal should be removed (onclick: if(event.target === this) this.remove())
    await expect(page.locator('#user-modal')).not.toBeVisible();
  });
});
  async function ensurePageContent(page, url, readySelector) {
    await page.goto(url);
    await page.waitForLoadState('networkidle');
    let text = (await page.locator('#main-content').innerText().catch(() => '')).trim();
    if (!text) {
      await page.reload();
      await page.waitForLoadState('networkidle');
    }
    await expect(page.locator(readySelector)).toBeVisible({ timeout: 10000 });
  }
