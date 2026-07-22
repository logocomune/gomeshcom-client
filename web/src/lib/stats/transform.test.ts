import { describe, it, expect } from 'vitest';
import { buildChannelTable } from './transform';
import type { Bucket } from '$lib/api/stats';

function bucket(channels: Record<string, number>): Bucket {
	return {
		hour: 0,
		dm: 0,
		dm_ack: 0,
		public: 0,
		telemetry: 0,
		position: 0,
		errors: 0,
		total: 0,
		channels,
		distance_km: {}
	};
}

describe('buildChannelTable', () => {
	it('aggregates all dm: entries into one DM row', () => {
		const rows = buildChannelTable([
			bucket({ 'dm:QQ1ABC': 3, 'dm:QQ1DEF': 7, 'ch:9': 1 })
		]);
		const dm = rows.find((r) => r.kind === 'dm');
		expect(dm).toBeDefined();
		expect(dm!.count).toBe(10);
		expect(dm!.key).toBe('dm');
		expect(dm!.label).toBe('✉️ DM');
		// only one DM row
		expect(rows.filter((r) => r.kind === 'dm')).toHaveLength(1);
	});

	it('aggregates dm: entries across multiple buckets', () => {
		const rows = buildChannelTable([
			bucket({ 'dm:QQ1ABC': 4 }),
			bucket({ 'dm:QQ1ABC': 2, 'dm:QQ1XYZ': 5 })
		]);
		const dm = rows.find((r) => r.kind === 'dm');
		expect(dm!.count).toBe(11);
	});

	it('labels known channel with note from KNOWN_GROUPS', () => {
		const rows = buildChannelTable([bucket({ 'ch:222': 5 })]);
		const ch = rows.find((r) => r.key === 'ch:222');
		expect(ch).toBeDefined();
		expect(ch!.label).toBe('🇮🇹 CH. 222 - Italy');
	});

	it('labels known local channel with correct note', () => {
		const rows = buildChannelTable([bucket({ 'ch:9': 2 })]);
		const ch = rows.find((r) => r.key === 'ch:9');
		expect(ch!.label).toBe('📍 CH. 9 - Local group');
	});

	it('labels unknown channel with CH. N only', () => {
		const rows = buildChannelTable([bucket({ 'ch:99999': 1 })]);
		const ch = rows.find((r) => r.key === 'ch:99999');
		expect(ch!.label).toBe('🌍 CH. 99999');
	});

	it('labels broadcast with satellite emoji', () => {
		const rows = buildChannelTable([bucket({ broadcast: 4 })]);
		const bc = rows.find((r) => r.kind === 'broadcast');
		expect(bc!.label).toBe('📡 Broadcast');
	});

	it('no DM row when no dm: keys present', () => {
		const rows = buildChannelTable([bucket({ 'ch:9': 3, broadcast: 1 })]);
		expect(rows.find((r) => r.kind === 'dm')).toBeUndefined();
	});

	it('sorts rows by count descending', () => {
		const rows = buildChannelTable([
			bucket({ 'ch:9': 1, 'dm:X': 10, broadcast: 5 })
		]);
		expect(rows[0].count).toBeGreaterThanOrEqual(rows[1].count);
		expect(rows[1].count).toBeGreaterThanOrEqual(rows[2].count);
	});

	it('returns empty array for empty buckets', () => {
		expect(buildChannelTable([])).toEqual([]);
	});
});
