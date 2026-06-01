import {
	eventSummary,
	messageKind,
	packetBadge,
	packetFromEvent,
	splitSourcePath,
	stripMessagePrefix
} from '$lib/api/events';
import type { StreamEvent } from '$lib/api/types';
import {
	mdiAlertCircleOutline,
	mdiBroadcast,
	mdiCheckCircleOutline,
	mdiClockOutline,
	mdiCogOutline,
	mdiEmailOutline,
	mdiMapMarkerRadiusOutline
} from '@mdi/js';

export type MessageRoute = {
	origin: string;
	relays: string[];
	destination: string;
};

export function filterStreamEvents(events: StreamEvent[], filter: string): StreamEvent[] {
	const term = filter.trim().toLowerCase();
	if (term === '') return events;

	return events.filter((event) => streamEventMatchesTerm(event, term));
}

function streamEventMatchesTerm(event: StreamEvent, term: string): boolean {
	const packet = packetFromEvent(event);
	if (eventSummary(event).toLowerCase().includes(term)) return true;
	if (packet?.src?.toLowerCase().includes(term)) return true;
	if (packet?.dst?.toLowerCase().includes(term)) return true;
	if (packet?.msg?.toLowerCase().includes(term)) return true;
	if (packetBadge(event).toLowerCase().includes(term)) return true;
	return false;
}

export function packetTone(event: StreamEvent): string {
	const badge = packetBadge(event);
	if (badge === 'error') return 'border-coral/70 bg-coral/20 text-coral';
	if (badge === 'msg')   return 'border-mint/70 bg-mint/20 text-mint';
	if (badge === 'pos')   return 'border-azure/70 bg-azure/20 text-azure';
	if (badge === 'tele')  return 'border-lavender/70 bg-lavender/20 text-lavender';
	return 'border-ink-dim/50 bg-ink-dim/20 text-ink-muted';
}

export function packetField(event: StreamEvent, key: string): string {
	const packet = packetFromEvent(event);
	const value = packet?.[key];
	return value == null || value === '' ? '—' : String(value);
}

export function isMessageEvent(event: StreamEvent): boolean {
	return packetFromEvent(event)?.type === 'msg';
}

export function messageRoute(event: StreamEvent): MessageRoute {
	const packet = packetFromEvent(event);
	const source = splitSourcePath(packet?.src);
	return {
		origin: source.origin,
		relays: source.relays,
		destination: packet?.dst === '*' ? 'Broadcast' : (packet?.dst ?? 'unknown')
	};
}

export function messageText(event: StreamEvent): string {
	const packet = packetFromEvent(event);
	return stripMessagePrefix(packet?.msg ?? '') || packetField(event, 'msg');
}

export function mdiForEvent(event: StreamEvent): string {
	const packet = packetFromEvent(event);
	if (packet?.type === 'msg') {
		const kind = messageKind(packet.msg).kind;
		if (kind === 'time') return mdiClockOutline;
		if (kind === 'ack') return mdiCheckCircleOutline;
		if (kind === 'reject') return mdiAlertCircleOutline;
		if (kind === 'config') return mdiCogOutline;
		return mdiEmailOutline;
	}
	if (packet?.type === 'pos') return mdiMapMarkerRadiusOutline;
	if (packet?.type === 'tele') return mdiBroadcast;
	return mdiAlertCircleOutline;
}

export function iconTooltip(event: StreamEvent): string {
	const packet = packetFromEvent(event);
	if (!packet) return 'Parse error';
	if (packet.type === 'msg') {
		const kind = messageKind(packet.msg).kind;
		if (kind === 'ack') return 'ACK — message acknowledged';
		if (kind === 'reject') return 'REJ — message rejected';
		if (kind === 'time') return 'Network time sync';
		if (kind === 'config') return 'Config update';
		return 'Text message';
	}
	if (packet.type === 'pos') return 'Position report';
	if (packet.type === 'tele') return 'Telemetry';
	return `Raw packet · ${packet.src_type ?? 'unknown'}`;
}
