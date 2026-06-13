import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest';
import {
	deleteConversation,
	conversationIdForRecord,
	fetchHistory,
	loadLastChatTarget,
	saveLastChatTarget,
	markConversationRead,
	baseCallFrom,
	dmPeerFromId
} from './chat';

describe('baseCallFrom', () => {
	it.each([
		['IU5PMP-1', 'IU5PMP'],
		['IU5PMP-10', 'IU5PMP'],
		['IU5PMP', 'IU5PMP'],
		['IK5FCK-10', 'IK5FCK'],
		['XX5YYY-2', 'XX5YYY'],
		['iu5pmp-1', 'IU5PMP']
	])('baseCallFrom(%s) = %s', (input, expected) => {
		expect(baseCallFrom(input)).toBe(expected);
	});
});

describe('dmPeerFromId', () => {
	it.each([
		['DM_IK5FCK-10', 'IK5FCK-10'],
		['DM_IU5PMP-1_IK5FCK-10', 'IK5FCK-10'],
		['DM_IU5PMP_IK5FCK-10', 'IK5FCK-10'],
		['P_broadcast', ''],
		['P_123', '']
	])('dmPeerFromId(%s) = %s', (id, expected) => {
		expect(dmPeerFromId(id)).toBe(expected);
	});
});

describe('conversationIdForRecord', () => {
	it('maps outbound DM to mycall-prefixed conv id (mycall scope)', () => {
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
				'NEWMYCALL-1',
				'mycall'
			)
		).toBe('DM_NEWMYCALL-1_QQ1ABC-1');
	});

	it('maps outbound DM to basecall-prefixed conv id (basecall scope)', () => {
		expect(
			conversationIdForRecord(
				{
					received_at: '2026-05-18T09:00:00Z',
					src: 'IU5PMP-1',
					dst: 'IK5FCK-10',
					msg: 'hello',
					direction: 'outbound',
					delivery_status: 'failed'
				},
				'IU5PMP-1',
				'basecall'
			)
		).toBe('DM_IU5PMP_IK5FCK-10');
	});

	it.each([
		{ dst: '*', want: 'P_broadcast' },
		{ dst: '123', want: 'P_123' }
	])('returns $want for outbound destination $dst', ({ dst, want }) => {
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

	it('uses default mycall scope when scope omitted', () => {
		expect(
			conversationIdForRecord(
				{
					received_at: '2026-05-18T09:00:00Z',
					src: 'IU5PMP-1',
					dst: 'IK5FCK-10',
					msg: 'hello',
					direction: 'outbound',
					delivery_status: 'failed'
				},
				'IU5PMP-1'
			)
		).toBe('DM_IU5PMP-1_IK5FCK-10');
	});
});

describe('markConversationRead', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('calls POST without scope param for public conversations', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));
		await markConversationRead('P_broadcast');
		expect(fetchSpy).toHaveBeenCalledWith('/api/chat/P_broadcast/read', {
			method: 'POST',
			credentials: 'same-origin'
		});
	});

	it('appends scope=mycall for DM conversations (default scope)', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));
		await markConversationRead('DM_IU5PMP-1_IK5FCK-10');
		expect(fetchSpy).toHaveBeenCalledWith('/api/chat/DM_IU5PMP-1_IK5FCK-10/read?scope=mycall', {
			method: 'POST',
			credentials: 'same-origin'
		});
	});

	it('appends scope=basecall when requested', async () => {
		const fetchSpy = vi
			.spyOn(globalThis, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));
		await markConversationRead('DM_IU5PMP_IK5FCK-10', 'basecall');
		expect(fetchSpy).toHaveBeenCalledWith('/api/chat/DM_IU5PMP_IK5FCK-10/read?scope=basecall', {
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

	it('saves provided convId when given', () => {
		saveLastChatTarget({ kind: 'contact', value: 'QQ1ABC-1' }, 'DM_IU5PMP-1_QQ1ABC-1');
		expect(store['meshcom:chat:last']).toBe('DM_IU5PMP-1_QQ1ABC-1');
	});

	it('loads saved DM when it still exists', () => {
		store['meshcom:chat:last'] = 'DM_IU5PMP-1_QQ1ABC-1';
		expect(
			loadLastChatTarget([
				{ id: 'P_broadcast', kind: 'broadcast', label: 'Broadcast', last_seen: '', size: 0 },
				{ id: 'DM_IU5PMP-1_QQ1ABC-1', kind: 'dm', label: 'QQ1ABC-1', last_seen: '', size: 0 }
			])
		).toEqual({ kind: 'contact', value: 'QQ1ABC-1' });
	});

	it('extracts peer from DM id when label missing', () => {
		store['meshcom:chat:last'] = 'DM_IU5PMP-1_QQ1ABC-1';
		expect(
			loadLastChatTarget([
				{ id: 'DM_IU5PMP-1_QQ1ABC-1', kind: 'dm', label: '', last_seen: '', size: 0 }
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

	it('sends scope=mycall for DM history (default)', async () => {
		vi.stubGlobal('location', { origin: 'http://localhost:3000' });
		const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(Response.json([]));

		await fetchHistory('DM_IU5PMP-1_QQ1ABC-1', 168);

		expect(fetchSpy).toHaveBeenCalledWith(
			'http://localhost:3000/api/chat/DM_IU5PMP-1_QQ1ABC-1?scope=mycall',
			{ credentials: 'same-origin' }
		);
	});

	it('sends scope=basecall for DM history when requested', async () => {
		vi.stubGlobal('location', { origin: 'http://localhost:3000' });
		const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(Response.json([]));

		await fetchHistory('DM_IU5PMP_QQ1ABC-1', 168, 'basecall');

		expect(fetchSpy).toHaveBeenCalledWith(
			'http://localhost:3000/api/chat/DM_IU5PMP_QQ1ABC-1?scope=basecall',
			{ credentials: 'same-origin' }
		);
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
