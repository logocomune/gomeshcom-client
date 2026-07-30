import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

vi.mock('$env/dynamic/public', () => ({ env: {} }));

import ConnectionOverlay from './ConnectionOverlay.svelte';
import UdpStreamPanel from './UdpStreamPanel.svelte';

describe('stream status copy', () => {
	it('uses transport-neutral waiting messages', async () => {
		expect.assertions(3);

		render(ConnectionOverlay, { props: { state: 'connecting' } });
		render(UdpStreamPanel, {
			props: {
				events: [],
				filteredEvents: [],
				streamFilter: '',
				selectedEvent: null,
				isDesktop: true,
				streamHeightPx: 320,
				selectEvent: vi.fn(),
				onClearEvents: vi.fn(),
				showRawEvent: vi.fn()
			}
		});

		await expect.element(page.getByText('Waiting for packet stream…')).toBeVisible();
		await expect.element(page.getByText('Packet stream', { exact: true })).toBeVisible();
		await expect.element(page.getByText('Waiting for packets')).toBeVisible();
	});
});
