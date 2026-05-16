import { API_BASE } from './events';
import { apiFetch } from './auth';
import type { ChatRecord, Conversation } from './types';

const READ_KEY_PREFIX = 'meshcom:chat:read:';

export function loadReadTimestamps(): Record<string, string> {
	try {
		const result: Record<string, string> = {};
		for (let i = 0; i < localStorage.length; i++) {
			const key = localStorage.key(i);
			if (key?.startsWith(READ_KEY_PREFIX)) {
				const val = localStorage.getItem(key);
				if (val) result[key.slice(READ_KEY_PREFIX.length)] = val;
			}
		}
		return result;
	} catch {
		return {};
	}
}

export function saveReadTimestamp(convId: string, isoTs: string): void {
	try {
		localStorage.setItem(READ_KEY_PREFIX + convId, isoTs);
	} catch {
		// quota or SSR — ignore
	}
}

export function isUnread(conv: Conversation, readTs: string | undefined): boolean {
	if (!conv.last_seen || !readTs) return false;
	return conv.last_seen > readTs;
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

export function destinationFor(target: { kind: 'channel' | 'contact'; value: string }): string {
	if (target.kind === 'channel') return target.value === 'Broadcast' ? '*' : target.value;
	return target.value;
}

export async function fetchConversations(): Promise<Conversation[]> {
	const response = await apiFetch(`${API_BASE}/chat/list`);
	if (!response.ok) return [];
	return ((await response.json()) as Conversation[] | null) ?? [];
}

export async function fetchHistory(id: string, hours?: number): Promise<ChatRecord[]> {
	const url = new URL(`${API_BASE}/chat/${encodeURIComponent(id)}`, location.origin);
	if (hours != null) url.searchParams.set('hours', String(hours));
	const response = await apiFetch(url.toString());
	if (!response.ok) return [];
	const records = ((await response.json()) as ChatRecord[] | null) ?? [];
	const seen = new Set<string>();
	return records
		.filter((r) => {
			const key = chatRecordKey(r);
			if (seen.has(key)) return false;
			seen.add(key);
			return true;
		})
		.map((r) => ({ ...r, source: 'event-history' as const }));
}

export function conversationIdFor(target: { kind: 'channel' | 'contact'; value: string }): string {
	if (target.kind === 'channel') {
		if (target.value === 'Broadcast') return 'P_broadcast';
		return 'P_' + target.value;
	}
	return 'DM_' + target.value.toUpperCase().replace(/[^A-Z0-9_-]/g, '_');
}

export function chatRecordKey(rec: ChatRecord): string {
	return rec.msg_id || `${rec.src ?? ''}|${rec.dst ?? ''}|${rec.msg}|${rec.received_at}`;
}

export async function deleteConversation(id: string): Promise<void> {
	const res = await apiFetch(`${API_BASE}/chat/${encodeURIComponent(id)}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 404) throw new Error(`delete failed: ${res.status}`);
}

export function clearReadTimestamp(convId: string): void {
	try {
		localStorage.removeItem(READ_KEY_PREFIX + convId);
	} catch {
		// quota or SSR — ignore
	}
}
