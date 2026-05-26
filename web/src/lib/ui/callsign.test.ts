import { describe, expect, it } from 'vitest';

import { isValidCallsign, normalizeCallsign } from './callsign';

describe('callsign helpers', () => {
	it('rewrites non-IU prefixes to QQ', () => {
		expect(normalizeCallsign('ik5mnn-1')).toBe('QQ5MNN-1');
		expect(normalizeCallsign('xx0xx-1')).toBe('QQ0XX-1');
	});

	it('keeps IU5PMP unchanged', () => {
		expect(normalizeCallsign('iu5pmp')).toBe('IU5PMP');
		expect(normalizeCallsign('iu5pmp-1')).toBe('IU5PMP-1');
	});

	it('validates normalized callsigns only', () => {
		expect(isValidCallsign('QQ1ABC-1')).toBe(true);
		expect(isValidCallsign('IU5PMP-1')).toBe(true);
		expect(isValidCallsign('bad')).toBe(false);
	});
});
