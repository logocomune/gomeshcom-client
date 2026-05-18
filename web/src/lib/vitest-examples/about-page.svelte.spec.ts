import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import AboutPage from '../../routes/about/+page.svelte';

describe('About page', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('shows backend version and my call', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => {
				return new Response(JSON.stringify({ version: 'v1.2.3', callsign: 'QQ1ABC-1' }), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				});
			})
		);

		render(AboutPage);

		await expect.element(page.getByText('Version')).toBeVisible();
		await expect.element(page.getByText('v1.2.3')).toBeVisible();
		await expect.element(page.getByText('My call')).toBeVisible();
		await expect.element(page.getByText('QQ1ABC-1')).toBeVisible();
	});

	it('links reference repository', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => {
				return new Response(JSON.stringify({}), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				});
			})
		);

		render(AboutPage);

		await expect
			.element(page.getByRole('link', { name: 'github.com/logocomune/gomeshcom-udp' }))
			.toHaveAttribute('href', 'https://github.com/logocomune/gomeshcom-udp');
	});
});
