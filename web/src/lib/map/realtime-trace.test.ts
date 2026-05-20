import { describe, expect, it } from 'vitest';
import type { StreamEvent } from '$lib/api/types';
import type { MapPosition } from './types';
import { buildRealtimeDmTraceSegments, DM_TRACE_TTL_MS } from './realtime-trace';

function makePosition(overrides: Partial<MapPosition>): MapPosition {
	return {
		id: overrides.id ?? overrides.source ?? 'id',
		source: overrides.source ?? 'QQ0AAA',
		lat: overrides.lat ?? 45,
		lon: overrides.lon ?? 12,
		updatedAt: overrides.updatedAt ?? '2026-05-20T10:00:00Z',
		lastSeen: overrides.lastSeen ?? '2026-05-20T10:00:00Z',
		...overrides
	};
}

function makeMsgEvent(receivedAt: string, src: string, dst: string, msg = 'ciao'): StreamEvent {
	return {
		id: `${src}-${dst}-${receivedAt}`,
		type: 'packet.received',
		receivedAt,
		data: {
			packet: {
				type: 'msg',
				src,
				dst,
				msg
			}
		}
	};
}

describe('buildRealtimeDmTraceSegments', () => {
	it('builds dashed-route segments across relays for DM events', () => {
		const nowMs = Date.parse('2026-05-20T10:00:30Z');
		const positions = [
			makePosition({ source: 'QQ1AAA', lat: 45.0, lon: 12.0 }),
			makePosition({ source: 'QQ2BBB', lat: 45.1, lon: 12.1 }),
			makePosition({ source: 'QQ3CCC', lat: 45.2, lon: 12.2 }),
			makePosition({ source: 'QQ4DDD', lat: 45.3, lon: 12.3 })
		];
		const events: StreamEvent[] = [
			makeMsgEvent('2026-05-20T10:00:00Z', 'QQ1AAA,QQ2BBB,QQ3CCC', 'QQ4DDD', 'hello')
		];

		const segments = buildRealtimeDmTraceSegments(positions, events, nowMs);
		const paths = segments.map((segment) => `${segment.from.source}>${segment.to.source}`);

		expect(paths).toContain('QQ1AAA>QQ2BBB');
		expect(paths).toContain('QQ2BBB>QQ3CCC');
		expect(paths).toContain('QQ3CCC>QQ4DDD');
		expect(segments).toHaveLength(3);
	});

	it('drops expired traces after 45 seconds', () => {
		const at = '2026-05-20T10:00:00Z';
		const positions = [makePosition({ source: 'QQ1AAA' }), makePosition({ source: 'QQ2BBB' })];
		const events: StreamEvent[] = [makeMsgEvent(at, 'QQ1AAA', 'QQ2BBB')];

		const nowMs = Date.parse(at) + DM_TRACE_TTL_MS + 1;
		expect(buildRealtimeDmTraceSegments(positions, events, nowMs)).toHaveLength(0);
	});

	it('includes ack packets in realtime DM traces', () => {
		const positions = [
			makePosition({ source: 'IU5RTR-02' }),
			makePosition({ source: 'IZ5CND-10' }),
			makePosition({ source: 'IU5PMP-1' })
		];
		const nowMs = Date.parse('2026-05-20T20:31:50Z');
		const events: StreamEvent[] = [
			makeMsgEvent(
				'2026-05-20T20:31:37.162625521Z',
				'IU5RTR-02,IZ5CND-10',
				'IU5PMP-1',
				'IU5PMP-1 :ack950'
			)
		];

		const segments = buildRealtimeDmTraceSegments(positions, events, nowMs);
		const paths = segments.map((segment) => segment.from.source + '>' + segment.to.source);
		expect(paths).toContain('IU5RTR-02>IZ5CND-10');
		expect(paths).toContain('IZ5CND-10>IU5PMP-1');
		expect(segments).toHaveLength(2);
		expect(segments.every((segment) => segment.isAck)).toBe(true);
	});
	it('ignores broadcast/channel and system control traffic', () => {
		const positions = [makePosition({ source: 'QQ1AAA' }), makePosition({ source: 'QQ2BBB' })];
		const nowMs = Date.parse('2026-05-20T10:00:10Z');
		const events: StreamEvent[] = [
			makeMsgEvent('2026-05-20T10:00:00Z', 'QQ1AAA', '*', 'broadcast'),
			makeMsgEvent('2026-05-20T10:00:00Z', 'QQ1AAA', '2', 'channel'),
			makeMsgEvent('2026-05-20T10:00:00Z', 'QQ1AAA', 'QQ2BBB', '{CET}control')
		];

		expect(buildRealtimeDmTraceSegments(positions, events, nowMs)).toHaveLength(0);
	});
});
