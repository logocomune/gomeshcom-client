import { expect, test } from '@playwright/test';

const MOBILE = { width: 390, height: 844 };
const DESKTOP = { width: 1280, height: 800 };

// ── TopNav visibility ─────────────────────────────────────────────────────────

test.describe('responsive layout — mobile (390px)', () => {
	test.use({ viewport: MOBILE });

	test('status pill hidden on mobile', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="status-pill"]')).not.toBeVisible();
	});

	test('packet counter hidden on mobile', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="packet-counter"]')).not.toBeVisible();
	});

	test('status dot visible next to logo on mobile', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="status-dot"]')).toBeVisible();
	});
});

test.describe('responsive layout — desktop (1280px)', () => {
	test.use({ viewport: DESKTOP });

	test('status pill visible on desktop', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="status-pill"]')).toBeVisible();
	});

	test('packet counter visible on desktop', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="packet-counter"]')).toBeVisible();
	});

	test('mobile status dot hidden on desktop', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="status-dot"]')).not.toBeVisible();
	});
});

// ── Dashboard summary layout ───────────────────────────────────────────────────

test.describe('dashboard — summary layout', () => {
	test('status card renders', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="dashboard-status"]')).toBeVisible();
	});

	test('nodes metric card renders', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="dashboard-nodes-card"]')).toBeVisible();
	});

	test('unread metric card renders', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="dashboard-unread-card"]')).toBeVisible();
	});

	test('events metric card renders', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="dashboard-events-card"]')).toBeVisible();
	});

	test('recent messages section renders', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="dashboard-messages"]')).toBeVisible();
	});

	test('recent traffic section renders', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('[data-testid="dashboard-traffic"]')).toBeVisible();
	});

});
