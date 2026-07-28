import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

vi.mock('$env/dynamic/public', () => ({ env: {} }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

import ChatPage from './ChatRouteHarness.svelte';
import { chatState } from '$lib/stores/chat.svelte';
import { eventsState } from '$lib/stores/events.svelte';

class StubEventSource {
	onopen: (() => void) | null = null;
	onerror: (() => void) | null = null;

	constructor(_url: string) {}

	addEventListener(_type: string, _listener: EventListener): void {}

	close(): void {}
}

describe('chat callsign mentions', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		eventsState.storedPositions = [];
	});

	it('underlines base callsigns and opens menu for SSID callsigns', async () => {
		vi.stubGlobal('EventSource', StubEventSource);
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
		await expect.element(panel.getByRole('button', { name: '@IU5PMP-12' })).toBeVisible();
		await expect.element(panel.getByText('@IU5PMP', { exact: true })).toBeVisible();
		await expect
			.element(panel.getByRole('button', { name: '@IU5PMP', exact: true }))
			.not.toBeInTheDocument();
		eventsState.storedPositions = [
			{ id: 'IU5PMP-12', source: 'IU5PMP-12', lat: 43.7, lon: 11.2, updatedAt: '' }
		];
		await page.getByRole('button', { name: '@IU5PMP-12' }).click();
		await expect.element(panel.getByRole('button', { name: 'Map' })).toBeVisible();
		await expect.element(panel.getByRole('button', { name: 'Chat' })).toBeVisible();

		const openSpy = vi.spyOn(window, 'open').mockReturnValue(null);
		await page.getByRole('button', { name: 'https://example.org' }).click();
		await panel.getByRole('button', { name: 'Follow link' }).click();
		expect(openSpy).toHaveBeenCalledWith('https://example.org', '_blank', 'noopener,noreferrer');

		await page.getByRole('button', { name: '@IU5PMP-12' }).click();
		await panel.getByRole('button', { name: 'Chat' }).click();
		expect(chatState.chatTarget).toEqual({ kind: 'contact', value: 'IU5PMP-12' });
	});
});

const records = [
	{
		received_at: '2026-05-16T09:00:00Z',
		src: 'QQ5PFI-1',
		dst: '*',
		msg: 'CQ @IU5PMP and @IU5PMP-12; info https://example.org'
	}
];

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}
