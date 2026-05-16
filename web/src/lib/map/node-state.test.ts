import { describe, it, expect } from 'vitest';
import { nodeFreshness, FRESHNESS_FILL } from './node-state';
import type { MapPosition } from './types';

function makePosition(overrides: Partial<MapPosition> = {}): MapPosition {
	return {
		id: 'test',
		source: 'TEST-1',
		lat: 45,
		lon: 12,
		updatedAt: new Date().toISOString(),
		...overrides
	};
}

const NOW = new Date('2024-01-15T12:00:00Z').getTime();

describe('nodeFreshness', () => {
	it('returns hidden when lastSeen is missing', () => {
		expect(nodeFreshness(makePosition(), NOW)).toBe('hidden');
	});

	it('returns direct when lastDirectSeen within 30 min', () => {
		const pos = makePosition({
			lastSeen: new Date(NOW - 10 * 60 * 1000).toISOString(),
			lastDirectSeen: new Date(NOW - 10 * 60 * 1000).toISOString()
		});
		expect(nodeFreshness(pos, NOW)).toBe('direct');
	});

	it('returns indirect when lastSeen within 60 min but no direct', () => {
		const pos = makePosition({
			lastSeen: new Date(NOW - 45 * 60 * 1000).toISOString()
		});
		expect(nodeFreshness(pos, NOW)).toBe('indirect');
	});

	it('returns stale when lastSeen between 60 min and 48 h', () => {
		const pos = makePosition({
			lastSeen: new Date(NOW - 2 * 60 * 60 * 1000).toISOString()
		});
		expect(nodeFreshness(pos, NOW)).toBe('stale');
	});

	it('returns stale when lastSeen just under 48 h', () => {
		const pos = makePosition({
			lastSeen: new Date(NOW - (48 * 60 * 60 * 1000 - 1000)).toISOString()
		});
		expect(nodeFreshness(pos, NOW)).toBe('stale');
	});

	it('returns hidden when lastSeen over 48 h', () => {
		const pos = makePosition({
			lastSeen: new Date(NOW - 49 * 60 * 60 * 1000).toISOString()
		});
		expect(nodeFreshness(pos, NOW)).toBe('hidden');
	});

	it('returns stale at exactly 48 h boundary', () => {
		const pos = makePosition({
			lastSeen: new Date(NOW - 48 * 60 * 60 * 1000).toISOString()
		});
		expect(nodeFreshness(pos, NOW)).toBe('stale');
	});
});

describe('FRESHNESS_FILL', () => {
	it('has non-transparent fill for direct, indirect, stale', () => {
		expect(FRESHNESS_FILL.direct).not.toContain('rgba(0,0,0,0)');
		expect(FRESHNESS_FILL.indirect).not.toContain('rgba(0,0,0,0)');
		expect(FRESHNESS_FILL.stale).not.toContain('rgba(0,0,0,0)');
	});

	it('has transparent fill for hidden', () => {
		expect(FRESHNESS_FILL.hidden).toBe('rgba(0,0,0,0)');
	});
});
