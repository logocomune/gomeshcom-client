import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSseStore } from './sse.svelte';

describe('createSseStore', () => {
	beforeEach(() => {
		vi.stubGlobal('localStorage', {
			getItem: () => null,
			setItem: vi.fn(),
			removeItem: vi.fn(),
			clear: vi.fn()
		});
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('opens at most one stream while active', () => {
		const stop = vi.fn();
		const connect = vi.fn(() => stop);
		const store = createSseStore({ clearSendEcho: vi.fn() }, connect);

		store.connect();
		store.connect();

		expect(connect).toHaveBeenCalledTimes(1);

		store.disconnect();

		expect(stop).toHaveBeenCalledTimes(1);
	});
});
