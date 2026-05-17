import { expect, test } from '@playwright/test';

const MOBILE = { width: 390, height: 844 };
const DESKTOP = { width: 1280, height: 800 };

// ── Mobile ────────────────────────────────────────────────────────────────────

test.describe('responsive layout — mobile (390px)', () => {
	test.use({ viewport: MOBILE });

	test('status pill hidden', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="status-pill"]')).not.toBeVisible();
	});

	test('packet counter hidden', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="packet-counter"]')).not.toBeVisible();
	});

	test('status dot visible next to logo', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="status-dot"]')).toBeVisible();
	});

	test('vertical drag handle hidden', async ({ page }) => {
		await page.goto('/');
		await expect(page.getByRole('separator', { name: 'Resize chat and map' })).not.toBeVisible();
	});

	test('horizontal drag handle hidden', async ({ page }) => {
		await page.goto('/');
		await expect(
			page.getByRole('separator', { name: 'Resize UDP stream panel' })
		).not.toBeVisible();
	});

	test('chat panel renders above map panel', async ({ page }) => {
		await page.goto('/');
		const chatBox = await page.locator('[data-testid="chat-panel"]').boundingBox();
		const mapBox = await page.locator('[data-testid="map-panel"]').boundingBox();
		expect(chatBox).not.toBeNull();
		expect(mapBox).not.toBeNull();
		expect(chatBox!.y).toBeLessThan(mapBox!.y);
	});

	test('map panel renders above udp panel', async ({ page }) => {
		await page.goto('/');
		const mapBox = await page.locator('[data-testid="map-panel"]').boundingBox();
		const udpBox = await page.locator('[data-testid="udp-panel"]').boundingBox();
		expect(mapBox).not.toBeNull();
		expect(udpBox).not.toBeNull();
		expect(mapBox!.y).toBeLessThan(udpBox!.y);
	});

	test('chat panel height is ~80vh', async ({ page }) => {
		await page.goto('/');
		const box = await page.locator('[data-testid="chat-panel"]').boundingBox();
		expect(box).not.toBeNull();
		const expected = MOBILE.height * 0.8;
		expect(box!.height).toBeGreaterThanOrEqual(expected - 10);
		expect(box!.height).toBeLessThanOrEqual(expected + 10);
	});

	test('map panel height is ~80vh', async ({ page }) => {
		await page.goto('/');
		const box = await page.locator('[data-testid="map-panel"]').boundingBox();
		expect(box).not.toBeNull();
		const expected = MOBILE.height * 0.8;
		expect(box!.height).toBeGreaterThanOrEqual(expected - 10);
		expect(box!.height).toBeLessThanOrEqual(expected + 10);
	});

	test('udp panel height is ~80vh', async ({ page }) => {
		await page.goto('/');
		const box = await page.locator('[data-testid="udp-panel"]').boundingBox();
		expect(box).not.toBeNull();
		const expected = MOBILE.height * 0.8;
		expect(box!.height).toBeGreaterThanOrEqual(expected - 10);
		expect(box!.height).toBeLessThanOrEqual(expected + 10);
	});

	test('panels do not overlap', async ({ page }) => {
		await page.goto('/');
		const chatBox = await page.locator('[data-testid="chat-panel"]').boundingBox();
		const mapBox = await page.locator('[data-testid="map-panel"]').boundingBox();
		const udpBox = await page.locator('[data-testid="udp-panel"]').boundingBox();
		expect(chatBox).not.toBeNull();
		expect(mapBox).not.toBeNull();
		expect(udpBox).not.toBeNull();
		// chat bottom <= map top
		expect(chatBox!.y + chatBox!.height).toBeLessThanOrEqual(mapBox!.y + 4);
		// map bottom <= udp top
		expect(mapBox!.y + mapBox!.height).toBeLessThanOrEqual(udpBox!.y + 4);
	});
});

// ── Desktop ───────────────────────────────────────────────────────────────────

test.describe('responsive layout — desktop (1280px)', () => {
	test.use({ viewport: DESKTOP });

	test('status pill visible', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="status-pill"]')).toBeVisible();
	});

	test('packet counter visible', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="packet-counter"]')).toBeVisible();
	});

	test('mobile status dot hidden', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="status-dot"]')).not.toBeVisible();
	});

	test('vertical drag handle visible', async ({ page }) => {
		await page.goto('/');
		await expect(page.getByRole('separator', { name: 'Resize chat and map' })).toBeVisible();
	});

	test('horizontal drag handle visible', async ({ page }) => {
		await page.goto('/');
		await expect(page.getByRole('separator', { name: 'Resize UDP stream panel' })).toBeVisible();
	});

	test('chat and map panels are side by side (same vertical origin)', async ({ page }) => {
		await page.goto('/');
		const chatBox = await page.locator('[data-testid="chat-panel"]').boundingBox();
		const mapBox = await page.locator('[data-testid="map-panel"]').boundingBox();
		expect(chatBox).not.toBeNull();
		expect(mapBox).not.toBeNull();
		// top edges within 4px — flex-row layout
		expect(Math.abs(chatBox!.y - mapBox!.y)).toBeLessThanOrEqual(4);
	});

	test('chat panel is left of map panel', async ({ page }) => {
		await page.goto('/');
		const chatBox = await page.locator('[data-testid="chat-panel"]').boundingBox();
		const mapBox = await page.locator('[data-testid="map-panel"]').boundingBox();
		expect(chatBox).not.toBeNull();
		expect(mapBox).not.toBeNull();
		expect(chatBox!.x + chatBox!.width).toBeLessThanOrEqual(mapBox!.x + 20);
	});

	test('udp panel renders below chat and map row', async ({ page }) => {
		await page.goto('/');
		const chatBox = await page.locator('[data-testid="chat-panel"]').boundingBox();
		const udpBox = await page.locator('[data-testid="udp-panel"]').boundingBox();
		expect(chatBox).not.toBeNull();
		expect(udpBox).not.toBeNull();
		expect(chatBox!.y + chatBox!.height).toBeLessThanOrEqual(udpBox!.y + 4);
	});
});
