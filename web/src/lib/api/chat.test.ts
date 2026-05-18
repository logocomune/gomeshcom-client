import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest';
import {
	loadReadTimestamps,
	saveReadTimestamp,
	isUnread,
	clearReadTimestamp,
	deleteConversation,
	conversationIdForRecord,
	fetchHistory,
	loadLastChatTarget,
	saveLastChatTarget
} from './chat';
import type { Conversation } from './types';

function conv(id: string, last_seen: string): Conversation {
	return { id, kind: 'channel', label: id, last_seen, size: 0 };
}

describe('isUnread', () => {
	it('returns false when readTs is undefined (first-load seed pending)', () => {
		expect(isUnread(conv('P_broadcast', '2026-05-16T10:00:00Z'), undefined)).toBe(false);
	});

	it('returns true when conv.last_seen is newer than readTs', () => {
		expect(isUnread(conv('P_broadcast', '2026-05-16T10:05:00Z'), '2026-05-16T10:00:00Z')).toBe(
			true
		);
	});

	it('returns false when conv.last_seen equals readTs', () => {
		expect(isUnread(conv('P_broadcast', '2026-05-16T10:00:00Z'), '2026-05-16T10:00:00Z')).toBe(
			false
		);
	});

	it('returns false when conv.last_seen is older than readTs', () => {
		expect(isUnread(conv('P_broadcast', '2026-05-16T09:00:00Z'), '2026-05-16T10:00:00Z')).toBe(
			false
		);
	});

	it('returns false when conv.last_seen is empty', () => {
		expect(isUnread(conv('P_broadcast', ''), '2026-05-16T10:00:00Z')).toBe(false);
	});
});

describe('conversationIdForRecord', () => {
	it('keeps outbound failed DM tied to destination when MyCall changes', () => {
		expect(
			conversationIdForRecord(
				{
					received_at: '2026-05-18T09:00:00Z',
					src: 'OLDMYCALL-1',
					dst: 'QQ1ABC-1',
					msg: 'hello',
					direction: 'outbound',
					delivery_status: 'failed'
				},
				'NEWMYCALL-1'
			)
		).toBe('DM_QQ1ABC-1');
	});

	it.each([
		{ dst: '*', want: 'P_broadcast' },
		{ dst: '123', want: 'P_123' }
	])('returns $want for outbound failed destination $dst', ({ dst, want }) => {
		expect(
			conversationIdForRecord(
				{
					received_at: '2026-05-18T09:00:00Z',
					src: 'QQ0QQ-1',
					dst,
					msg: 'hello',
					direction: 'outbound',
					delivery_status: 'failed'
				},
				'NEWMYCALL-1'
			)
		).toBe(want);
	});
});

describe('loadReadTimestamps / saveReadTimestamp', () => {
	let store: Record<string, string> = {};

	beforeEach(() => {
		store = {};
		vi.stubGlobal('localStorage', {
			length: 0,
			_data: store,
			getItem(key: string) {
				return store[key] ?? null;
			},
			setItem(key: string, val: string) {
				store[key] = val;
				(this as unknown as { length: number }).length = Object.keys(store).length;
			},
			key(index: number) {
				return Object.keys(store)[index] ?? null;
			},
			removeItem(key: string) {
				delete store[key];
				(this as unknown as { length: number }).length = Object.keys(store).length;
			},
			clear() {
				store = {};
				(this as unknown as { length: number }).length = 0;
			}
		});
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('returns empty map when localStorage is empty', () => {
		expect(loadReadTimestamps()).toEqual({});
	});

	it('loads only meshcom:chat:read: prefixed keys', () => {
		saveReadTimestamp('P_broadcast', '2026-05-16T10:00:00Z');
		saveReadTimestamp('DM_QQ1ABC', '2026-05-16T10:01:00Z');
		store['other:key'] = 'ignored';
		store['meshcom:other'] = 'ignored';
		(localStorage as unknown as { length: number }).length = Object.keys(store).length;

		const result = loadReadTimestamps();
		expect(result).toEqual({
			P_broadcast: '2026-05-16T10:00:00Z',
			DM_QQ1ABC: '2026-05-16T10:01:00Z'
		});
	});

	it('saveReadTimestamp writes correct key', () => {
		saveReadTimestamp('P_1', '2026-05-16T09:00:00Z');
		expect(store['meshcom:chat:read:P_1']).toBe('2026-05-16T09:00:00Z');
	});

	it('loadReadTimestamps returns empty on localStorage throw', () => {
		vi.stubGlobal('localStorage', {
			length: 0,
			getItem() {
				throw new Error('quota');
			},
			setItem() {
				throw new Error('quota');
			},
			key() {
				throw new Error('quota');
			},
			removeItem() {},
			clear() {}
		});
		expect(loadReadTimestamps()).toEqual({});
	});

	it('saveReadTimestamp does not throw on quota error', () => {
		vi.stubGlobal('localStorage', {
			length: 0,
			getItem() {
				return null;
			},
			setItem() {
				throw new Error('QuotaExceededError');
			},
			key() {
				return null;
			},
			removeItem() {},
			clear() {}
		});
		expect(() => saveReadTimestamp('P_broadcast', '2026-05-16T10:00:00Z')).not.toThrow();
	});

	it('clearReadTimestamp removes the correct key', () => {
		saveReadTimestamp('P_broadcast', '2026-05-16T10:00:00Z');
		expect(store['meshcom:chat:read:P_broadcast']).toBeDefined();
		clearReadTimestamp('P_broadcast');
		expect(store['meshcom:chat:read:P_broadcast']).toBeUndefined();
	});

	it('clearReadTimestamp does not throw on localStorage error', () => {
		vi.stubGlobal('localStorage', {
			length: 0,
			getItem() {
				return null;
			},
			setItem() {},
			key() {
				return null;
			},
			removeItem() {
				throw new Error('SecurityError');
			},
			clear() {}
		});
		expect(() => clearReadTimestamp('P_broadcast')).not.toThrow();
	});
});

describe('loadLastChatTarget / saveLastChatTarget', () => {
	let store: Record<string, string> = {};

	beforeEach(() => {
		store = {};
		vi.stubGlobal('localStorage', {
			length: 0,
			getItem(key: string) {
				return store[key] ?? null;
			},
			setItem(key: string, val: string) {
				store[key] = val;
			},
			key() {
				return null;
			},
			removeItem(key: string) {
				delete store[key];
			},
			clear() {
				store = {};
			}
		});
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('saves selected chat as a conversation id', () => {
		saveLastChatTarget({ kind: 'contact', value: 'QQ1ABC-1' });

		expect(store['meshcom:chat:last']).toBe('DM_QQ1ABC-1');
	});

	it('loads saved DM when it still exists', () => {
		store['meshcom:chat:last'] = 'DM_QQ1ABC-1';

		expect(
			loadLastChatTarget([
				{ id: 'P_broadcast', kind: 'broadcast', label: 'Broadcast', last_seen: '', size: 0 },
				{ id: 'DM_QQ1ABC-1', kind: 'dm', label: 'QQ1ABC-1', last_seen: '', size: 0 }
			])
		).toEqual({ kind: 'contact', value: 'QQ1ABC-1' });
	});

	it('loads saved channel when it still exists', () => {
		store['meshcom:chat:last'] = 'P_222';

		expect(
			loadLastChatTarget([
				{ id: 'P_broadcast', kind: 'broadcast', label: 'Broadcast', last_seen: '', size: 0 },
				{ id: 'P_222', kind: 'channel', label: '222', last_seen: '', size: 0 }
			])
		).toEqual({ kind: 'channel', value: '222' });
	});

	it('falls back to Broadcast when saved chat no longer exists', () => {
		store['meshcom:chat:last'] = 'DM_MISSING-1';

		expect(
			loadLastChatTarget([
				{ id: 'P_broadcast', kind: 'broadcast', label: 'Broadcast', last_seen: '', size: 0 }
			])
		).toEqual({ kind: 'channel', value: 'Broadcast' });
	});

	it('falls back to Broadcast when localStorage throws', () => {
		vi.stubGlobal('localStorage', {
			getItem() {
				throw new Error('SecurityError');
			},
			setItem() {
				throw new Error('SecurityError');
			}
		});

		expect(loadLastChatTarget([])).toEqual({ kind: 'channel', value: 'Broadcast' });
		expect(() => saveLastChatTarget({ kind: 'channel', value: '222' })).not.toThrow();
	});
});

describe('fetchHistory', () => {
	afterEach(() => {
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	it('sends hours for channel history', async () => {
		vi.stubGlobal('location', { origin: 'http://localhost:3000' });
		const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(Response.json([]));

		await fetchHistory('P_broadcast', 168);

		expect(fetchSpy).toHaveBeenCalledWith('http://localhost:3000/api/chat/P_broadcast?hours=168', {
			credentials: 'same-origin'
		});
	});

	it('omits hours for DM history so backend default window applies', async () => {
		vi.stubGlobal('location', { origin: 'http://localhost:3000' });
		const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(Response.json([]));

		await fetchHistory('DM_QQ1ABC-1', 168);

		expect(fetchSpy).toHaveBeenCalledWith('http://localhost:3000/api/chat/DM_QQ1ABC-1', {
			credentials: 'same-origin'
		});
	});
});

describe('deleteConversation', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('calls DELETE with the correct URL', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));
		await deleteConversation('P_broadcast');
		expect(fetchSpy).toHaveBeenCalledWith('/api/chat/P_broadcast', {
			method: 'DELETE',
			credentials: 'same-origin'
		});
	});

	it('does not throw on 404 response', async () => {
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('not found', { status: 404 }));
		await expect(deleteConversation('P_broadcast')).resolves.toBeUndefined();
	});

	it('throws on 500 response', async () => {
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('error', { status: 500 }));
		await expect(deleteConversation('P_broadcast')).rejects.toThrow('delete failed: 500');
	});
});
