// @ts-check
const { test, expect } = require('@playwright/test');
const fs = require('fs');
const path = require('path');

const APP_URL = process.env.APP_URL || 'http://localhost:8080';
const ADMIN_USER = process.env.ADMIN_USER || 'minioadmin';
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || 'minioadmin';

test.describe('Bucket Stats', () => {
	let testBucket;
	const testFileName = 'test-upload.txt';
	const testFileContent = 'Hello IronBuckets! This is a test file for verifying size updates.';
	const testFilePath = path.join(__dirname, testFileName);

	// Helper to open Create Bucket Modal
	async function openCreateBucketModal(page) {
		await page.waitForFunction(() => typeof window.htmx !== 'undefined');
		await page.evaluate(() => {
			const btn = document.querySelector('button[hx-get="/buckets/create"]');
			if (btn) window.htmx.trigger(btn, 'click');
		});
		await page.waitForSelector('#bucket-modal', { state: 'visible', timeout: 5000 });
	}

	test.beforeAll(async () => {
		// Create a dummy file for upload
		fs.writeFileSync(testFilePath, testFileContent);
	});

	test.afterAll(async () => {
		// Cleanup dummy file
		if (fs.existsSync(testFilePath)) {
			fs.unlinkSync(testFilePath);
		}
	});

	test.beforeEach(async ({ page }) => {
		console.log('beforeEach: Starting setup');
		testBucket = `stats-test-${Date.now()}`;
		console.log(`beforeEach: Test bucket name: ${testBucket}`);

		// Login
		await page.goto(`${APP_URL}/login`);
		console.log('beforeEach: Navigated to login');
		await page.fill('input[name="accessKey"]', ADMIN_USER);
		await page.fill('input[name="secretKey"]', ADMIN_PASSWORD);
		await page.click('button[type="submit"]');
		await page.waitForURL(`${APP_URL}/`);
		console.log('beforeEach: Login successful');
	});

	test.afterEach(async ({ page }) => {
		console.log('afterEach: Starting cleanup');
		if (!testBucket) return;
		try {
			// Navigate to buckets list
			await page.goto(`${APP_URL}/buckets`);
			console.log('afterEach: Navigated to buckets');

			// Find bucket card
			const bucketLink = page.locator(`a[href="/buckets/${testBucket}"]`).first();
			if (await bucketLink.count() > 0) {
				// Find dropdown trigger within the card (parent of parent of link usually, or sibling logic)
				// The structure is:
				// <div class="group ...">
				//   <div class="flex ..."> ... dropdown ... </div>
				//   <a href="..."> ... </a>
				// </div>
				// We can find the card by text, then find the dropdown button.
				const card = page.locator('.group').filter({ hasText: testBucket }).first();
				const dropdownBtn = card.locator('button').first();

				await dropdownBtn.click();
				await page.getByRole('button', { name: 'Delete Bucket' }).click();
				await page.waitForSelector('#confirm-dialog[open]');
				await page.click('#confirm-dialog-confirm');
				await page.waitForTimeout(500);
			}
		} catch (e) {
			console.log('Cleanup failed:', e);
		}
	});

	test('should update bucket size after file upload', async ({ page }) => {
		test.setTimeout(120000);
		console.log('Starting test...');
		// 1. Create Bucket
		await page.goto(`${APP_URL}/buckets`);
		console.log('Navigated to buckets list');

		await openCreateBucketModal(page);
		console.log('Opened create modal');

		await page.fill('#bucket-modal input[name="bucketName"]', testBucket);
		await page.click('#bucket-modal button[type="submit"]');
		await page.waitForURL(`${APP_URL}/buckets`, { timeout: 5000 });
		console.log('Bucket created');

		// Verify initial state
		const bucketCard = page.locator('.group').filter({ hasText: testBucket });
		await expect(bucketCard).toBeVisible();
		await expect(bucketCard.getByText('Size')).toBeVisible();
		await expect(bucketCard.getByText('0 B')).toBeVisible();
		console.log('Initial state verified');

		// 2. Navigate to bucket and upload file
		await page.locator(`.group`).filter({ hasText: testBucket }).locator('a.block').filter({ hasText: testBucket }).click();
		await page.waitForURL(`${APP_URL}/buckets/${testBucket}`);
		console.log('Navigated to bucket');

		// Trigger the upload modal/input
		// Use the specific ID from the HTML
		const fileInput = page.locator('#upload-input');

		// Set files on the hidden input
		await fileInput.setInputFiles(testFilePath);
		console.log('File set on input');

		// The onchange handler triggers HTMX submit, which reloads the page.
		// We wait for the file name to appear in the list.
		await expect(page.getByText(testFileName)).toBeVisible({ timeout: 10000 });
		console.log('File uploaded and visible');

		// 3. Navigate back to buckets list
		await page.goto(`${APP_URL}/buckets`);
		console.log('Navigated back to buckets list');

		// 4. Verify Size updated
		// The size might take a moment to update if it's async (MinIO scanner).
		// We retry reloading the page until the size is not "0 B".
		await expect(async () => {
			await page.reload();
			const card = page.locator('.group').filter({ hasText: testBucket });
			// Debug: print current size text
			try {
				// Try to find the element that contains the size (usually "0 B" or "X B")
				// The structure is <div>...<span...>Size</span><div>...SIZE...</div>...</div>
				// We can look for the sibling of the "Size" label or similar.
				// Based on previous code:
				// <span ...>Size</span>
				// <div class="text-lg font-semibold text-white">{{ .FormattedSize }}</div>
				const sizeEl = card.locator('div.text-lg.font-semibold.text-white');
				const sizeText = await sizeEl.textContent();
				console.log(`Current size text: ${sizeText}`);
			} catch (e) {
				console.log('Could not read size text');
			}
			await expect(card.getByText('0 B')).not.toBeVisible({ timeout: 2000 });
		}).toPass({
			timeout: 120000, // Wait up to 120 seconds for MinIO to update stats
			intervals: [2000, 5000, 10000] // Retry intervals
		});

		console.log('Size updated (not 0 B)');
	});
});
