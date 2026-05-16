import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	applyLiveFreshness,
	connectEvents,
	eventDetail,
	eventJSON,
	eventSummary,
	freshnessDeltasFromEvent,
	messageKind,
	mergeMapPositions,
	packetBadge,
	packetIcon,
	positionsFromRecords,
	positionsFromEvents,
	prependEvent,
	stationCallsignFromEvent,
	splitSourcePath
} from './events';
import type { StreamEvent } from './types';
import type { MapPosition } from '$lib/map/types';

function event(type: string, data: unknown): StreamEvent {
	return timedEvent(type, '2026-05-14T19:00:00Z', data);
}

function timedEvent(type: string, receivedAt: string, data: unknown): StreamEvent {
	return {
		id: 'id',
		type,
		receivedAt,
		data
	};
}

describe('event helpers', () => {
	it('summarizes message packets', () => {
		expect.assertions(5);

		const received = event('packet.received', {
			remote_addr: '192.168.1.53:1799',
			packet: { type: 'msg', src: 'QQ1ABC-1', dst: '*', msg: 'hello' }
		});

		expect(packetBadge(received)).toBe('msg');
		expect(packetIcon(received)).toBe('✉');
		expect(eventSummary(received)).toBe('QQ1ABC-1 -> Broadcast: hello');
		expect(eventDetail(received)).toBe('Message · no id');
		expect(eventJSON(received)).toContain('"msg": "hello"');
	});

	it('keeps newest events first', () => {
		expect.assertions(1);

		const first = event('packet.error', 'bad json');
		const second = event('message.created', { msg: 'hello' });

		expect(prependEvent([first], second)).toEqual([second, first]);
	});

	it('summarizes station identity app event', () => {
		expect.assertions(3);

		const identity = event('station.identity', { callsign: 'QQ1ABC-7' });

		expect(eventSummary(identity)).toBe('station.identity');
		expect(eventJSON(identity)).toContain('QQ1ABC-7');
		expect(stationCallsignFromEvent(identity)).toBe('QQ1ABC-7');
	});

	it('falls back to local node packet source for station callsign', () => {
		expect.assertions(1);

		const received = event('packet.received', {
			packet: { type: 'pos', src_type: 'node', src: 'QQ1XYZ-2,RELAY', lat: 48.1, long: 16.3 }
		});

		expect(stationCallsignFromEvent(received)).toBe('QQ1XYZ-2');
	});

	it('adds position hardware human name to event detail', () => {
		expect.assertions(1);

		const received = event('packet.received', {
			packet: { type: 'pos', src: 'QQ1XYZ-2', hw_id: 42, lat: 48.1, long: 16.3 }
		});

		expect(eventDetail(received)).toBe('Heltec Stick V3');
	});

	it('splits relay path and detects system messages', () => {
		expect.assertions(3);

		expect(splitSourcePath('QQ1XAR-32,QQ5AKT-10,QQ5CND-10')).toEqual({
			origin: 'QQ1XAR-32',
			relays: ['QQ5AKT-10', 'QQ5CND-10']
		});
		expect(messageKind('{CET}2026-05-14 19:19:22')).toEqual({
			kind: 'time',
			label: 'Network time',
			icon: '◷'
		});
		expect(messageKind('ack123').kind).toBe('ack');
	});

	it('extracts latest map positions from events', () => {
		expect.assertions(1);

		const first = timedEvent('packet.received', '2026-05-14T19:00:00Z', {
			packet: { type: 'pos', src: 'QQ5EKX-11,RELAY', lat: 43.5, long: 10.3, msg_id: '1' }
		});
		const second = timedEvent('packet.received', '2026-05-14T19:01:00Z', {
			packet: { type: 'pos', src: 'QQ5EKX-11,RELAY', lat: 44.5, long: 11.3, msg_id: '2' }
		});

		expect(positionsFromEvents([first, second])).toEqual([
			{
				id: '2',
				source: 'QQ5EKX-11',
				lat: 44.5,
				lon: 11.3,
				altitude: undefined,
				battery: undefined,
				rssi: undefined,
				snr: undefined,
				lastDirectSeen: undefined,
				via: ['RELAY'],
				lastSeen: '2026-05-14T19:01:00Z',
				updatedAt: '2026-05-14T19:01:00Z'
			}
		]);
	});

	it('keeps newest map position even when events arrive newest first', () => {
		expect.assertions(1);

		const newest = timedEvent('packet.received', '2026-05-14T19:01:00Z', {
			packet: { type: 'pos', src: 'QQ5EKX-11', lat: 44.5, long: 11.3, msg_id: '2' }
		});
		const older = timedEvent('packet.received', '2026-05-14T19:00:00Z', {
			packet: { type: 'pos', src: 'QQ5EKX-11', lat: 43.5, long: 10.3, msg_id: '1' }
		});

		expect(positionsFromEvents([newest, older])[0].lat).toBe(44.5);
	});

	it('maps stored positions and lets live events override them', () => {
		expect.assertions(1);

		const stored = positionsFromRecords({
			'QQ1ABC-1': {
				lat: 48.1,
				lng: 16.3,
				alt: 123,
				firstseen: '2026-05-14T19:00:00Z',
				lastseen: '2026-05-14T19:10:00Z',
				rssi: -90,
				snr: 8,
				via: ['RELAY-1']
			}
		});
		const live = positionsFromEvents([
			timedEvent('packet.received', '2026-05-14T19:11:00Z', {
				packet: { type: 'pos', src: 'QQ1ABC-1', lat: 48.2, long: 16.4, alt: 130, msg_id: '2' }
			})
		]);

		expect(mergeMapPositions(stored, live)).toEqual([
			{
				id: '2',
				source: 'QQ1ABC-1',
				lat: 48.2,
				lon: 16.4,
				altitude: 130,
				battery: undefined,
				rssi: undefined,
				snr: undefined,
				lastDirectSeen: '2026-05-14T19:11:00Z',
				via: [],
				lastSeen: '2026-05-14T19:11:00Z',
				updatedAt: '2026-05-14T19:11:00Z'
			}
		]);
	});

	it('propagates hw_id from SSE packet to MapPosition', () => {
		expect.assertions(2);

		const withHw = timedEvent('packet.received', '2026-05-14T19:00:00Z', {
			packet: { type: 'pos', src: 'QQ5EKX-11', lat: 43.5, long: 10.3, hw_id: 4, msg_id: '1' }
		});
		const withoutHw = timedEvent('packet.received', '2026-05-14T19:00:00Z', {
			packet: { type: 'pos', src: 'QQ1ABC-1', lat: 48.1, long: 16.3, msg_id: '2' }
		});

		const [pos1] = positionsFromEvents([withHw]);
		const [pos2] = positionsFromEvents([withoutHw]);

		expect(pos1.hwId).toBe('4');
		expect(pos2.hwId).toBeUndefined();
	});

	it('propagates hw_id from stored position records to MapPosition', () => {
		expect.assertions(2);

		const positions = positionsFromRecords({
			'QQ5EKX-11': {
				lat: 43.5,
				lng: 10.3,
				alt: 367,
				hw_id: '4',
				firstseen: '2026-05-14T19:00:00Z',
				lastseen: '2026-05-14T19:10:00Z',
				rssi: -108,
				snr: 1
			},
			'QQ1ABC-1': {
				lat: 48.1,
				lng: 16.3,
				alt: 123,
				firstseen: '2026-05-14T19:00:00Z',
				lastseen: '2026-05-14T19:10:00Z',
				rssi: -90,
				snr: 8
			}
		});

		const i5ekx = positions.find((p) => p.source === 'QQ5EKX-11');
		const oe1abc = positions.find((p) => p.source === 'QQ1ABC-1');

		expect(i5ekx?.hwId).toBe('4');
		expect(oe1abc?.hwId).toBeUndefined();
	});
});

describe('freshnessDeltasFromEvent', () => {
	function event(type: string, data: unknown): StreamEvent {
		return { id: 'x', type, receivedAt: '2026-05-15T10:00:00Z', data };
	}

	it('returns empty for non-packet events', () => {
		expect(freshnessDeltasFromEvent(event('station.identity', {}))).toEqual([]);
	});

	it('direct packet → single direct delta for origin', () => {
		const e = event('packet.received', {
			packet: { type: 'pos', src: 'A-1', rssi: -80, snr: 5 }
		});
		const deltas = freshnessDeltasFromEvent(e);
		expect(deltas).toHaveLength(1);
		expect(deltas[0]).toMatchObject({ source: 'A-1', mode: 'direct', rssi: -80, snr: 5 });
	});

	it('indirect packet → indirect delta for origin + direct delta for last relay', () => {
		const e = event('packet.received', {
			packet: { type: 'msg', src: 'A-1,R1,R2', rssi: -90, snr: 3 }
		});
		const deltas = freshnessDeltasFromEvent(e);
		expect(deltas).toHaveLength(2);
		expect(deltas[0]).toMatchObject({ source: 'A-1', mode: 'indirect' });
		expect(deltas[1]).toMatchObject({ source: 'R2', mode: 'direct', rssi: -90, snr: 3 });
	});

	it('works for msg and tele packet types', () => {
		const msg = event('packet.received', {
			packet: { type: 'msg', src: 'A-1', rssi: -70, snr: 4 }
		});
		const tele = event('packet.received', {
			packet: { type: 'tele', src: 'B-1,RELAY', rssi: -85, snr: 2 }
		});
		expect(freshnessDeltasFromEvent(msg)[0].mode).toBe('direct');
		expect(freshnessDeltasFromEvent(tele)[1].source).toBe('RELAY');
	});
});

describe('applyLiveFreshness', () => {
	const t0 = '2026-05-15T10:00:00Z';
	const t1 = '2026-05-15T10:05:00Z';
	const t2 = '2026-05-15T10:10:00Z';

	function stored(source: string, extra: Partial<MapPosition> = {}): MapPosition {
		return {
			id: source,
			source,
			lat: 43,
			lon: 10,
			updatedAt: t0,
			lastSeen: t0,
			...extra
		};
	}

	function packetEvent(
		src: string,
		type: string,
		extra: Record<string, unknown> = {}
	): StreamEvent {
		return {
			id: 'x',
			type: 'packet.received',
			receivedAt: t1,
			data: { packet: { type, src, ...extra } }
		};
	}

	it('returns stored positions unchanged when no events', () => {
		const s = [stored('A-1')];
		const result = applyLiveFreshness(s, []);
		expect(result).toHaveLength(1);
		expect(result[0].source).toBe('A-1');
	});

	it('direct msg updates lastSeen + lastDirectSeen + rssi/snr of existing node', () => {
		const s = [stored('A-1', { lastSeen: t0 })];
		const events: StreamEvent[] = [packetEvent('A-1', 'msg', { rssi: -80, snr: 5 })];
		const result = applyLiveFreshness(s, events);
		const a = result.find((p) => p.source === 'A-1')!;
		expect(a.lastSeen).toBe(t1);
		expect(a.lastDirectSeen).toBe(t1);
		expect(a.rssi).toBe(-80);
		expect(a.snr).toBe(5);
	});

	it('indirect msg updates lastSeen of origin, lastDirectSeen/rssi/snr of last relay', () => {
		const s = [stored('ORIGIN-1'), stored('RELAY-1', { lastDirectSeen: t0 })];
		const events: StreamEvent[] = [packetEvent('ORIGIN-1,RELAY-1', 'msg', { rssi: -95, snr: 3 })];
		const result = applyLiveFreshness(s, events);

		const origin = result.find((p) => p.source === 'ORIGIN-1')!;
		expect(origin.lastSeen).toBe(t1);
		expect(origin.lastDirectSeen).toBeUndefined();

		const relay = result.find((p) => p.source === 'RELAY-1')!;
		expect(relay.lastDirectSeen).toBe(t1);
		expect(relay.rssi).toBe(-95);
		expect(relay.snr).toBe(3);
	});

	it('skips freshness update for node not in stored (no record)', () => {
		const s = [stored('A-1')];
		const events: StreamEvent[] = [packetEvent('GHOST-1', 'msg', { rssi: -80, snr: 5 })];
		const result = applyLiveFreshness(s, events);
		expect(result.find((p) => p.source === 'GHOST-1')).toBeUndefined();
	});

	it('indirect relay skip if relay has no record', () => {
		const s = [stored('A-1')];
		const events: StreamEvent[] = [packetEvent('A-1,GHOST-RELAY', 'msg', { rssi: -80, snr: 5 })];
		const result = applyLiveFreshness(s, events);
		expect(result.find((p) => p.source === 'GHOST-RELAY')).toBeUndefined();
		// origin should still get lastSeen updated
		expect(result.find((p) => p.source === 'A-1')?.lastSeen).toBe(t1);
	});

	it('pos packet updates coords of origin + freshness of last relay', () => {
		const s = [stored('ORIGIN-1'), stored('RELAY-1')];
		const posEvent: StreamEvent = {
			id: 'x',
			type: 'packet.received',
			receivedAt: t1,
			data: {
				packet: {
					type: 'pos',
					src: 'ORIGIN-1,RELAY-1',
					lat: 48.5,
					long: 16.5,
					rssi: -88,
					snr: 4
				}
			}
		};
		const result = applyLiveFreshness(s, [posEvent]);

		const origin = result.find((p) => p.source === 'ORIGIN-1')!;
		expect(origin.lat).toBe(48.5);
		expect(origin.lon).toBe(16.5);
		expect(origin.lastDirectSeen).toBeUndefined();

		const relay = result.find((p) => p.source === 'RELAY-1')!;
		expect(relay.lastDirectSeen).toBe(t1);
		expect(relay.rssi).toBe(-88);
	});

	it('does not regress lastSeen with older event', () => {
		const s = [stored('A-1', { lastSeen: t2 })];
		const events: StreamEvent[] = [
			{
				id: 'x',
				type: 'packet.received',
				receivedAt: t0,
				data: { packet: { type: 'msg', src: 'A-1' } }
			}
		];
		const result = applyLiveFreshness(s, events);
		expect(result.find((p) => p.source === 'A-1')?.lastSeen).toBe(t2);
	});
});

describe('connectEvents auth flow', () => {
	afterEach(() => {
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	it('switches to unauthenticated after SSE error and 401 session status', async () => {
		const states: string[] = [];
		let activeSource:
			| {
					withCredentials: boolean;
					emitError: () => void;
			  }
			| undefined;

		vi.stubGlobal(
			'EventSource',
			class extends FakeEventSource {
				constructor(url: string, init?: EventSourceInit) {
					super(url, init);
					activeSource = this;
				}
			}
		);
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(
			new Response(JSON.stringify({ required: true, authenticated: false }), {
				status: 401,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		const stop = connectEvents({
			onState: (state) => states.push(state),
			onEvent: () => undefined
		});

		expect(activeSource).toBeDefined();
		if (activeSource === undefined) {
			throw new Error('EventSource not created');
		}
		const source = activeSource;
		expect(source.withCredentials).toBe(true);
		source.emitError();
		await vi.waitFor(() => expect(states).toContain('unauthenticated'));

		stop();
	});
});

class FakeEventSource {
	url: string;
	withCredentials: boolean;
	onopen: (() => void) | null = null;
	onerror: (() => void) | null = null;
	private listeners = new Map<string, Array<(event: MessageEvent<string>) => void>>();

	constructor(url: string, init?: EventSourceInit) {
		this.url = url;
		this.withCredentials = init?.withCredentials ?? false;
	}

	addEventListener(type: string, listener: (event: MessageEvent<string>) => void) {
		const list = this.listeners.get(type) ?? [];
		list.push(listener);
		this.listeners.set(type, list);
	}

	close() {
		return undefined;
	}

	emitError() {
		this.onerror?.();
	}
}
