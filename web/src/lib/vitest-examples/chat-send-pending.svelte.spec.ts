import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

vi.mock('$env/dynamic/public', () => ({ env: {} }));

import ChatPage from '../../routes/+page.svelte';

class StubEventSource {
	static instances: StubEventSource[] = [];

	onopen: (() => void) | null = null;
	onerror: (() => void) | null = null;
	listeners = new Map<string, EventListener[]>();

	constructor(_url: string) {
		StubEventSource.instances.push(this);
	}

	addEventListener(type: string, listener: EventListener): void {
		const listeners = this.listeners.get(type) ?? [];
		listeners.push(listener);
		this.listeners.set(type, listeners);
	}

	close(): void {}

	emit(type: string, data: unknown): void {
		const event = new MessageEvent(type, { data: JSON.stringify(data) });
		for (const listener of this.listeners.get(type) ?? []) {
			listener(event);
		}
	}
}

describe('chat send pending state', () => {
	afterEach(() => {
		StubEventSource.instances = [];
		vi.unstubAllGlobals();
	});

	it('shows the sent message immediately with pending status', async () => {
		vi.stubGlobal('EventSource', StubEventSource);
		vi.stubGlobal(
			'fetch',
			vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
				const url = String(input);
				if (url.endsWith('/api/chat/list')) return jsonResponse([]);
				if (url.includes('/api/chat/P_broadcast')) return jsonResponse([]);
				if (url.endsWith('/api/messages') && init?.method === 'POST') {
					return jsonResponse({ type: 'msg', dst: '*', msg: 'hello pending' }, 202);
				}
				return jsonResponse({});
			})
		);

		render(ChatPage);

		await page.getByPlaceholder('Type a message…').fill('hello pending');
		await page.getByRole('button', { name: 'Send' }).click();

		await expect.element(page.getByText('hello pending')).toBeVisible();
		await expect.element(page.getByTitle('Pending').nth(1)).toBeVisible();
	});

	it('shows a green cloud when a public channel send echoes back from UDP', async () => {
		vi.stubGlobal('EventSource', StubEventSource);
		vi.stubGlobal(
			'fetch',
			vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
				const url = String(input);
				if (url.endsWith('/api/chat/list')) return jsonResponse([]);
				if (url.includes('/api/chat/P_broadcast')) return jsonResponse([]);
				if (url.endsWith('/api/messages') && init?.method === 'POST') {
					return jsonResponse({ type: 'msg', dst: '*', msg: 'hello public' }, 202);
				}
				return jsonResponse({});
			})
		);

		render(ChatPage);
		const source = StubEventSource.instances[0];
		source.emit('station.identity', { callsign: 'QQ0QQ-1' });

		await page.getByPlaceholder('Type a message…').fill('hello public');
		await page.getByRole('button', { name: 'Send' }).click();

		source.emit('packet.received', {
			packet: {
				type: 'msg',
				src_type: 'udp',
				src: 'QQ0QQ-1',
				dst: '*',
				msg: 'hello public'
			}
		});

		await expect.element(page.getByTitle('Node echo observed')).toBeVisible();
		await expect.element(page.getByTestId('chat-panel').getByText('hello public')).toBeVisible();
	});
	it('loads chat from history, ignores replayed UDP messages, and appends live UDP messages', async () => {
		vi.stubGlobal('EventSource', StubEventSource);
		vi.stubGlobal(
			'fetch',
			vi.fn(async (input: RequestInfo | URL) => {
				const url = String(input);
				if (url.endsWith('/api/chat/list')) {
					return jsonResponse([
						{
							id: 'P_broadcast',
							kind: 'broadcast',
							label: 'Broadcast',
							last_seen: '2026-05-19T17:00:00Z',
							size: 1
						}
					]);
				}
				if (url.includes('/api/chat/P_broadcast')) {
					return jsonResponse([
						{
							received_at: '2026-05-19T17:00:00Z',
							src: 'QQ1OLD-1',
							dst: '*',
							msg: 'history from file'
						}
					]);
				}
				return jsonResponse({});
			})
		);

		render(ChatPage);
		const source = StubEventSource.instances[0];

		await expect
			.element(page.getByTestId('chat-panel').getByText('history from file'))
			.toBeVisible();

		source.emit('packet.received', {
			replay: true,
			packet: {
				type: 'msg',
				src: 'QQ1RAW-1',
				dst: '*',
				msg: 'replayed raw message'
			}
		});

		await vi.waitFor(() => {
			expect(document.body.textContent).not.toContain('replayed raw message');
		});

		source.emit('packet.received', {
			packet: {
				type: 'msg',
				src: 'QQ1LIVE-1',
				dst: '*',
				msg: 'live stream message'
			}
		});

		await expect
			.element(page.getByTestId('chat-panel').getByText('live stream message'))
			.toBeVisible();
	});
});

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}
