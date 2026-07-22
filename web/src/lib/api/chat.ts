import { API_BASE } from './events';
import { apiFetch } from './auth';
import type { ChatRecord, Conversation } from './types';

export type ChatTarget = { kind: 'channel' | 'contact'; value: string };
export type DmScope = 'mycall' | 'basecall';

const LAST_TARGET_KEY = 'meshcom:chat:last';
const CHAT_RECORD_DEDUPE_WINDOW_MS = 5 * 60 * 1000;

export function baseCallFrom(callsign: string): string {
	const upper = callsign.toUpperCase();
	const i = upper.lastIndexOf('-');
	if (i >= 0 && /^\d+$/.test(upper.slice(i + 1))) return upper.slice(0, i);
	return upper;
}

export function dmPeerFromId(id: string): string {
	if (!id.startsWith('DM_')) return '';
	const rest = id.slice(3);
	const i = rest.lastIndexOf('_');
	return i >= 0 ? rest.slice(i + 1) : rest;
}

export function loadLastChatTarget(conversations: Conversation[]): ChatTarget {
	try {
		const id = localStorage.getItem(LAST_TARGET_KEY);
		if (!id || id === 'P_broadcast') return broadcastTarget();
		const conversation = conversations.find((c) => c.id === id);
		if (!conversation) return broadcastTarget();
		if (conversation.kind === 'dm') {
			return { kind: 'contact', value: conversation.label || dmPeerFromId(id) };
		}
		return { kind: 'channel', value: conversation.label || id.replace(/^P_/, '') };
	} catch {
		return broadcastTarget();
	}
}

export function saveLastChatTarget(target: ChatTarget, convId?: string): void {
	try {
		localStorage.setItem(LAST_TARGET_KEY, convId ?? conversationIdFor(target));
	} catch {
		// quota or SSR — ignore
	}
}

function broadcastTarget(): ChatTarget {
	return { kind: 'channel', value: 'Broadcast' };
}

export class SendError extends Error {
	constructor(
		message: string,
		public status: number
	) {
		super(message);
		this.name = 'SendError';
	}
}

export async function sendMessage(dst: string, msg: string): Promise<void> {
	const res = await apiFetch(`${API_BASE}/messages`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ dst, msg })
	});
	if (res.status === 429) throw new SendError('duplicate', 429);
	if (!res.ok) throw new SendError(await res.text(), res.status);
}

export function destinationFor(target: ChatTarget): string {
	if (target.kind === 'channel') return target.value === 'Broadcast' ? '*' : target.value;
	return target.value;
}

export async function fetchConversations(scope: DmScope = 'mycall'): Promise<Conversation[]> {
	const response = await apiFetch(`${API_BASE}/chat/list?scope=${scope}`);
	if (!response.ok) return [];
	return ((await response.json()) as Conversation[] | null) ?? [];
}

export async function fetchHistory(
	id: string,
	hours?: number,
	scope: DmScope = 'mycall'
): Promise<ChatRecord[]> {
	const url = new URL(`${API_BASE}/chat/${encodeURIComponent(id)}`, location.origin);
	if (isDirectMessageConversation(id)) {
		url.searchParams.set('scope', scope);
	} else if (hours != null) {
		url.searchParams.set('hours', String(hours));
	}
	const response = await apiFetch(url.toString());
	if (!response.ok) return [];
	const records = ((await response.json()) as ChatRecord[] | null) ?? [];
	const uniqueRecords: ChatRecord[] = [];
	for (const record of records) {
		if (uniqueRecords.some((existing) => isDuplicateChatRecord(existing, record))) continue;
		uniqueRecords.push(record);
	}
	return uniqueRecords.map((r) => ({ ...r, source: 'event-history' as const }));
}

function isDirectMessageConversation(id: string): boolean {
	return id.startsWith('DM_');
}

export function conversationIdFor(target: ChatTarget): string {
	if (target.kind === 'channel') {
		if (target.value === 'Broadcast') return 'P_broadcast';
		return 'P_' + target.value;
	}
	return 'DM_' + sanitizeConversationPart(target.value);
}

export async function markConversationRead(id: string, scope: DmScope = 'mycall'): Promise<void> {
	const base = `${API_BASE}/chat/${encodeURIComponent(id)}/read`;
	const url = isDirectMessageConversation(id) ? `${base}?scope=${scope}` : base;
	await apiFetch(url, { method: 'POST' });
}

export function conversationIdForRecord(
	rec: ChatRecord,
	myCall: string,
	scope: DmScope = 'mycall'
): string | null {
	const dst = rec.dst ?? '';
	const origin = (rec.src ?? '').split(',', 1)[0].toUpperCase();
	if (dst === '' || dst === '*') return 'P_broadcast';
	if (/^\d+$/.test(dst)) return 'P_' + dst;

	const prefix = scope === 'basecall' ? baseCallFrom(myCall) : myCall;
	if (rec.direction === 'outbound') {
		return 'DM_' + sanitizeConversationPart(prefix) + '_' + sanitizeConversationPart(dst);
	}

	const localCall = myCall.toUpperCase();
	const dstUpper = dst.toUpperCase();
	const localBase = baseCallFrom(localCall);
	const isBasecall = scope === 'basecall';
	const matchesMy = isBasecall
		? baseCallFrom(dstUpper) === localBase || baseCallFrom(origin) === localBase
		: dstUpper === localCall || origin === localCall;
	if (localCall && !matchesMy) return null;
	const interlocutor = (isBasecall ? baseCallFrom(dstUpper) === localBase : dstUpper === localCall)
		? origin
		: dstUpper;
	return 'DM_' + sanitizeConversationPart(prefix) + '_' + sanitizeConversationPart(interlocutor);
}

export function chatRecordKey(rec: ChatRecord): string {
	return [rec.src ?? '', rec.dst ?? '', rec.msg_id ?? '', rec.msg, rec.received_at].join('|');
}

export function isDuplicateChatRecord(existing: ChatRecord, candidate: ChatRecord): boolean {
	if (existing.msg_id && candidate.msg_id) {
		return (
			existing.msg_id === candidate.msg_id &&
			senderFrom(existing.src) === senderFrom(candidate.src) &&
			receivedWithinDedupeWindow(existing.received_at, candidate.received_at)
		);
	}
	return chatRecordKey(existing) === chatRecordKey(candidate);
}

function senderFrom(source: string | undefined): string {
	return (source ?? '').split(',', 1)[0].trim().toUpperCase();
}

function receivedWithinDedupeWindow(left: string, right: string): boolean {
	const leftTime = Date.parse(left);
	const rightTime = Date.parse(right);
	if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) return left === right;
	return Math.abs(leftTime - rightTime) <= CHAT_RECORD_DEDUPE_WINDOW_MS;
}

export function sanitizeConversationPart(value: string): string {
	return value.toUpperCase().replace(/[^A-Z0-9_-]/g, '_');
}

export async function deleteConversation(id: string): Promise<void> {
	const res = await apiFetch(`${API_BASE}/chat/${encodeURIComponent(id)}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 404) throw new Error(`delete failed: ${res.status}`);
}
