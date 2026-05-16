import { describe, expect, it } from 'vitest';
import { hardwareHumanName, hardwareInfo } from './hardware';

describe('hardware lookup', () => {
	it('maps known hardware IDs to human names', () => {
		expect(hardwareHumanName(42)).toBe('Heltec Stick V3');
		expect(hardwareHumanName('52')).toBe('Heltec V4');
	});

	it('returns readable fallback for unknown numeric hardware IDs', () => {
		expect(hardwareInfo(999)).toEqual({ id: 999, name: 'HW_999', humanName: 'HW 999' });
	});
});
