import { describe, expect, it } from 'vitest';
import type { ChatRecord, ChatStatusEntry, Conversation } from '$lib/api/types';
import { conversationPreview, latestPreview, sortByRecency, formatRelativeTime } from './chat-list';

function makeRecord(msg: string, received_at = '2026-01-01T00:00:00Z'): ChatRecord {
	return { msg, received_at };
}

function makeConv(id: string, last_seen: string): Conversation {
	return { id, kind: 'channel', label: id, last_seen, size: 0 };
}

function makeStatus(lastMsgReceived: string, lastMsg: string): ChatStatusEntry {
	return { lastMsgReceived, lastRead: '', unreadCount: 0, lastMsg };
}

describe('conversationPreview', () => {
	it('returns empty string for empty records', () => {
		expect(conversationPreview([])).toBe('');
	});

	it('returns last message text', () => {
		const records = [makeRecord('hello'), makeRecord('world')];
		expect(conversationPreview(records)).toBe('world');
	});

	it('truncates at 40 chars', () => {
		const long = 'a'.repeat(50);
		const records = [makeRecord(long)];
		const result = conversationPreview(records);
		expect(result.length).toBe(41); // 40 chars + ellipsis
		expect(result.endsWith('…')).toBe(true);
	});

	it('does not truncate exactly 40 chars', () => {
		const text = 'a'.repeat(40);
		const records = [makeRecord(text)];
		expect(conversationPreview(records)).toBe(text);
	});
});

describe('latestPreview', () => {
	it('returns empty string when no history and no status', () => {
		expect(latestPreview([], undefined)).toBe('');
	});

	it('returns status lastMsg when no history loaded', () => {
		const status = makeStatus('2026-01-01T10:00:00Z', 'received msg');
		expect(latestPreview([], status)).toBe('received msg');
	});

	it('shows sent message when newer than last received', () => {
		const status = makeStatus('2026-01-01T09:00:00Z', 'received msg');
		const records = [
			makeRecord('received msg', '2026-01-01T09:00:00Z'),
			makeRecord('sent msg', '2026-01-01T10:00:00Z')
		];
		expect(latestPreview(records, status)).toBe('sent msg');
	});

	it('shows received message from status when it is the latest', () => {
		const status = makeStatus('2026-01-01T11:00:00Z', 'latest received');
		const records = [makeRecord('older sent', '2026-01-01T10:00:00Z')];
		expect(latestPreview(records, status)).toBe('latest received');
	});

	it('uses history fallback when status has no lastMsg', () => {
		const status: ChatStatusEntry = { lastMsgReceived: '', lastRead: '', unreadCount: 0 };
		const records = [makeRecord('only in history', '2026-01-01T10:00:00Z')];
		expect(latestPreview(records, status)).toBe('only in history');
	});

	it('skips ACK record at end — shows prior text message', () => {
		const records = [
			makeRecord('hello', '2026-01-01T10:00:00Z'),
			makeRecord('PEER:ack42', '2026-01-01T10:01:00Z')
		];
		expect(latestPreview(records, undefined)).toBe('hello');
	});

	it('skips reject record at end — shows prior text message', () => {
		const records = [
			makeRecord('hi there', '2026-01-01T10:00:00Z'),
			makeRecord('PEER:rej99', '2026-01-01T10:01:00Z')
		];
		expect(latestPreview(records, undefined)).toBe('hi there');
	});

	it('returns empty string when all records are ACK', () => {
		const records = [
			makeRecord('PEER:ack1', '2026-01-01T10:00:00Z'),
			makeRecord('PEER:ack2', '2026-01-01T10:01:00Z')
		];
		expect(latestPreview(records, undefined)).toBe('');
	});

	it('skips ACK in status.lastMsg — falls back to history', () => {
		const status = makeStatus('2026-01-01T11:00:00Z', 'PEER:ack42');
		const records = [makeRecord('real message', '2026-01-01T10:00:00Z')];
		expect(latestPreview(records, status)).toBe('real message');
	});

	it('returns empty when status.lastMsg is ACK and no history', () => {
		const status = makeStatus('2026-01-01T11:00:00Z', 'PEER:ack42');
		expect(latestPreview([], status)).toBe('');
	});

	it('prefers status.lastMsg when newer than last non-ACK record', () => {
		const status = makeStatus('2026-01-01T11:00:00Z', 'latest received');
		const records = [
			makeRecord('older text', '2026-01-01T09:00:00Z'),
			makeRecord('PEER:ack5', '2026-01-01T10:30:00Z')
		];
		// last non-ACK is 09:00, status is 11:00 → status wins
		expect(latestPreview(records, status)).toBe('latest received');
	});
});

describe('sortByRecency', () => {
	it('sorts conversations newest first', () => {
		const convs = [
			makeConv('a', '2026-01-01T00:00:00Z'),
			makeConv('c', '2026-01-03T00:00:00Z'),
			makeConv('b', '2026-01-02T00:00:00Z')
		];
		const sorted = sortByRecency(convs);
		expect(sorted.map((c) => c.id)).toEqual(['c', 'b', 'a']);
	});

	it('does not mutate input', () => {
		const convs = [makeConv('a', '2026-01-01T00:00:00Z'), makeConv('b', '2026-01-02T00:00:00Z')];
		sortByRecency(convs);
		expect(convs[0].id).toBe('a');
	});
});

describe('formatRelativeTime', () => {
	it('returns empty string for invalid timestamp', () => {
		expect(formatRelativeTime('not-a-date')).toBe('');
	});

	it('returns "now" for very recent', () => {
		const ts = new Date(Date.now() - 30_000).toISOString();
		expect(formatRelativeTime(ts)).toBe('now');
	});

	it('returns minutes for <1h ago', () => {
		const ts = new Date(Date.now() - 10 * 60_000).toISOString();
		expect(formatRelativeTime(ts)).toBe('10m');
	});

	it('returns hours for <24h ago', () => {
		const ts = new Date(Date.now() - 3 * 3_600_000).toISOString();
		expect(formatRelativeTime(ts)).toBe('3h');
	});
});
