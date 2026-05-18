import { env } from '$env/dynamic/public';
import { apiFetch, getSessionStatus } from './auth';
import type {
	ConnectionState,
	MeshcomPacket,
	PacketReceivedPayload,
	PositionMap,
	StationIdentity,
	StreamEvent
} from './types';
import { hardwareHumanName } from './hardware';
import type { MapPosition } from '$lib/map/types';

export const API_BASE = env.PUBLIC_API_BASE || '/api';

const MAX_EVENTS = 300;

// How long without any SSE data before we consider the connection dead.
// Backend heartbeat interval is 15s; give it 2× headroom.
const HEARTBEAT_TIMEOUT_MS = 30_000;

export function connectEvents(handlers: {
	onState: (state: ConnectionState) => void;
	onEvent: (event: StreamEvent) => void;
	onPositions?: (positions: MapPosition[]) => void;
	onStation?: (station: StationIdentity) => void;
}): () => void {
	let source: EventSource | null = null;
	let retryTimer: ReturnType<typeof setTimeout> | null = null;
	let heartbeatTimer: ReturnType<typeof setTimeout> | null = null;
	let closed = false;

	function resetHeartbeat() {
		if (heartbeatTimer !== null) clearTimeout(heartbeatTimer);
		heartbeatTimer = setTimeout(() => {
			// No data received within timeout — treat as silent disconnect.
			source?.close();
			source = null;
			scheduleReconnect();
		}, HEARTBEAT_TIMEOUT_MS);
	}

	function clearHeartbeat() {
		if (heartbeatTimer !== null) {
			clearTimeout(heartbeatTimer);
			heartbeatTimer = null;
		}
	}

	function generateId(): string {
		if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
			return crypto.randomUUID();
		}
		return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
	}

	function push(type: string, data: unknown) {
		resetHeartbeat();
		handlers.onEvent({
			id: generateId(),
			type,
			receivedAt: new Date().toISOString(),
			data
		});
	}

	function parseEvent(type: string, event: MessageEvent<string>) {
		try {
			push(type, JSON.parse(event.data));
		} catch {
			push(type, event.data);
		}
	}

	function scheduleReconnect() {
		clearHeartbeat();
		if (closed || retryTimer !== null) return;
		handlers.onState('disconnected');
		retryTimer = setTimeout(() => {
			retryTimer = null;
			open();
		}, 2000);
	}

	function open() {
		if (closed) return;
		handlers.onState('connecting');
		source = new EventSource(`${API_BASE}/events`, { withCredentials: true });

		source.onopen = () => {
			handlers.onState('connected');
			resetHeartbeat();
		};
		source.addEventListener('positions.snapshot', (event) => {
			resetHeartbeat();
			try {
				handlers.onPositions?.(
					positionsFromRecords(JSON.parse((event as MessageEvent<string>).data))
				);
			} catch {
				push('positions.snapshot', (event as MessageEvent<string>).data);
			}
		});
		source.addEventListener('station.identity', (event) => {
			resetHeartbeat();
			try {
				const station = JSON.parse((event as MessageEvent<string>).data) as StationIdentity;
				handlers.onStation?.(station);
			} catch {
				// ignore malformed station.identity
			}
		});
		source.addEventListener('packet.received', (event) =>
			parseEvent('packet.received', event as MessageEvent<string>)
		);
		source.addEventListener('packet.error', (event) =>
			parseEvent('packet.error', event as MessageEvent<string>)
		);
		source.addEventListener('message.created', (event) =>
			parseEvent('message.created', event as MessageEvent<string>)
		);
		source.addEventListener('message.failed', (event) =>
			parseEvent('message.failed', event as MessageEvent<string>)
		);
		source.addEventListener('heartbeat', () => resetHeartbeat());
		source.onerror = () => {
			source?.close();
			source = null;
			void getSessionStatus()
				.then((status) => {
					if (status.required && !status.authenticated) {
						handlers.onState('unauthenticated');
						return;
					}
					scheduleReconnect();
				})
				.catch(() => scheduleReconnect());
		};
	}

	open();

	return () => {
		closed = true;
		clearHeartbeat();
		if (retryTimer !== null) clearTimeout(retryTimer);
		source?.close();
		source = null;
		handlers.onState('disconnected');
	};
}

export function prependEvent(events: StreamEvent[], event: StreamEvent): StreamEvent[] {
	return [event, ...events].slice(0, MAX_EVENTS);
}

export function packetFromEvent(event: StreamEvent): MeshcomPacket | null {
	if (event.type !== 'packet.received') return null;
	const payload = event.data as PacketReceivedPayload;
	return payload.packet ?? null;
}

export function eventSummary(event: StreamEvent): string {
	if (event.type === 'packet.error') {
		return typeof event.data === 'string' ? event.data : 'Parse error';
	}

	const packet = packetFromEvent(event);
	if (!packet) return event.type;
	const source = splitSourcePath(packet.src);

	if (packet.type === 'msg') {
		const destination = packet.dst === '*' ? 'Broadcast' : (packet.dst ?? 'unknown');
		return packet.msg
			? `${source.origin} -> ${destination}: ${stripMessagePrefix(packet.msg)}`
			: 'Message';
	}

	if (packet.type === 'pos') {
		const position =
			packet.lat != null && packet.long != null
				? `${packet.lat.toFixed(4)}, ${packet.long.toFixed(4)}`
				: 'no position';
		return `${source.origin} position ${position}`;
	}

	if (packet.type === 'tele') {
		const parts = [
			packet.batt != null ? `batt ${packet.batt}%` : '',
			packet.temp1 != null ? `temp ${packet.temp1}C` : '',
			packet.hum != null ? `hum ${packet.hum}%` : ''
		].filter(Boolean);
		return `${source.origin} telemetry ${parts.join(' ')}`.trim();
	}

	return `${source.origin} · ${packet.src_type ?? 'raw'}`;
}

export function eventDetail(event: StreamEvent): string {
	if (event.type === 'packet.error') return 'Datagram rejected by parser';

	const packet = packetFromEvent(event);
	if (!packet) return 'Application event';
	const source = splitSourcePath(packet.src);

	if (packet.type === 'msg') {
		const id = packet.msg_id ? `id ${packet.msg_id}` : 'no id';
		const kind = messageKind(packet.msg).label;
		const relays = source.relays.length > 0 ? `via ${source.relays.join(', ')}` : '';
		const quality = formatQuality(packet);
		return [kind, relays, id, quality].filter(Boolean).join(' · ');
	}

	if (packet.type === 'pos') {
		const relays = source.relays.length > 0 ? `via ${source.relays.join(', ')}` : '';
		const hardware = hardwareHumanName(packet.hw_id);
		const altitude = packet.alt != null ? `${packet.alt} m` : '';
		const battery = packet.batt != null ? `${packet.batt}% battery` : '';
		const quality = formatQuality(packet);
		return [relays, hardware, altitude, battery, quality].filter(Boolean).join(' · ');
	}

	if (packet.type === 'tele') {
		const pressure =
			packet.qnh != null
				? `QNH ${packet.qnh} hPa`
				: packet.qfe != null
					? `QFE ${packet.qfe} hPa`
					: '';
		const gas = packet.gas != null ? `gas ${packet.gas}` : '';
		const co2 = packet.co2 != null ? `CO2 ${packet.co2} ppm` : '';
		return [pressure, gas, co2].filter(Boolean).join(' · ');
	}

	return packet.src_type ? `source ${packet.src_type}` : '';
}

export function packetBadge(event: StreamEvent): string {
	const packet = packetFromEvent(event);
	if (packet?.type) return packet.type;
	if (event.type === 'packet.error') return 'error';
	if (packet) return String(packet.src_type ?? 'raw');
	return 'raw';
}

export function packetIcon(event: StreamEvent): string {
	const badge = packetBadge(event);
	const packet = packetFromEvent(event);
	if (badge === 'msg') return messageKind(packet?.msg).icon;
	if (badge === 'pos') return '⌖';
	if (badge === 'tele') return '▣';
	if (badge === 'error') return '!';
	return '•';
}

export function eventJSON(event: StreamEvent): string {
	return JSON.stringify(event.data, null, 2);
}

export function stationCallsignFromEvent(event: StreamEvent): string | null {
	if (event.type === 'station.identity') {
		const data = event.data as Partial<StationIdentity>;
		return typeof data.callsign === 'string' && data.callsign !== '' ? data.callsign : null;
	}

	const packet = packetFromEvent(event);
	if (packet?.src_type !== 'node') return null;
	return splitSourcePath(packet.src).origin;
}

function formatQuality(packet: MeshcomPacket): string {
	const parts = [
		packet.rssi != null ? `${packet.rssi} dBm` : '',
		packet.snr != null ? `SNR ${packet.snr}` : ''
	].filter(Boolean);
	return parts.join(' · ');
}

export function splitSourcePath(source?: string): { origin: string; relays: string[] } {
	const parts = (source ?? 'unknown')
		.split(',')
		.map((part) => part.trim())
		.filter(Boolean);
	return {
		origin: parts[0] ?? 'unknown',
		relays: parts.slice(1)
	};
}

export function messageKind(message?: string): { kind: string; label: string; icon: string } {
	const text = message ?? '';
	// ACK format: "ack571", ":ack571", or "CALLSIGN :ack571"
	if (/(?:^|\s):?ack\d+/i.test(text)) return { kind: 'ack', label: 'ACK', icon: '✓' };
	// REJ format: "rej571", ":rej571", or "CALLSIGN :rej571"
	if (/(?:^|\s):?rej\d+/i.test(text)) return { kind: 'reject', label: 'Reject', icon: '!' };
	if (text.startsWith('{CET}')) return { kind: 'time', label: 'Network time', icon: '◷' };
	if (text.startsWith('{SET}')) return { kind: 'config', label: 'Config', icon: '⚙' };
	return { kind: 'message', label: 'Message', icon: '✉' };
}

export function cleanMessage(message: string): string {
	return message
		.replace(/^\{CET\}/, '')
		.replace(/^\{SET\}/, '')
		.replace(/\{\d+\s*$/, '') // strip trailing MeshCom seq-ID suffix e.g. "{123"
		.trim();
}

// Strip only type prefixes, preserving the trailing {NNN seq marker for stream/debug views.
export function stripMessagePrefix(message: string): string {
	return message
		.replace(/^\{CET\}/, '')
		.replace(/^\{SET\}/, '')
		.trim();
}

export function msgSeqId(packet: MeshcomPacket | null): string | null {
	if (!packet) return null;
	// msg_id is a hex packet ID (e.g. "E4B9D23B"), not the seq number.
	// The seq number is always embedded as {NNN at the end of the message text.
	const match = (packet.msg ?? '').match(/\{(\d+)\s*$/);
	return match?.[1] ?? null;
}

export function ackSeqId(packet: MeshcomPacket | null): string | null {
	// Handles: "ack571", ":ack571", "IU5PMP-1 :ack571"
	const match = (packet?.msg ?? '').match(/(?:^|\s):?ack(\d+)/i);
	return match?.[1] ?? null;
}

export function rejSeqId(packet: MeshcomPacket | null): string | null {
	// Handles: "rej571", ":rej571", "IU5PMP-1 :rej571"
	const match = (packet?.msg ?? '').match(/(?:^|\s):?rej(\d+)/i);
	return match?.[1] ?? null;
}

export type AckSource = 'lora' | 'gateway';

export function ackSource(packet: MeshcomPacket | null): AckSource {
	return packet?.src_type === 'udp' ? 'gateway' : 'lora';
}

export function ackTargetCallsign(packet: MeshcomPacket | null): string | null {
	const match = (packet?.msg ?? '').match(/^(\S+?)\s*:ack\d+/i);
	return match?.[1] ?? null;
}

export type FreshnessDelta = {
	source: string;
	mode: 'direct' | 'indirect';
	rssi?: number;
	snr?: number;
	seenAt: string;
};

// freshnessDeltasFromEvent extracts last-hop freshness updates from any packet.received
// event regardless of packet type (msg / pos / tele).
// Direct packet → origin gets direct freshness.
// Indirect packet → origin gets indirect freshness (lastSeen only); last relay gets direct.
export function freshnessDeltasFromEvent(event: StreamEvent): FreshnessDelta[] {
	if (event.type !== 'packet.received') return [];
	const packet = packetFromEvent(event);
	if (!packet?.src) return [];

	const { origin, relays } = splitSourcePath(packet.src);
	const seenAt = event.receivedAt;
	const rssi = packet.rssi ?? undefined;
	const snr = packet.snr ?? undefined;

	if (relays.length === 0) {
		return [{ source: origin, mode: 'direct', rssi, snr, seenAt }];
	}
	const deltas: FreshnessDelta[] = [{ source: origin, mode: 'indirect', seenAt }];
	for (let i = 0; i < relays.length; i++) {
		const source = relays[i];
		if (i === relays.length - 1) {
			deltas.push({ source, mode: 'direct', rssi, snr, seenAt });
		} else {
			deltas.push({ source, mode: 'indirect', seenAt });
		}
	}
	return deltas;
}

// applyLiveFreshness merges stored positions with live SSE events.
// Coord updates come from pos packets; freshness (lastSeen, lastDirectSeen, rssi, snr)
// is applied from any packet type. Skip-if-no-record: freshness is never applied to a
// node that has no existing coord record.
export function applyLiveFreshness(stored: MapPosition[], events: StreamEvent[]): MapPosition[] {
	const bySource = new Map<string, MapPosition>();
	for (const position of stored) {
		bySource.set(position.source, { ...position });
	}

	for (const event of events) {
		// Coord update: pos packets upsert lat/lon/etc for origin.
		const coordPos = positionFromEvent(event);
		if (coordPos) {
			const current = bySource.get(coordPos.source);
				if (!current || coordPos.updatedAt >= current.updatedAt) {
					const merged: MapPosition = {
						...(current ?? coordPos),
						...coordPos,
						rssi: coordPos.rssi ?? current?.rssi,
						snr: coordPos.snr ?? current?.snr
					};
					// Indirect pos sets lastDirectSeen=undefined — preserve existing if better.
					if (
						current?.lastDirectSeen &&
					(!merged.lastDirectSeen || current.lastDirectSeen > merged.lastDirectSeen)
				) {
					merged.lastDirectSeen = current.lastDirectSeen;
				}
				bySource.set(coordPos.source, merged);
			} else if (
				coordPos.lastDirectSeen &&
				(!current.lastDirectSeen || coordPos.lastDirectSeen > current.lastDirectSeen)
			) {
				bySource.set(coordPos.source, { ...current, lastDirectSeen: coordPos.lastDirectSeen });
			}
		}

		// Freshness deltas: apply to any existing record (skip-if-no-record).
		for (const delta of freshnessDeltasFromEvent(event)) {
			const rec = bySource.get(delta.source);
			if (!rec) continue;
			if (delta.seenAt < (rec.lastSeen ?? '')) continue;

			if (delta.mode === 'direct') {
				bySource.set(delta.source, {
					...rec,
					lastSeen: delta.seenAt,
					lastDirectSeen: delta.seenAt,
					rssi: delta.rssi ?? rec.rssi,
					snr: delta.snr ?? rec.snr,
					updatedAt: delta.seenAt
				});
			} else {
				bySource.set(delta.source, {
					...rec,
					lastSeen: delta.seenAt,
					updatedAt: delta.seenAt
				});
			}
		}
	}

	return Array.from(bySource.values()).sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
}

export function positionFromEvent(event: StreamEvent): MapPosition | null {
	const packet = packetFromEvent(event);
	if (packet?.type !== 'pos' || packet.lat == null || packet.long == null) return null;
	const source = splitSourcePath(packet.src);
	const isDirect = source.relays.length === 0;
	return {
		id: packet.msg_id ?? `${source.origin}-${packet.lat}-${packet.long}`,
		source: source.origin,
		lat: packet.lat,
		lon: packet.long,
		altitude: packet.alt,
		battery: packet.batt,
		rssi: isDirect ? packet.rssi : undefined,
		snr: isDirect ? packet.snr : undefined,
		hwId: packet.hw_id != null ? String(packet.hw_id) : undefined,
		lastSeen: event.receivedAt,
		lastDirectSeen: isDirect ? event.receivedAt : undefined,
		via: source.relays,
		updatedAt: event.receivedAt
	};
}

export function positionsFromEvents(events: StreamEvent[]): MapPosition[] {
	const bySource = new Map<string, MapPosition>();
	for (const event of events) {
		const position = positionFromEvent(event);
		const current = position ? bySource.get(position.source) : null;
		if (position && (!current || position.updatedAt > current.updatedAt)) {
			bySource.set(position.source, position);
		}
	}
	return Array.from(bySource.values());
}

export async function fetchPositions(): Promise<MapPosition[]> {
	const response = await apiFetch(`${API_BASE}/positions`);
	if (!response.ok) return [];

	const records = ((await response.json()) as PositionMap | null) ?? {};
	return positionsFromRecords(records);
}

export function positionsFromRecords(records: PositionMap): MapPosition[] {
	return Object.entries(records).map(([source, record]) => ({
		id: source,
		source,
		lat: record.lat,
		lon: record.lng,
		altitude: record.alt,
		rssi: record.rssi,
		snr: record.snr,
		hwId: record.hw_id,
		firstSeen: record.firstseen,
		lastSeen: record.lastseen,
		lastDirectSeen: record.lastdirectseen,
		via: record.via,
		updatedAt: record.lastseen
	}));
}

export function mergeMapPositions(stored: MapPosition[], live: MapPosition[]): MapPosition[] {
	const bySource = new Map<string, MapPosition>();
	for (const position of stored) {
		bySource.set(position.source, position);
	}
	for (const position of live) {
		const current = bySource.get(position.source);
		if (!current || position.updatedAt >= current.updatedAt) {
			const merged = { ...position };
			if (
				current?.lastDirectSeen &&
				(!merged.lastDirectSeen || current.lastDirectSeen > merged.lastDirectSeen)
			) {
				merged.lastDirectSeen = current.lastDirectSeen;
			}
			bySource.set(position.source, merged);
		} else if (
			position.lastDirectSeen &&
			(!current.lastDirectSeen || position.lastDirectSeen > current.lastDirectSeen)
		) {
			bySource.set(position.source, { ...current, lastDirectSeen: position.lastDirectSeen });
		}
	}
	return Array.from(bySource.values()).sort((left, right) =>
		right.updatedAt.localeCompare(left.updatedAt)
	);
}

export function messageEvents(events: StreamEvent[]): StreamEvent[] {
	return events.filter((event) => packetFromEvent(event)?.type === 'msg');
}

export function messageChannels(events: StreamEvent[]): string[] {
	const channels = new Set<string>();
	for (const event of messageEvents(events)) {
		const packet = packetFromEvent(event);
		if (!packet?.dst) continue;
		const dst = packet.dst;
		if (dst === '*') {
			channels.add('Broadcast');
		} else if (/^\d+$/.test(dst)) {
			// pure numeric = group channel, not a callsign
			channels.add(dst);
		}
	}
	return Array.from(channels).sort((left, right) => left.localeCompare(right));
}

export function messageContacts(events: StreamEvent[]): string[] {
	const contacts = new Set<string>();
	for (const event of messageEvents(events)) {
		contacts.add(splitSourcePath(packetFromEvent(event)?.src).origin);
	}
	return Array.from(contacts).sort((left, right) => left.localeCompare(right));
}

export function dmEvents(
	events: StreamEvent[],
	isChannel: (dst: string) => boolean
): StreamEvent[] {
	return messageEvents(events).filter((event) => {
		const dst = packetFromEvent(event)?.dst;
		if (!dst || dst === '*') return false;
		return !isChannel(dst);
	});
}

export function dmContacts(
	events: StreamEvent[],
	isChannel: (dst: string) => boolean,
	stationCallsign: string
): string[] {
	if (!stationCallsign) return [];
	const contacts = new Set<string>();
	for (const event of dmEvents(events, isChannel)) {
		const packet = packetFromEvent(event);
		if (!packet) continue;
		const origin = splitSourcePath(packet.src).origin;
		const dst = packet.dst ?? '';
		if (dst === stationCallsign) {
			contacts.add(origin);
		} else if (origin === stationCallsign) {
			contacts.add(dst);
		}
	}
	return Array.from(contacts).sort((a, b) => a.localeCompare(b));
}
