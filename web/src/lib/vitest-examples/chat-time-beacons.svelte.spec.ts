import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

vi.mock('$env/dynamic/public', () => ({ env: {} }));

import ChatPage from './ChatRouteHarness.svelte';

class StubEventSource {
	onopen: (() => void) | null = null;
	onerror: (() => void) | null = null;

	constructor(_url: string) {}

	addEventListener(_type: string, _listener: EventListener): void {}

	close(): void {}
}

describe('Broadcast time beacons', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		localStorage.removeItem('meshcom:chatShowTimeBeacons');
	});

	it('hides CET time beacons by default and shows them when enabled', async () => {
		vi.stubGlobal('EventSource', StubEventSource);
		localStorage.removeItem('meshcom:chatShowTimeBeacons');
		vi.stubGlobal(
			'fetch',
			vi.fn(async (input: RequestInfo | URL) => {
				const url = String(input);
				if (url.includes('/api/chat/list')) return jsonResponse([]);
				if (url.includes('/api/chat/P_broadcast')) return jsonResponse(records);
				return jsonResponse({});
			})
		);

		render(ChatPage);

		const panel = page.getByTestId('chat-panel');
		await expect.element(panel.getByText('No messages in this chat')).toBeVisible();
		await page.getByRole('checkbox', { name: 'Show time beacons' }).click();
		expect(localStorage.getItem('meshcom:chatShowTimeBeacons')).toBe('1');
		await expect.element(panel.getByText('2026-07-23 10:06:35')).toBeVisible();
	});
});

const records = [
	{
		received_at: '2026-07-23T10:06:35Z',
		src: 'OE1XAR-33,IZ5CND-10',
		dst: '*',
		msg: '{CET}2026-07-23 10:06:35'
	}
];

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}
