import { describe, expect, it } from 'vitest';
import { getLocator } from './maidenhead';

describe('getLocator', () => {
	it('returns known Maidenhead locator prefixes', () => {
		expect.assertions(3);

		expect(getLocator(10.3476, 43.5076, 2)).toBe('JN53');
		expect(getLocator(10.3476, 43.5076, 3)).toMatch(/^JN53/);
		expect(getLocator(181, 0, 1)).toBe('AJ');
	});
});
