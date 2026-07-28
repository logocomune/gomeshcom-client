export type ChatMentionPart =
	| { kind: 'text'; value: string }
	| { kind: 'mention'; value: string; callsign: string };

export type ChatContentPart = ChatMentionPart | { kind: 'link'; value: string; href: string };

const CALLSIGN_MENTION_PATTERN = /@([A-Z]{1,3}[0-9][A-Z0-9]{1,6}(?:-[0-9]{1,2})?)/gi;
const CHAT_TOKEN_PATTERN = /@[A-Z]{1,3}[0-9][A-Z0-9]{1,6}(?:-[0-9]{1,2})?|https?:\/\/[^\s<>"']+/gi;

export function splitChatMentions(message: string): ChatMentionPart[] {
	const parts: ChatMentionPart[] = [];
	let textStart = 0;

	for (const match of message.matchAll(CALLSIGN_MENTION_PATTERN)) {
		const matchStart = match.index ?? 0;
		const matchEnd = matchStart + match[0].length;
		if (!hasMentionBoundary(message, matchStart, matchEnd)) continue;
		if (matchStart > textStart)
			parts.push({ kind: 'text', value: message.slice(textStart, matchStart) });
		parts.push({ kind: 'mention', value: match[0], callsign: match[1].toUpperCase() });
		textStart = matchEnd;
	}

	if (textStart < message.length || parts.length === 0) {
		parts.push({ kind: 'text', value: message.slice(textStart) });
	}
	return parts;
}

export function splitChatContent(message: string): ChatContentPart[] {
	const parts: ChatContentPart[] = [];
	let textStart = 0;

	for (const match of message.matchAll(CHAT_TOKEN_PATTERN)) {
		const matchStart = match.index ?? 0;
		const token = match[0];
		const part = token.startsWith('@') ? mentionPart(message, matchStart, token) : linkPart(token);
		if (!part) continue;
		if (matchStart > textStart)
			parts.push({ kind: 'text', value: message.slice(textStart, matchStart) });
		parts.push(part);
		textStart = matchStart + part.value.length;
	}

	if (textStart < message.length || parts.length === 0) {
		parts.push({ kind: 'text', value: message.slice(textStart) });
	}
	return parts;
}

export function positionForCallsign(
	positions: MapPosition[],
	callsign: string
): MapPosition | undefined {
	const normalized = callsign.toUpperCase();
	return positions.find(
		(position) =>
			position.id.toUpperCase() === normalized || position.source.toUpperCase() === normalized
	);
}

export function hasNumericSsid(callsign: string): boolean {
	return /-[0-9]{1,2}$/.test(callsign);
}

function hasMentionBoundary(message: string, start: number, end: number): boolean {
	const previous = message[start - 1] ?? '';
	const next = message[end] ?? '';
	return !/[A-Z0-9_]/i.test(previous) && !/[A-Z0-9-]/i.test(next);
}

function mentionPart(message: string, start: number, value: string): ChatMentionPart | null {
	const callsign = value.slice(1);
	if (!hasMentionBoundary(message, start, start + value.length)) return null;
	return { kind: 'mention', value, callsign: callsign.toUpperCase() };
}

function linkPart(value: string): ChatContentPart | null {
	const href = value.replace(/[.,!?;:]+$/, '');
	try {
		const url = new URL(href);
		if ((url.protocol !== 'http:' && url.protocol !== 'https:') || url.hostname === '') return null;
		return { kind: 'link', value: href, href };
	} catch {
		return null;
	}
}
import type { MapPosition } from '$lib/map/types';
