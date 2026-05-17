import { afterEach, describe, expect, it, vi } from 'vitest';
import { watchDesktop } from './responsive';

function setupMatchMedia(initialMatches: boolean) {
	let _matches = initialMatches;
	const listeners = new Set<() => void>();
	const mq = {
		get matches() {
			return _matches;
		},
		set matches(v: boolean) {
			_matches = v;
		},
		addEventListener: vi.fn((_: string, cb: () => void) => listeners.add(cb)),
		removeEventListener: vi.fn((_: string, cb: () => void) => listeners.delete(cb)),
		fire() {
			listeners.forEach((cb) => cb());
		}
	};
	vi.stubGlobal('matchMedia', vi.fn(() => mq));
	return mq;
}

afterEach(() => vi.unstubAllGlobals());

describe('watchDesktop', () => {
	it('calls onChange immediately with true when matches=true', () => {
		setupMatchMedia(true);
		const values: boolean[] = [];
		const stop = watchDesktop((v) => values.push(v));
		stop();
		expect(values).toEqual([true]);
	});

	it('calls onChange immediately with false when matches=false', () => {
		setupMatchMedia(false);
		const values: boolean[] = [];
		const stop = watchDesktop((v) => values.push(v));
		stop();
		expect(values).toEqual([false]);
	});

	it('calls onChange when matchMedia fires a change to false', () => {
		const mq = setupMatchMedia(true);
		const values: boolean[] = [];
		const stop = watchDesktop((v) => values.push(v));
		mq.matches = false;
		mq.fire();
		stop();
		expect(values).toEqual([true, false]);
	});

	it('calls onChange when matchMedia fires a change to true', () => {
		const mq = setupMatchMedia(false);
		const values: boolean[] = [];
		const stop = watchDesktop((v) => values.push(v));
		mq.matches = true;
		mq.fire();
		stop();
		expect(values).toEqual([false, true]);
	});

	it('removes event listener on cleanup', () => {
		const mq = setupMatchMedia(true);
		const stop = watchDesktop(() => {});
		stop();
		expect(mq.removeEventListener).toHaveBeenCalledOnce();
	});

	it('does not fire after cleanup', () => {
		const mq = setupMatchMedia(true);
		const values: boolean[] = [];
		const stop = watchDesktop((v) => values.push(v));
		stop();
		mq.matches = false;
		mq.fire();
		expect(values).toEqual([true]);
	});

	it('returns true immediately when matchMedia is unavailable (SSR)', () => {
		// matchMedia is not defined in Node — no stub needed, just verify SSR fallback
		const values: boolean[] = [];
		const stop = watchDesktop((v) => values.push(v));
		stop();
		expect(values).toEqual([true]);
	});
});
