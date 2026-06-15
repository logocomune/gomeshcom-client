import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChatRecord, MeshcomPacket } from '$lib/api/types';
import { chatState } from './chat.svelte';
import { connectionState } from './connection.svelte';
import { uiPrefs } from './ui-prefs.svelte';
import { toastStore } from './toasts.svelte';

vi.mock('$lib/ui/sound', () => ({ playDmAlert: vi.fn() }));

import { playDmAlert } from '$lib/ui/sound';

function packet(overrides: Partial<MeshcomPacket> = {}): MeshcomPacket {
	return { src: 'IU5PMP-1', msg: 'hello', ...overrides };
}

function record(overrides: Partial<ChatRecord> = {}): ChatRecord {
	return {
		received_at: '2026-06-01T10:00:00Z',
		src: 'IU5PMP-1',
		msg: 'hello',
		...overrides
	};
}

beforeEach(() => {
	chatState.chatHistory = {};
	chatState.conversations = [];
	chatState.chatTarget = { kind: 'channel', value: 'Broadcast' };
	chatState.chatStatus = {};
	chatState.dmScope = 'mycall';
	connectionState.stationCallsign = 'XX5YYY-1';
	uiPrefs.dmSoundEnabled = false;
	uiPrefs.dmToastEnabled = false;
	uiPrefs.mentionToastEnabled = false;
	toastStore.toasts = [];
	vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// appendLiveChatRecord
// ---------------------------------------------------------------------------

describe('appendLiveChatRecord — broadcast', () => {
	it('routes dst=* to P_broadcast', () => {
		chatState.appendLiveChatRecord(packet({ dst: '*' }), '2026-06-01T10:00:00Z');
		expect(chatState.chatHistory['P_broadcast']).toHaveLength(1);
	});

	it('routes empty dst to P_broadcast', () => {
		chatState.appendLiveChatRecord(packet({ dst: '' }), '2026-06-01T10:00:00Z');
		expect(chatState.chatHistory['P_broadcast']).toHaveLength(1);
	});

	it('creates broadcast conversation if missing', () => {
		chatState.appendLiveChatRecord(packet({ dst: '*' }), '2026-06-01T10:00:00Z');
		expect(chatState.conversations.some((c) => c.id === 'P_broadcast')).toBe(true);
	});

	it('updates existing broadcast conversation last_seen', () => {
		chatState.conversations = [
			{
				id: 'P_broadcast',
				kind: 'broadcast',
				label: 'Broadcast',
				last_seen: '2026-01-01T00:00:00Z',
				size: 0
			}
		];
		chatState.appendLiveChatRecord(packet({ dst: '*' }), '2026-06-01T10:00:00Z');
		expect(chatState.conversations[0].last_seen).toBe('2026-06-01T10:00:00Z');
	});
});

describe('appendLiveChatRecord — channel', () => {
	it('routes numeric dst to channel conversation', () => {
		chatState.appendLiveChatRecord(packet({ dst: '9' }), '2026-06-01T10:00:00Z');
		expect(chatState.chatHistory['P_9']).toHaveLength(1);
	});
});

describe('appendLiveChatRecord — DM', () => {
	it('stores DM addressed to myCall', () => {
		chatState.appendLiveChatRecord(
			packet({ src: 'IU5PMP-1', dst: 'XX5YYY-1', msg: 'hey' }),
			'2026-06-01T10:00:00Z'
		);
		expect(chatState.chatHistory['DM_XX5YYY-1_IU5PMP-1']).toHaveLength(1);
	});

	it('ignores DM not addressed to myCall', () => {
		chatState.appendLiveChatRecord(
			packet({ src: 'IU5PMP-1', dst: 'OTHER-1', msg: 'hey' }),
			'2026-06-01T10:00:00Z'
		);
		expect(Object.keys(chatState.chatHistory)).toHaveLength(0);
	});

	it('routes DM sent by myCall to the peer', () => {
		chatState.appendLiveChatRecord(
			packet({ src: 'XX5YYY-1', dst: 'IU5PMP-1', msg: 'sent' }),
			'2026-06-01T10:00:00Z'
		);
		expect(chatState.chatHistory['DM_XX5YYY-1_IU5PMP-1']).toHaveLength(1);
	});

	it('deduplicates records with same key', () => {
		const ts = '2026-06-01T10:00:00Z';
		chatState.appendLiveChatRecord(packet({ src: 'IU5PMP-1', dst: 'XX5YYY-1', msg_id: '42' }), ts);
		chatState.appendLiveChatRecord(packet({ src: 'IU5PMP-1', dst: 'XX5YYY-1', msg_id: '42' }), ts);
		expect(chatState.chatHistory['DM_XX5YYY-1_IU5PMP-1']).toHaveLength(1);
	});

	it('removes matching pending record on receive', () => {
		const convId = 'DM_XX5YYY-1_IU5PMP-1';
		chatState.chatHistory = {
			[convId]: [
				{
					received_at: '2026-06-01T09:00:00Z',
					src: 'XX5YYY-1',
					dst: 'IU5PMP-1',
					msg: 'hey',
					delivery_status: 'pending'
				}
			]
		};
		chatState.appendLiveChatRecord(
			packet({ src: 'XX5YYY-1', dst: 'IU5PMP-1', msg: 'hey' }),
			'2026-06-01T10:00:00Z'
		);
		const history = chatState.chatHistory[convId] ?? [];
		expect(history.every((r) => r.delivery_status !== 'pending')).toBe(true);
	});
});

describe('appendLiveChatRecord — ACK/reject', () => {
	it('does not increment unread count for ack message', () => {
		chatState.appendLiveChatRecord(
			packet({ src: 'IU5PMP-1', dst: '*', msg: 'ack42' }),
			'2026-06-01T10:00:00Z'
		);
		expect(chatState.chatStatus['P_broadcast']?.unreadCount ?? 0).toBe(0);
	});
});

// ---------------------------------------------------------------------------
// appendChatRecord
// ---------------------------------------------------------------------------

describe('appendChatRecord — routing', () => {
	it('routes inbound broadcast record to P_broadcast', () => {
		chatState.appendChatRecord(record({ dst: '*' }));
		expect(chatState.chatHistory['P_broadcast']).toHaveLength(1);
	});

	it('skips inbound DM not addressed to myCall', () => {
		// localCall is truthy → conversationIdForRecord returns null for non-matching DM
		chatState.appendChatRecord(record({ src: 'IU5PMP-1', dst: 'STRANGER-1' }));
		expect(Object.keys(chatState.chatHistory)).toHaveLength(0);
	});
});

describe('appendChatRecord — DM toast / sound', () => {
	beforeEach(() => {
		connectionState.stationCallsign = 'XX5YYY-1';
	});

	it('shows DM toast when dmToastEnabled and inbound', () => {
		uiPrefs.dmToastEnabled = true;
		chatState.appendChatRecord(record({ src: 'IU5PMP-1', dst: 'XX5YYY-1' }));
		expect(toastStore.toasts).toHaveLength(1);
		expect(toastStore.toasts[0].kind).toBe('dm');
		expect(toastStore.toasts[0].from).toBe('IU5PMP-1');
	});

	it('suppresses DM toast when dmToastEnabled=false', () => {
		uiPrefs.dmToastEnabled = false;
		chatState.appendChatRecord(record({ src: 'IU5PMP-1', dst: 'XX5YYY-1' }));
		expect(toastStore.toasts).toHaveLength(0);
	});

	it('plays DM sound when dmSoundEnabled and inbound', () => {
		uiPrefs.dmSoundEnabled = true;
		chatState.appendChatRecord(record({ src: 'IU5PMP-1', dst: 'XX5YYY-1' }));
		expect(playDmAlert).toHaveBeenCalledOnce();
	});

	it('suppresses DM sound when dmSoundEnabled=false', () => {
		uiPrefs.dmSoundEnabled = false;
		chatState.appendChatRecord(record({ src: 'IU5PMP-1', dst: 'XX5YYY-1' }));
		expect(playDmAlert).not.toHaveBeenCalled();
	});

	it('no toast for outbound DM', () => {
		uiPrefs.dmToastEnabled = true;
		chatState.appendChatRecord(record({ src: 'XX5YYY-1', dst: 'IU5PMP-1', direction: 'outbound' }));
		expect(toastStore.toasts).toHaveLength(0);
	});

	it('no toast for pending DM', () => {
		uiPrefs.dmToastEnabled = true;
		chatState.appendChatRecord(
			record({ src: 'IU5PMP-1', dst: 'XX5YYY-1', delivery_status: 'pending' })
		);
		expect(toastStore.toasts).toHaveLength(0);
	});
});

describe('appendChatRecord — mention toast', () => {
	beforeEach(() => {
		connectionState.stationCallsign = 'XX5YYY-1';
		uiPrefs.mentionToastEnabled = true;
	});

	it('shows mention toast when @basecall in channel message', () => {
		chatState.appendChatRecord(record({ dst: '*', msg: 'hey @XX5YYY what do you think?' }));
		expect(toastStore.toasts).toHaveLength(1);
		expect(toastStore.toasts[0].kind).toBe('mention');
		if (toastStore.toasts[0].kind === 'mention') {
			expect(toastStore.toasts[0].from).toBe('IU5PMP-1');
			expect(toastStore.toasts[0].channel).toBe('broadcast');
		}
	});

	it('no mention toast when message has no @callsign', () => {
		chatState.appendChatRecord(record({ dst: '*', msg: 'generic broadcast message' }));
		expect(toastStore.toasts).toHaveLength(0);
	});

	it('no mention toast when mentionToastEnabled=false', () => {
		uiPrefs.mentionToastEnabled = false;
		chatState.appendChatRecord(record({ dst: '*', msg: 'hey @XX5YYY are you there?' }));
		expect(toastStore.toasts).toHaveLength(0);
	});

	it('no mention toast for outbound channel message', () => {
		chatState.appendChatRecord(
			record({
				src: 'XX5YYY-1',
				dst: '*',
				msg: 'hey @XX5YYY are you there?',
				direction: 'outbound'
			})
		);
		expect(toastStore.toasts).toHaveLength(0);
	});
});

describe('appendChatRecord — mention toast false positives', () => {
	beforeEach(() => {
		connectionState.stationCallsign = 'XX5YYY-1';
		uiPrefs.mentionToastEnabled = true;
	});

	it('no toast when @basecall is prefix of longer callsign (@XX5YYYABC)', () => {
		chatState.appendChatRecord(record({ dst: '*', msg: 'hello @XX5YYYABC great job' }));
		expect(toastStore.toasts).toHaveLength(0);
	});

	it('shows toast when src contains relay path (src has comma)', () => {
		chatState.appendChatRecord(
			record({ src: 'IU5PMP-1,RELAY', dst: '*', msg: 'hey @XX5YYY are you there?' })
		);
		expect(toastStore.toasts).toHaveLength(1);
		expect(toastStore.toasts[0].kind).toBe('mention');
	});
});

describe('appendLiveChatRecord — ACK handling', () => {
	it('stores ACK in history but does not update chat status', () => {
		chatState.appendLiveChatRecord(
			packet({ src: 'IU5PMP-1', dst: '*', msg: 'ack42' }),
			'2026-06-01T10:00:00Z'
		);
		expect(chatState.chatHistory['P_broadcast']).toHaveLength(1);
		expect(chatState.chatHistory['P_broadcast'][0].msg).toBe('ack42');
		expect(chatState.chatStatus['P_broadcast']?.unreadCount ?? 0).toBe(0);
	});

	it('does not create new conversation for ACK on unknown conv', () => {
		chatState.appendLiveChatRecord(
			packet({ src: 'IU5PMP-1', dst: '*', msg: 'ack42' }),
			'2026-06-01T10:00:00Z'
		);
		expect(chatState.conversations.some((c) => c.id === 'P_broadcast')).toBe(false);
	});

	it('does not update last_seen on existing conversation for ACK', () => {
		chatState.conversations = [
			{
				id: 'P_broadcast',
				kind: 'broadcast',
				label: 'Broadcast',
				last_seen: '2026-01-01T00:00:00Z',
				size: 0
			}
		];
		chatState.appendLiveChatRecord(
			packet({ src: 'IU5PMP-1', dst: '*', msg: 'ack42' }),
			'2026-06-01T10:00:00Z'
		);
		expect(chatState.conversations[0].last_seen).toBe('2026-01-01T00:00:00Z');
	});
});

describe('appendChatRecord — ACK handling', () => {
	it('stores ACK in history but does not update chat status', () => {
		chatState.appendChatRecord(record({ dst: '*', msg: 'ack42' }));
		expect(chatState.chatHistory['P_broadcast']).toHaveLength(1);
		expect(chatState.chatStatus['P_broadcast']?.unreadCount ?? 0).toBe(0);
	});

	it('does not create new conversation for ACK on unknown conv', () => {
		chatState.appendChatRecord(record({ dst: '*', msg: 'ack42' }));
		expect(chatState.conversations.some((c) => c.id === 'P_broadcast')).toBe(false);
	});

	it('does not update last_seen on existing conversation for ACK', () => {
		chatState.conversations = [
			{
				id: 'P_broadcast',
				kind: 'broadcast',
				label: 'Broadcast',
				last_seen: '2026-01-01T00:00:00Z',
				size: 0
			}
		];
		chatState.appendChatRecord(record({ dst: '*', msg: 'ack42' }));
		expect(chatState.conversations[0].last_seen).toBe('2026-01-01T00:00:00Z');
	});
});

describe('appendChatRecord — failed delivery removes pending', () => {
	it('removes pending record matching failed record', () => {
		const convId = 'P_broadcast';
		chatState.chatHistory = {
			[convId]: [
				{
					received_at: '2026-06-01T09:00:00Z',
					src: 'XX5YYY-1',
					dst: '*',
					msg: 'hello',
					delivery_status: 'pending'
				}
			]
		};
		chatState.appendChatRecord(
			record({
				src: 'XX5YYY-1',
				dst: '*',
				msg: 'hello',
				delivery_status: 'failed',
				direction: 'outbound'
			})
		);
		const pending = (chatState.chatHistory[convId] ?? []).filter(
			(r) => r.delivery_status === 'pending'
		);
		expect(pending).toHaveLength(0);
	});
});
