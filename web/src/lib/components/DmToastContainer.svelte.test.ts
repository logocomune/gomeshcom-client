import { page } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DmToastContainer from './DmToastContainer.svelte';
import { toastStore } from '$lib/stores/toasts.svelte';

beforeEach(() => {
	toastStore.toasts = [];
});

afterEach(() => {
	toastStore.toasts = [];
});

describe('DmToastContainer — DM toast', () => {
	it('renders DM toast with sender callsign', async () => {
		toastStore.addDm('IU5PMP-1');
		render(DmToastContainer);

		await expect.element(page.getByText('Direct Message')).toBeVisible();
		await expect.element(page.getByText('IU5PMP-1')).toBeVisible();
	});

	it('renders dismiss button with accessible label', async () => {
		toastStore.addDm('IU5PMP-1');
		render(DmToastContainer);

		await expect.element(page.getByRole('button', { name: 'Dismiss' })).toBeVisible();
	});

	it('dismiss button removes toast', async () => {
		toastStore.addDm('IU5PMP-1');
		render(DmToastContainer);

		await page.getByRole('button', { name: 'Dismiss' }).click();
		await expect.element(page.getByText('Direct Message')).not.toBeInTheDocument();
	});

	it('has aria-live polite region for screen readers', async () => {
		toastStore.addDm('IU5PMP-1');
		render(DmToastContainer);

		await expect.element(page.getByRole('status')).toBeVisible();
	});
});

describe('DmToastContainer — mention toast', () => {
	it('renders mention toast with sender and channel', async () => {
		toastStore.addMention('IU5PMP-1', '9');
		render(DmToastContainer);

		await expect.element(page.getByText('Mention')).toBeVisible();
		await expect.element(page.getByText('IU5PMP-1')).toBeVisible();
		await expect.element(page.getByText('#9')).toBeVisible();
	});

	it('renders no toasts when store is empty', async () => {
		render(DmToastContainer);

		await expect.element(page.getByRole('status')).not.toBeInTheDocument();
	});
});
