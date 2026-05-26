import type { ChannelShowConfig, Conversation, ChatRecord, ChatStatusEntry, ChatStatusSnapshot } from '$lib/api/types';
import { DEFAULT_CHANNEL_SHOW, isConvHidden } from '$lib/api/channelShow';
import type { ChatTarget } from '$lib/api/chat';
import {
	conversationIdFor,
	conversationIdForRecord,
	chatRecordKey,
	loadLastChatTarget,
	saveLastChatTarget,
	markConversationRead
} from '$lib/api/chat';
import { messageKind } from '$lib/api/events';
import { partitionChannels } from '$lib/api/groups';
import { chatRecordMatchesFilter, stripNodeSequence } from '$lib/ui/chat-records';
import { normalizeCallsign } from '$lib/ui/callsign';
import { loadChatChannelsCollapsed, saveChatChannelsCollapsed } from '$lib/ui/chat-layout';
import { connectionState } from '$lib/stores/connection.svelte';

const DEFAULT_CHAT_WIDTH = 50;
const STORAGE_CHAT_WIDTH = 'meshcom:chatWidthPct';

const DEFAULT_CHAT_LIST_WIDTH_PX = 256;
const STORAGE_CHAT_LIST_WIDTH = 'meshcom:chatListWidthPx';
const CHAT_LIST_MIN_PX = 160;
const CHAT_LIST_MAX_PX = 520;

class ChatStore {
	chatHistory = $state<Record<string, ChatRecord[]>>({});
	conversations = $state<Conversation[]>([]);
	chatTarget = $state<ChatTarget>({ kind: 'channel', value: 'Broadcast' });
	chatStatus = $state<Record<string, ChatStatusEntry>>({});
	chatFilter = $state('');
	fetchedConvIds = $state(new Set<string>());
	historyHours = $state(168);
	draftMessage = $state('');
	sending = $state(false);
	sendError = $state<string | null>(null);
	rawChatRecord = $state<ChatRecord | null>(null);
	newDmOpen = $state(false);
	newDmCallsign = $state('');
	newDmError = $state('');
	newChannelOpen = $state(false);
	newChannelValue = $state('');
	newChannelError = $state('');
	deleteConfirmOpen = $state(false);
	deleting = $state(false);
	deleteError = $state<string | null>(null);
	channelShow = $state<ChannelShowConfig>(DEFAULT_CHANNEL_SHOW);
	channelShowOpen = $state(false);
	channelShowDraftMode = $state<ChannelShowConfig['mode']>('all');
	channelShowDraftChannels = $state<string[]>([]);
	channelShowDraftInput = $state('');
	channelShowError = $state('');
	channelShowSaving = $state(false);
	channelsCollapsed = $state(false);
	chatWidthPct = $state(DEFAULT_CHAT_WIDTH);
	chatListWidthPx = $state(DEFAULT_CHAT_LIST_WIDTH_PX);
	conversationsLoaded = $state(false);

	currentConvId = $derived(conversationIdFor(this.chatTarget));
	isBroadcastTarget = $derived(
		this.chatTarget.kind === 'channel' && this.chatTarget.value === 'Broadcast'
	);
	unreadIds = $derived(
		new Set(
			Object.entries(this.chatStatus)
				.filter(([, e]) => e.unreadCount > 0)
				.map(([id]) => id)
		)
	);
	visibleConversations = $derived(
		this.conversations.filter((c) => !isConvHidden(c.id, this.channelShow))
	);
	visibleUnreadIds = $derived(
		new Set(
			this.visibleConversations
				.filter((c) => (this.chatStatus[c.id]?.unreadCount ?? 0) > 0)
				.map((c) => c.id)
		)
	);
	channelLabels = $derived(
		this.conversations.filter((c) => c.kind !== 'dm').map((c) => c.label)
	);
	resolvedChannels = $derived(partitionChannels(this.channelLabels));
	contacts = $derived(
		this.conversations.filter((c) => c.kind === 'dm').map((c) => c.label)
	);
	displayChatRecords = $derived(
		(this.chatHistory[this.currentConvId] ?? []).filter((rec) => {
			const kind = messageKind(rec.msg).kind;
			if (kind === 'ack' || kind === 'reject') return false;
			return chatRecordMatchesFilter(rec, this.chatFilter);
		})
	);

	loadLayout() {
		const w = parseFloat(localStorage.getItem(STORAGE_CHAT_WIDTH) ?? '');
		if (!isNaN(w) && w >= 20 && w <= 80) this.chatWidthPct = w;
		this.channelsCollapsed = loadChatChannelsCollapsed(localStorage);
		const lw = parseInt(localStorage.getItem(STORAGE_CHAT_LIST_WIDTH) ?? '', 10);
		if (!isNaN(lw) && lw >= CHAT_LIST_MIN_PX && lw <= CHAT_LIST_MAX_PX) this.chatListWidthPx = lw;
	}

	saveChatWidth() {
		localStorage.setItem(STORAGE_CHAT_WIDTH, String(this.chatWidthPct));
	}

	saveChatListWidth() {
		localStorage.setItem(STORAGE_CHAT_LIST_WIDTH, String(this.chatListWidthPx));
	}

	setChatListWidth(px: number) {
		this.chatListWidthPx = Math.max(CHAT_LIST_MIN_PX, Math.min(CHAT_LIST_MAX_PX, px));
	}

	saveChannelsCollapsed() {
		saveChatChannelsCollapsed(localStorage, this.channelsCollapsed);
	}

	setChatStatus(snapshot: ChatStatusSnapshot) {
		this.chatStatus = snapshot;
	}

	setChannelShow(cfg: ChannelShowConfig) {
		this.channelShow = cfg;
	}

	openChannelShowModal() {
		this.channelShowDraftMode = this.channelShow.mode;
		this.channelShowDraftChannels = [...this.channelShow.channels];
		this.channelShowDraftInput = '';
		this.channelShowError = '';
		this.channelShowOpen = true;
	}

	setConversations(conversations: Conversation[]) {
		this.conversations = conversations;
		this.fetchedConvIds = new Set();
		this.chatTarget = loadLastChatTarget(conversations);
		this.conversationsLoaded = true;
	}

	async markCurrentRead(convId: string) {
		if ((this.chatStatus[convId]?.unreadCount ?? 0) === 0) return;
		const prev = this.chatStatus[convId] ?? { lastMsgReceived: '', lastRead: '', unreadCount: 0 };
		this.chatStatus = {
			...this.chatStatus,
			[convId]: { ...prev, unreadCount: 0, lastRead: new Date().toISOString() }
		};
		try {
			await markConversationRead(convId);
		} catch {
			// fire-and-forget; next snapshot reconciles
		}
	}

	selectChannel(channel: string) {
		this.chatTarget = { kind: 'channel', value: channel };
		saveLastChatTarget(this.chatTarget);
	}

	selectContact(contact: string) {
		this.chatTarget = { kind: 'contact', value: normalizeCallsign(contact) };
		saveLastChatTarget(this.chatTarget);
	}

	toggleChannelsSidebar() {
		this.channelsCollapsed = !this.channelsCollapsed;
		this.saveChannelsCollapsed();
	}

	appendLiveChatRecord(
		packet: import('$lib/api/types').MeshcomPacket,
		receivedAt: string
	) {
		const stationCallsign = connectionState.stationCallsign;
		const dst = packet.dst ?? '';
		const origin = (packet.src ?? '').split(',', 1)[0].toUpperCase();

		let convId: string;
		if (dst === '' || dst === '*') {
			convId = 'P_broadcast';
		} else if (/^\d+$/.test(dst)) {
			convId = 'P_' + dst;
		} else {
			const myCall = stationCallsign ? stationCallsign.toUpperCase() : '';
			const dstUpper = dst.toUpperCase();
			if (myCall && dstUpper !== myCall && origin !== myCall) return;
			const interlocutor = myCall && dstUpper === myCall ? origin : dstUpper;
			convId = 'DM_' + interlocutor.replace(/[^A-Z0-9_-]/g, '_');
		}

		const rec: ChatRecord = {
			received_at: receivedAt,
			src: packet.src,
			src_type: packet.src_type,
			dst: dst || undefined,
			msg_id: packet.msg_id,
			msg: packet.msg ?? '',
			rssi: packet.rssi != null ? (packet.rssi as number) : undefined,
			snr: packet.snr != null ? (packet.snr as number) : undefined,
			source: 'event-live'
		};

		this.removeMatchingPendingRecord(convId, rec);
		this.appendChatRecordToConversation(convId, rec);

		const idx = this.conversations.findIndex((c) => c.id === convId);
		if (idx === -1) {
			let kind: Conversation['kind'] = 'broadcast';
			let label = 'Broadcast';
			if (dst !== '' && dst !== '*') {
				if (/^\d+$/.test(dst)) {
					kind = 'channel';
					label = dst;
				} else {
					kind = 'dm';
					label = convId.replace(/^DM_/, '');
				}
			}
			this.conversations = [
				{ id: convId, kind, label, last_seen: receivedAt, size: 0 },
				...this.conversations
			];
		} else {
			this.conversations = this.conversations.map((c) =>
				c.id === convId ? { ...c, last_seen: receivedAt } : c
			);
		}

		this.updateChatStatusOnReceive(convId, receivedAt, rec.msg);
	}

	appendChatRecord(rec: ChatRecord) {
		const convId = conversationIdForRecord(rec, connectionState.stationCallsign);
		if (!convId) return;
		if (rec.delivery_status === 'failed') {
			this.removeMatchingPendingRecord(convId, rec);
		}
		this.appendChatRecordToConversation(convId, { ...rec, source: 'event-live' });

		const idx = this.conversations.findIndex((c) => c.id === convId);
		if (idx === -1) {
			this.conversations = [
				{
					id: convId,
					kind:
						convId === 'P_broadcast'
							? 'broadcast'
							: convId.startsWith('P_')
								? 'channel'
								: 'dm',
					label: convId === 'P_broadcast' ? 'Broadcast' : convId.replace(/^(P_|DM_)/, ''),
					last_seen: rec.received_at,
					size: 0
				},
				...this.conversations
			];
		} else {
			const copy = this.conversations.slice();
			copy[idx] = { ...copy[idx], last_seen: rec.received_at };
			this.conversations = copy.sort((a, b) => b.last_seen.localeCompare(a.last_seen));
		}

		// Only inbound messages affect unread counts — skip outbound/pending/failed.
		if (rec.direction !== 'outbound' && rec.delivery_status !== 'pending') {
			this.updateChatStatusOnReceive(convId, rec.received_at, rec.msg);
		}
	}

	removeChatRecord(rec: ChatRecord) {
		const convId = conversationIdForRecord(rec, connectionState.stationCallsign);
		if (!convId) return;
		const existing = this.chatHistory[convId] ?? [];
		const key = chatRecordKey(rec);
		this.chatHistory = {
			...this.chatHistory,
			[convId]: existing.filter((item) => chatRecordKey(item) !== key)
		};
	}

	appendChatRecordToConversation(convId: string, rec: ChatRecord) {
		const existing = this.chatHistory[convId] ?? [];
		const key = chatRecordKey(rec);
		if (existing.some((r) => chatRecordKey(r) === key)) return;
		this.chatHistory = {
			...this.chatHistory,
			[convId]: [...existing, rec].sort((a, b) =>
				a.received_at.localeCompare(b.received_at)
			)
		};
	}

	removeMatchingPendingRecord(convId: string, rec: ChatRecord) {
		const existing = this.chatHistory[convId] ?? [];
		const recDst = rec.dst ?? '';
		const recMsg = stripNodeSequence(rec.msg);
		const next = existing.filter((item) => {
			if (item.delivery_status !== 'pending') return true;
			if ((item.dst ?? '') !== recDst) return true;
			return item.msg !== recMsg;
		});
		if (next.length === existing.length) return;
		this.chatHistory = { ...this.chatHistory, [convId]: next };
	}

	mergeHistory(id: string, records: ChatRecord[]) {
		const live = this.chatHistory[id] ?? [];
		const seen = new Set<string>();
		const merged = [...records, ...live]
			.filter((r) => {
				const key = chatRecordKey(r);
				if (seen.has(key)) return false;
				seen.add(key);
				return true;
			})
			.sort((a, b) => a.received_at.localeCompare(b.received_at));
		this.chatHistory = { ...this.chatHistory, [id]: merged };
		this.fetchedConvIds = new Set([...this.fetchedConvIds, id]);
	}

	resetAfterLogin() {
		this.chatHistory = {};
		this.fetchedConvIds = new Set();
	}

	private updateChatStatusOnReceive(convId: string, receivedAt: string, msg: string) {
		const stationCallsign = connectionState.stationCallsign;
		const entry = this.chatStatus[convId] ?? { lastMsgReceived: '', lastRead: '', unreadCount: 0 };
		if (convId === this.currentConvId) {
			this.chatStatus = {
				...this.chatStatus,
				[convId]: { ...entry, lastMsgReceived: receivedAt, lastRead: receivedAt, unreadCount: 0, lastMsg: msg }
			};
		} else {
			// Skip self-echo: self-echoes already filtered for DMs (appendLiveChatRecord line 144 guard),
			// but broadcast/channel self-echoes still reach here — skip them.
			const history = this.chatHistory[convId] ?? [];
			const last = history.at(-1);
			if (
				last &&
				stationCallsign &&
				(last.src ?? '').split(',', 1)[0].toUpperCase() === stationCallsign.toUpperCase() &&
				last.received_at === receivedAt
			) {
				return;
			}
			this.chatStatus = {
				...this.chatStatus,
				[convId]: {
					...entry,
					lastMsgReceived: receivedAt,
					unreadCount: entry.unreadCount + 1,
					lastMsg: msg
				}
			};
		}
	}

	deleteLocalConversation(id: string) {
		const nextHistory = { ...this.chatHistory };
		delete nextHistory[id];
		this.chatHistory = nextHistory;

		const nextStatus = { ...this.chatStatus };
		delete nextStatus[id];
		this.chatStatus = nextStatus;

		if (id === 'P_broadcast') {
			this.chatHistory = { ...this.chatHistory, [id]: [] };
		} else {
			this.conversations = this.conversations.filter((c) => c.id !== id);
			this.chatTarget = { kind: 'channel', value: 'Broadcast' };
			saveLastChatTarget(this.chatTarget);
		}
	}
}

export const chatState = new ChatStore();
