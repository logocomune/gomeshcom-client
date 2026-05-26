import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { eventsState } from './events.svelte';

describe('eventsState replay cursor', () => {
	let storage: Map<string, string>;

	beforeEach(() => {
		storage = new Map();
		vi.stubGlobal('localStorage', {
			getItem: (key: string) => storage.get(key) ?? null,
			setItem: (key: string, value: string) => storage.set(key, value),
			removeItem: (key: string) => storage.delete(key),
			clear: () => storage.clear()
		});
		eventsState.clear();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('does not create replay cursor when clearing in-memory state', () => {
		eventsState.clear();

		expect(eventsState.replayFrom()).toBeUndefined();
	});

	it('creates replay cursor only for UDP stream clear', () => {
		eventsState.clearAndSaveReplayCursor();

		expect(eventsState.replayFrom()).toEqual(expect.any(String));
	});
});
