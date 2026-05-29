import { describe, expect, it } from 'vitest';

import { isValidCallsign, normalizeCallsign } from './callsign';

describe('callsign helpers', () => {
	it('normalizes to uppercase and trims', () => {
		expect(normalizeCallsign('  xx5yyy-1  ')).toBe('XX5YYY-1');
		expect(normalizeCallsign('iu5pmp')).toBe('IU5PMP');
	});

	it('accepts arbitrary callsigns', () => {
		expect(isValidCallsign('XX5YYY-1')).toBe(true);
		expect(isValidCallsign('IU5PMP')).toBe(true);
		expect(isValidCallsign('XX0XX-2')).toBe(true);
		expect(isValidCallsign('bad')).toBe(false);
		expect(isValidCallsign('')).toBe(false);
	});
});
