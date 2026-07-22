import { describe, expect, it } from 'vitest';
import { buildActiveMapPathSegments } from './map-paths';
import type { MapPosition } from './types';

const NOW = Date.parse('2026-05-20T10:00:00Z');

describe('buildActiveMapPathSegments', () => {
	it('builds direct and relayed paths for active non-stale nodes only', () => {
		const segments = buildActiveMapPathSegments(
			'MYCALL-1',
			[
				position('MYCALL-1', { lastDirectSeen: '2026-05-20T09:59:00Z' }),
				position('DIRECT-1', { lastDirectSeen: '2026-05-20T09:58:00Z', lon: 12.1 }),
				position('R2', { lastDirectSeen: '2026-05-20T09:57:00Z', lon: 12.2 }),
				position('R1', { lastSeen: '2026-05-20T09:56:00Z', lon: 12.3 }),
				position('ORIGIN-1', {
					lastSeen: '2026-05-20T09:55:00Z',
					lon: 12.4,
					via: ['R1', 'R2']
				}),
				position('STALE-1', {
					lastSeen: '2026-05-20T08:30:00Z',
					lastDirectSeen: undefined,
					lon: 12.5
				})
			],
			NOW
		);

		expect(segments.map((segment) => segment.id)).toEqual([
			'MYCALL-1->DIRECT-1',
			'MYCALL-1->R2',
			'R1->ORIGIN-1',
			'R2->R1'
		]);
		expect(segments.some((segment) => segment.pathNodeIds.includes('STALE-1'))).toBe(false);
	});

	it('marks only selected node path segments highlighted', () => {
		const segments = buildActiveMapPathSegments(
			'MYCALL-1',
			[
				position('MYCALL-1', { lastDirectSeen: '2026-05-20T09:59:00Z' }),
				position('NODE-1', { lastDirectSeen: '2026-05-20T09:58:00Z', lon: 12.1 }),
				position('NODE-2', { lastDirectSeen: '2026-05-20T09:57:00Z', lon: 12.2 })
			],
			NOW,
			'NODE-2'
		);

		expect(segments.find((segment) => segment.id === 'MYCALL-1->NODE-1')?.highlighted).toBe(false);
		expect(segments.find((segment) => segment.id === 'MYCALL-1->NODE-2')?.highlighted).toBe(true);
	});

	it('highlights every active path that reaches the selected node', () => {
		const segments = buildActiveMapPathSegments(
			'MYCALL-1',
			[
				position('MYCALL-1', { lastDirectSeen: '2026-05-20T09:59:00Z' }),
				position('R1', { lastDirectSeen: '2026-05-20T09:58:00Z', lon: 12.1 }),
				position('R2', { lastDirectSeen: '2026-05-20T09:57:00Z', lon: 12.2 }),
				position('ORIGIN-1', {
					lastSeen: '2026-05-20T09:56:00Z',
					lon: 12.3,
					via: ['R1', 'R2']
				})
			],
			NOW,
			'R1'
		);

		expect(highlightedIds(segments)).toEqual(['MYCALL-1->R1', 'MYCALL-1->R2', 'R2->R1']);
	});

	it('highlights alternate paths through undirected active interconnections up to five hops', () => {
		const segments = buildActiveMapPathSegments(
			'MYCALL-1',
			[
				position('MYCALL-1', { lastDirectSeen: '2026-05-20T09:59:00Z' }),
				position('A-1', { lastDirectSeen: '2026-05-20T09:58:00Z', lon: 12.1 }),
				position('B-1', { lastDirectSeen: '2026-05-20T09:57:00Z', lon: 12.2 }),
				position('C-1', {
					lastSeen: '2026-05-20T09:56:00Z',
					lon: 12.3,
					via: ['A-1', 'B-1']
				})
			],
			NOW,
			'B-1'
		);

		expect(highlightedIds(segments)).toEqual(['B-1->A-1', 'MYCALL-1->A-1', 'MYCALL-1->B-1']);
	});
});

function highlightedIds(segments: ReturnType<typeof buildActiveMapPathSegments>): string[] {
	return segments.filter((segment) => segment.highlighted).map((segment) => segment.id);
}

function position(source: string, overrides: Partial<MapPosition> = {}): MapPosition {
	return {
		id: source,
		source,
		lat: 45,
		lon: 12,
		updatedAt: '2026-05-20T10:00:00Z',
		lastSeen: '2026-05-20T09:59:00Z',
		via: [],
		...overrides
	};
}
