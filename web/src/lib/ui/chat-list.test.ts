import { describe, expect, it } from 'vitest';
import type { ChatRecord, Conversation } from '$lib/api/types';
import { conversationPreview, sortByRecency, formatRelativeTime } from './chat-list';

function makeRecord(msg: string, received_at = '2026-01-01T00:00:00Z'): ChatRecord {
	return { msg, received_at };
}

function makeConv(id: string, last_seen: string): Conversation {
	return { id, kind: 'channel', label: id, last_seen, size: 0 };
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
