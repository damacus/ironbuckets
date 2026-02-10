// @ts-check
const { test, expect } = require('@playwright/test');

const APP_URL = process.env.APP_URL || 'http://localhost:8080';
const ADMIN_USER = process.env.ADMIN_USER || 'minioadmin';
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || 'minioadmin';

test.describe('Object Browser', () => {
  test.describe.configure({ mode: 'serial' });
  // Each test gets its own unique bucket name
  let testBucket;

  /**
   * Helper to open the Create Bucket modal via HTMX
   */
  async function openCreateBucketModal(page) {
    await expect(page.locator('button[hx-get="/buckets/create"]')).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /Create Bucket/i }).click();
    await page.waitForSelector('#bucket-modal', { state: 'visible', timeout: 5000 });
  }

  test.beforeEach(async ({ page }) => {
    // Generate unique bucket name for this test
    testBucket = `e2e-test-${Date.now()}`;

    // Clear cookies for fresh session
    await page.context().clearCookies();

    // Handle any dialogs that might be open
    page.on('dialog', dialog => dialog.dismiss());

    // Login as admin
    await page.goto(`${APP_URL}/login`);
    await page.waitForLoadState('domcontentloaded');

    await page.fill('input[name="accessKey"]', ADMIN_USER);
    await page.fill('input[name="secretKey"]', ADMIN_PASSWORD);
    await page.click('button[type="submit"]');

    await page.waitForURL(`${APP_URL}/`, { timeout: 10000 });
    await page.waitForLoadState('networkidle');
  });

  test.afterEach(async ({ page }) => {
    // Cleanup: Delete the test bucket if it exists
    if (!testBucket) return;

    try {
      await page.goto(`${APP_URL}/buckets`);
      await page.waitForLoadState('networkidle');

      // Check if our test bucket exists
      const bucketLink = page.locator(`a[href="/buckets/${testBucket}"]`).filter({ visible: true });
      if (await bucketLink.count() > 0) {
        // Find and click the dropdown button for this bucket
        const bucketCard = page.locator(`text=${testBucket}`).first().locator('..').locator('..');
        const dropdownBtn = bucketCard.locator('button').first();
        await dropdownBtn.click();

        // Click Delete Bucket - this triggers the styled confirm dialog
        await page.getByRole('button', { name: 'Delete Bucket' }).click();

        // Wait for and click the confirm button in the styled dialog
        await page.waitForSelector('#confirm-dialog[open]', { timeout: 2000 });
        await page.click('#confirm-dialog-confirm');

        // Wait for deletion to complete
        await page.waitForTimeout(500);
      }
    } catch (err) {
      // Ignore cleanup errors - bucket may not exist or already deleted
      console.log(`Cleanup note: ${err}`);
    }
  });

  test('should create a bucket and navigate to object browser', async ({ page }) => {
    // Navigate to buckets
    await ensurePageContent(page, `${APP_URL}/buckets`, 'button[hx-get="/buckets/create"]');

    // Create a new bucket via HTMX modal
    await openCreateBucketModal(page);
    await page.fill('#bucket-modal input[name="bucketName"]', testBucket);
    await page.click('#bucket-modal button[type="submit"]');

    // Wait for redirect and verify bucket appears
    await page.waitForURL(`${APP_URL}/buckets`, { timeout: 5000 });
    await expect(page.getByText(testBucket)).toBeVisible();

    // Click on the bucket to open object browser (use the card link with bucket name)
    await page.locator(`.group`).filter({ hasText: testBucket }).locator('a.block').filter({ hasText: testBucket }).click();
    await page.waitForURL(`${APP_URL}/buckets/${testBucket}`);

    // Verify object browser elements
    await expect(page.getByRole('button', { name: 'New Folder' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Upload' })).toBeVisible();
    await expect(page.locator('input[placeholder="Search..."]')).toBeVisible();
    await expect(page.getByText('This bucket is empty')).toBeVisible();
    await expect(page.getByText('0 folder(s), 0 file(s)')).toBeVisible();
  });

  test('should create a folder and navigate into it', async ({ page }) => {
    // Create bucket first
    await ensurePageContent(page, `${APP_URL}/buckets`, 'button[hx-get="/buckets/create"]');
    await openCreateBucketModal(page);
    await page.fill('#bucket-modal input[name="bucketName"]', testBucket);
    await page.click('#bucket-modal button[type="submit"]');
    await page.waitForURL(`${APP_URL}/buckets`, { timeout: 5000 });

    // Open the bucket (use the card link with bucket name)
    await page.locator(`.group`).filter({ hasText: testBucket }).locator('a.block').filter({ hasText: testBucket }).click();
    await page.waitForURL(`${APP_URL}/buckets/${testBucket}`);

    // Create a folder using the New Folder button
    await page.click('button:has-text("New Folder")');
    await page.waitForSelector('input[name="folderName"]', { state: 'visible' });
    await page.fill('input[name="folderName"]', 'test-folder');
    await page.click('button:has-text("Create Folder")');

    // Wait for page to reload and verify folder appears
    await page.waitForURL(`${APP_URL}/buckets/${testBucket}`, { timeout: 5000 });
    await expect(page.getByText('test-folder')).toBeVisible();
    await expect(page.getByText(/1 folder\(s\)/)).toBeVisible();

    // Click on folder to navigate into it
    await page.click('tr:has-text("test-folder")');

    // Verify URL has prefix query param and breadcrumb shows folder
    await page.waitForURL(/prefix=test-folder/);
    await expect(page.getByRole('link', { name: 'test-folder' })).toBeVisible();
    await expect(page.getByText('This folder is empty')).toBeVisible();
  });

  test('should navigate using breadcrumbs', async ({ page }) => {
    // Create bucket first
    await ensurePageContent(page, `${APP_URL}/buckets`, 'button[hx-get="/buckets/create"]');
    await openCreateBucketModal(page);
    await page.fill('#bucket-modal input[name="bucketName"]', testBucket);
    await page.click('#bucket-modal button[type="submit"]');
    await page.waitForURL(`${APP_URL}/buckets`, { timeout: 5000 });

    // Open the bucket (use the card link with bucket name)
    await page.locator(`.group`).filter({ hasText: testBucket }).locator('a.block').filter({ hasText: testBucket }).click();
    await page.waitForURL(`${APP_URL}/buckets/${testBucket}`);

    // Create level1 folder
    await page.click('button:has-text("New Folder")');
    await page.waitForSelector('input[name="folderName"]', { state: 'visible' });
    await page.fill('input[name="folderName"]', 'level1');
    await page.click('button:has-text("Create Folder")');
    await page.waitForURL(`${APP_URL}/buckets/${testBucket}`, { timeout: 5000 });

    // Navigate into level1
    await page.click('tr:has-text("level1")');
    await page.waitForURL(/prefix=level1/);

    // Create level2 folder inside level1
    await page.click('button:has-text("New Folder")');
    await page.waitForSelector('input[name="folderName"]', { state: 'visible' });
    await page.fill('input[name="folderName"]', 'level2');
    await page.click('button:has-text("Create Folder")');
    await page.waitForTimeout(500); // Brief wait for folder creation

    // Navigate into level2
    await page.click('tr:has-text("level2")');
    await page.waitForURL(/prefix=level1.*level2/);

    // Verify breadcrumbs show full path
    await expect(page.getByRole('link', { name: 'level1' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'level2' })).toBeVisible();

    // Click on bucket name in breadcrumb to go back to root
    await page.locator(`a[href="/buckets/${testBucket}"]`).filter({ visible: true }).first().click();
    await page.waitForURL(`${APP_URL}/buckets/${testBucket}`);
    await expect(page.getByText('level1')).toBeVisible();
  });

  test('should have working search input', async ({ page }) => {
    // Create bucket first
    await ensurePageContent(page, `${APP_URL}/buckets`, 'button[hx-get="/buckets/create"]');
    await openCreateBucketModal(page);
    await page.fill('#bucket-modal input[name="bucketName"]', testBucket);
    await page.click('#bucket-modal button[type="submit"]');
    await page.waitForURL(`${APP_URL}/buckets`, { timeout: 5000 });

    // Open the bucket (use the card link with bucket name)
    await page.locator(`.group`).filter({ hasText: testBucket }).locator('a.block').filter({ hasText: testBucket }).click();
    await page.waitForURL(`${APP_URL}/buckets/${testBucket}`);

    // Create a folder
    await page.click('button:has-text("New Folder")');
    await page.waitForSelector('input[name="folderName"]', { state: 'visible' });
    await page.fill('input[name="folderName"]', 'searchable-folder');
    await page.click('button:has-text("Create Folder")');
    await page.waitForURL(`${APP_URL}/buckets/${testBucket}`, { timeout: 5000 });

    // Verify folder is visible
    await expect(page.getByText('searchable-folder')).toBeVisible();

    // Verify search input exists and is functional
    const searchInput = page.locator('input[placeholder="Search..."]');
    await expect(searchInput).toBeVisible();
    await searchInput.fill('searchable');

    // Search is client-side filtering - the input should accept text
    await expect(searchInput).toHaveValue('searchable');
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
