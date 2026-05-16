import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	UnauthorizedError,
	apiFetch,
	login,
	logout,
	getSessionStatus,
	onUnauthorized
} from './auth';

describe('apiFetch', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('uses same-origin credentials by default', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));

		await apiFetch('/api/chat/list');

		expect(fetchSpy).toHaveBeenCalledWith(
			'/api/chat/list',
			expect.objectContaining({
				credentials: 'same-origin'
			})
		);
	});

	it('notifies and throws on 401', async () => {
		const unauthorized = vi.fn();
		const unsubscribe = onUnauthorized(unauthorized);
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('unauthorized', { status: 401 }));

		await expect(apiFetch('/api/chat/list')).rejects.toBeInstanceOf(UnauthorizedError);
		expect(unauthorized).toHaveBeenCalledTimes(1);

		unsubscribe();
	});
});

describe('session endpoints', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('posts login payload with same-origin credentials', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));

		await login('admin', 'secret');

		expect(fetchSpy).toHaveBeenCalledWith(
			'/api/session',
			expect.objectContaining({
				method: 'POST',
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ username: 'admin', password: 'secret' })
			})
		);
	});

	it('returns session status from endpoint', async () => {
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(
			new Response(JSON.stringify({ required: true, authenticated: false }), {
				status: 401,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		await expect(getSessionStatus()).resolves.toEqual({ required: true, authenticated: false });
	});

	it('sends logout request', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));

		await logout();

		expect(fetchSpy).toHaveBeenCalledWith(
			'/api/session',
			expect.objectContaining({
				method: 'DELETE',
				credentials: 'same-origin'
			})
		);
	});
});
