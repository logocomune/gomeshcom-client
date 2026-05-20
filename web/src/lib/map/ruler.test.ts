import { describe, expect, it } from 'vitest';
import { buildRulerLinks, calculateDistanceKm } from './ruler';
import type { MapPosition } from './types';

function makePosition(overrides: Partial<MapPosition>): MapPosition {
	return {
		id: overrides.id ?? 'id',
		source: overrides.source ?? 'QQ0AAA',
		lat: overrides.lat ?? 45,
		lon: overrides.lon ?? 12,
		updatedAt: overrides.updatedAt ?? '2026-05-20T10:00:00Z',
		lastSeen: overrides.lastSeen ?? '2026-05-20T10:00:00Z',
		...overrides
	};
}

describe('calculateDistanceKm', () => {
	it('returns 0 for identical points', () => {
		expect(calculateDistanceKm(45, 12, 45, 12)).toBeCloseTo(0, 6);
	});
});

describe('buildRulerLinks', () => {
	it('builds only direct live links from mycall to other stations', () => {
		const now = Date.parse('2026-05-20T10:00:00Z');
		const myCallPosition = makePosition({
			id: 'self',
			source: 'QQ1ABC',
			lat: 45,
			lon: 12,
			lastSeen: '2026-05-20T09:58:00Z'
		});

		const links = buildRulerLinks(
			myCallPosition,
			[
				myCallPosition,
				makePosition({
					id: 'direct',
					source: 'QQ2BBB',
					lat: 45.01,
					lon: 12.01,
					lastSeen: '2026-05-20T09:59:00Z',
					lastDirectSeen: '2026-05-20T09:59:30Z'
				}),
				makePosition({
					id: 'indirect',
					source: 'QQ3CCC',
					lat: 45.02,
					lon: 12.02,
					lastSeen: '2026-05-20T09:59:00Z'
				}),
				makePosition({
					id: 'stale',
					source: 'QQ4DDD',
					lat: 45.03,
					lon: 12.03,
					lastSeen: '2026-05-20T08:00:00Z',
					lastDirectSeen: '2026-05-20T08:00:00Z'
				})
			],
			now
		);

		expect(links).toHaveLength(1);
		expect(links[0].to.source).toBe('QQ2BBB');
		expect(links[0].label).toMatch(/^\d+\.\dkm$/);
	});
});
