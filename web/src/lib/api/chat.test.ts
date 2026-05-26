import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest';
import {
	deleteConversation,
	conversationIdForRecord,
	fetchHistory,
	loadLastChatTarget,
	saveLastChatTarget,
	markConversationRead
} from './chat';

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

describe('markConversationRead', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('calls POST with the correct URL', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));
		await markConversationRead('P_broadcast');
		expect(fetchSpy).toHaveBeenCalledWith('/api/chat/P_broadcast/read', {
			method: 'POST',
			credentials: 'same-origin'
		});
	});

	it('encodes callsign with special chars', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));
		await markConversationRead('DM_QQ1ABC-1');
		expect(fetchSpy).toHaveBeenCalledWith('/api/chat/DM_QQ1ABC-1/read', {
			method: 'POST',
			credentials: 'same-origin'
		});
	});
});

describe('loadLastChatTarget / saveLastChatTarget', () => {
	let store: Record<string, string> = {};

	beforeEach(() => {
		store = {};
		vi.stubGlobal('localStorage', {
			getItem(key: string) { return store[key] ?? null; },
			setItem(key: string, val: string) { store[key] = val; },
			key() { return null; },
			removeItem(key: string) { delete store[key]; },
			clear() { store = {}; }
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
			getItem() { throw new Error('SecurityError'); },
			setItem() { throw new Error('SecurityError'); }
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
