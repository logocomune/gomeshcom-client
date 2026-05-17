import { expect, test } from '@playwright/test';

test('about page loads', async ({ page }) => {
	await page.goto('/about');

	await expect(page.getByRole('heading', { level: 1, name: 'goMeshCom' })).toBeVisible();
	await expect(page.getByText('Web client for the MeshCom mesh radio network')).toBeVisible();
});
