import type { PacketReceivedPayload, StreamEvent } from '$lib/api/types';
import type { MapPosition } from './types';

export const DM_TRACE_TTL_MS = 45_000;

export type DmTraceSegment = {
	from: MapPosition;
	to: MapPosition;
	expiresAtMs: number;
};

function normalizeCallsign(value?: string): string {
	return (value ?? '').trim().toUpperCase();
}

function splitSourcePath(source?: string): { origin: string; relays: string[] } {
	const parts = (source ?? '')
		.split(',')
		.map((part) => part.trim())
		.filter(Boolean);
	return {
		origin: parts[0] ?? '',
		relays: parts.slice(1)
	};
}

function isDirectMessagePacket(packet: PacketReceivedPayload['packet']): boolean {
	if (!packet || packet.type !== 'msg') return false;
	const dst = (packet.dst ?? '').trim();
	if (dst === '' || dst === '*' || /^\d+$/.test(dst)) return false;
	const message = packet.msg ?? '';
	if (/^\{(?:CET|SET)\}/.test(message)) return false;
	return true;
}

export function buildRealtimeDmTraceSegments(
	positions: MapPosition[],
	events: StreamEvent[],
	nowMs: number
): DmTraceSegment[] {
	const positionBySource = new Map<string, MapPosition>();
	for (const position of positions) {
		positionBySource.set(normalizeCallsign(position.source), position);
	}

	const segmentByPath = new Map<string, DmTraceSegment>();

	for (const event of events) {
		if (event.type !== 'packet.received') continue;
		const payload = event.data as PacketReceivedPayload;
		if (payload.replay === true) continue;
		if (!isDirectMessagePacket(payload.packet)) continue;

		const eventMs = Date.parse(event.receivedAt);
		if (!Number.isFinite(eventMs)) continue;
		if (eventMs + DM_TRACE_TTL_MS <= nowMs) continue;

		const packet = payload.packet;
		if (!packet) continue;
		const source = splitSourcePath(packet.src);
		const dst = normalizeCallsign(packet.dst);
		if (!source.origin || !dst) continue;
		const hops = [source.origin, ...source.relays, dst].map((hop) => normalizeCallsign(hop));

		for (let index = 0; index < hops.length - 1; index++) {
			const from = positionBySource.get(hops[index]);
			const to = positionBySource.get(hops[index + 1]);
			if (!from || !to || from.source === to.source) continue;
			const key = `${normalizeCallsign(from.source)}>${normalizeCallsign(to.source)}`;
			const expiresAtMs = eventMs + DM_TRACE_TTL_MS;
			const current = segmentByPath.get(key);
			if (!current || current.expiresAtMs < expiresAtMs) {
				segmentByPath.set(key, { from, to, expiresAtMs });
			}
		}
	}

	return Array.from(segmentByPath.values());
}
