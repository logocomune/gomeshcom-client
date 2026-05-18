import { describe, expect, it } from 'vitest';
import { buildOwnMarkerTooltipHtml, buildTooltipHtml } from './map-tooltip';
import type { MapPosition } from './types';

const NOW = new Date('2026-05-18T10:00:00Z').getTime();

function makePosition(overrides: Partial<MapPosition> = {}): MapPosition {
	return {
		id: 'test',
		source: 'TEST-1',
		lat: 45,
		lon: 12,
		updatedAt: '2026-05-18T09:00:00Z',
		lastSeen: '2026-05-18T09:45:00Z',
		...overrides
	};
}

describe('buildTooltipHtml', () => {
	it('includes last seen and first seen in hover tooltip', () => {
		const html = buildTooltipHtml(
			makePosition({
				firstSeen: '2026-05-17T08:30:00Z',
				lastSeen: '2026-05-18T09:45:00Z'
			}),
			NOW
		);

		expect(html).toContain('last seen');
		expect(html).toContain('18/05/2026');
		expect(html).toContain('first seen');
		expect(html).toContain('17/05/2026');
	});

	it('escapes user-controlled fields', () => {
		const html = buildTooltipHtml(
			makePosition({
				source: '<CALL&1>',
				firstSeen: '<bad>',
				lastSeen: '2026-05-18T09:45:00Z',
				via: ['A&B']
			}),
			NOW
		);

		expect(html).toContain('&lt;CALL&amp;1&gt;');
		expect(html).toContain('&lt;bad&gt;');
		expect(html).toContain('A&amp;B');
		expect(html).not.toContain('<CALL&1>');
	});
});

describe('buildOwnMarkerTooltipHtml', () => {
	it('shows only callsign and device name for own marker', () => {
		const html = buildOwnMarkerTooltipHtml(
			makePosition({
				hwId: '10',
				firstSeen: '2026-05-17T08:30:00Z',
				lastSeen: '2026-05-18T09:45:00Z'
			})
		);

		expect(html).toBe('<strong>TEST-1</strong><br>Heltec V2.1');
		expect(html).not.toContain('last seen');
		expect(html).not.toContain('first seen');
	});

	it('falls back to callsign when own marker has no device name', () => {
		expect(buildOwnMarkerTooltipHtml(makePosition({ hwId: undefined }))).toBe('TEST-1');
	});
});
