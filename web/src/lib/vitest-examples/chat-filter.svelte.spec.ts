import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

vi.mock('$env/dynamic/public', () => ({ env: {} }));

import ChatPage from './ChatRouteHarness.svelte';
import type { ChatRecord, Conversation } from '$lib/api/types';

class StubEventSource {
	onopen: (() => void) | null = null;
	onerror: (() => void) | null = null;

	constructor(_url: string) {}

	addEventListener(_type: string, _listener: EventListener): void {}

	close(): void {}
}

describe('chat message filter', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('filters visible chat messages by entered text', async () => {
		vi.stubGlobal('EventSource', StubEventSource);
		vi.stubGlobal(
			'fetch',
			vi.fn(async (input: RequestInfo | URL) => {
				const url = String(input);
				if (url.endsWith('/api/chat/list')) return jsonResponse(conversations);
				if (url.includes('/api/chat/P_broadcast')) return jsonResponse(records);
				return jsonResponse({});
			})
		);

		render(ChatPage);

		await expect.element(page.getByTestId('chat-panel').getByText('alpha beacon')).toBeVisible();
		await expect.element(page.getByTestId('chat-panel').getByText('beta packet')).toBeVisible();

		await page.getByRole('searchbox', { name: 'Filter messages' }).fill('alpha');

		await expect.element(page.getByTestId('chat-panel').getByText('alpha beacon')).toBeVisible();

		await page.getByRole('searchbox', { name: 'Filter messages' }).fill('missing');

		await expect
			.element(page.getByTestId('chat-panel').getByText('No matching messages'))
			.toBeVisible();
	});
});

const conversations: Conversation[] = [
	{
		id: 'P_broadcast',
		kind: 'broadcast',
		label: 'Broadcast',
		last_seen: '2026-05-16T09:01:00Z',
		size: 2
	}
];

const records: ChatRecord[] = [
	{
		received_at: '2026-05-16T09:00:00Z',
		src: 'IU5PMP-1',
		dst: '*',
		msg: 'alpha beacon'
	},
	{
		received_at: '2026-05-16T09:01:00Z',
		src: 'IZ5PFI-1',
		dst: '*',
		msg: 'beta packet'
	}
];

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}
