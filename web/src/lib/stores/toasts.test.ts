import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { toastStore } from './toasts.svelte';

beforeEach(() => {
	vi.useFakeTimers();
	toastStore.toasts = [];
});

afterEach(() => {
	vi.useRealTimers();
});

describe('ToastStore.addDm', () => {
	it('appends a dm toast with correct fields', () => {
		toastStore.addDm('IU5PMP');
		expect(toastStore.toasts).toHaveLength(1);
		const t = toastStore.toasts[0];
		expect(t.kind).toBe('dm');
		expect(t.from).toBe('IU5PMP');
		expect(typeof t.id).toBe('number');
	});

	it('assigns unique ids for multiple toasts', () => {
		toastStore.addDm('A');
		toastStore.addDm('B');
		const ids = toastStore.toasts.map((t) => t.id);
		expect(new Set(ids).size).toBe(2);
	});

	it('auto-dismisses after 30 seconds', () => {
		toastStore.addDm('X');
		expect(toastStore.toasts).toHaveLength(1);
		vi.advanceTimersByTime(30_000);
		expect(toastStore.toasts).toHaveLength(0);
	});
});

describe('ToastStore.addMention', () => {
	it('appends a mention toast with from and channel', () => {
		toastStore.addMention('IU5PMP', 'Broadcast');
		expect(toastStore.toasts).toHaveLength(1);
		const t = toastStore.toasts[0];
		expect(t.kind).toBe('mention');
		expect(t.from).toBe('IU5PMP');
		if (t.kind === 'mention') expect(t.channel).toBe('Broadcast');
	});

	it('auto-dismisses after 30 seconds', () => {
		toastStore.addMention('Y', 'test');
		vi.advanceTimersByTime(30_000);
		expect(toastStore.toasts).toHaveLength(0);
	});
});

describe('ToastStore.dismiss', () => {
	it('removes toast by id', () => {
		toastStore.addDm('A');
		toastStore.addDm('B');
		const id = toastStore.toasts[0].id;
		toastStore.dismiss(id);
		expect(toastStore.toasts).toHaveLength(1);
		expect(toastStore.toasts[0].from).toBe('B');
	});

	it('no-ops for unknown id', () => {
		toastStore.addDm('A');
		toastStore.dismiss(9999);
		expect(toastStore.toasts).toHaveLength(1);
	});
});
