import type {
	ChannelShowConfig,
	Conversation,
	ChatRecord,
	ChatStatusEntry,
	ChatStatusSnapshot
} from '$lib/api/types';
import { DEFAULT_CHANNEL_SHOW, isConvHidden } from '$lib/api/channelShow';
import type { ChatTarget, DmScope } from '$lib/api/chat';
import {
	conversationIdFor,
	conversationIdForRecord,
	chatRecordKey,
	isDuplicateChatRecord,
	loadLastChatTarget,
	saveLastChatTarget,
	markConversationRead,
	baseCallFrom,
	dmPeerFromId,
	sanitizeConversationPart
} from '$lib/api/chat';
import { messageKind, splitSourcePath } from '$lib/api/events';
import { partitionChannels } from '$lib/api/groups';
import { chatRecordMatchesFilter, stripNodeSequence } from '$lib/ui/chat-records';
import { normalizeCallsign } from '$lib/ui/callsign';
import { loadChatChannelsCollapsed, saveChatChannelsCollapsed } from '$lib/ui/chat-layout';
import { connectionState } from '$lib/stores/connection.svelte';
import { uiPrefs } from '$lib/stores/ui-prefs.svelte';
import { playDmAlert } from '$lib/ui/sound';
import { toastStore } from '$lib/stores/toasts.svelte';

const DEFAULT_CHAT_WIDTH = 50;
const STORAGE_CHAT_WIDTH = 'meshcom:chatWidthPct';

const DEFAULT_CHAT_LIST_WIDTH_PX = 256;
const STORAGE_CHAT_LIST_WIDTH = 'meshcom:chatListWidthPx';
const CHAT_LIST_MIN_PX = 160;
const CHAT_LIST_MAX_PX = 520;

function aggregateBasecallStatus(
	snapshot: Record<string, ChatStatusEntry>,
	myCall: string
): Record<string, ChatStatusEntry> {
	if (!myCall) return snapshot;
	const myBase = baseCallFrom(myCall);
	const result: Record<string, ChatStatusEntry> = {};

	for (const [key, entry] of Object.entries(snapshot)) {
		if (!key.startsWith('DM_')) {
			result[key] = entry;
			continue;
		}
		const rest = key.slice(3);
		const sepIdx = rest.lastIndexOf('_');
		if (sepIdx < 0) {
			result[key] = entry;
			continue;
		}
		const prefixPart = rest.slice(0, sepIdx);
		const peer = rest.slice(sepIdx + 1);
		if (baseCallFrom(prefixPart) !== myBase) {
			result[key] = entry;
			continue;
		}
		const basecallKey = 'DM_' + myBase + '_' + peer;
		const existing = result[basecallKey];
		if (!existing) {
			result[basecallKey] = { ...entry };
		} else {
			const entryTime = new Date(entry.lastMsgReceived).getTime();
			const existTime = new Date(existing.lastMsgReceived).getTime();
			result[basecallKey] = {
				lastMsgReceived: entryTime > existTime ? entry.lastMsgReceived : existing.lastMsgReceived,
				lastRead: entry.lastRead > existing.lastRead ? entry.lastRead : existing.lastRead,
				unreadCount: Math.max(entry.unreadCount, existing.unreadCount),
				lastMsg: entryTime > existTime ? entry.lastMsg : existing.lastMsg
			};
		}
	}
	return result;
}

class ChatStore {
	chatHistory = $state<Record<string, ChatRecord[]>>({});
	conversations = $state<Conversation[]>([]);
	chatTarget = $state<ChatTarget>({ kind: 'channel', value: 'Broadcast' });
	chatStatus = $state<Record<string, ChatStatusEntry>>({});
	dmScope = $state<DmScope>('mycall');
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

	currentConvId = $derived(
		this.chatTarget.kind === 'contact' && connectionState.stationCallsign
			? 'DM_' +
					sanitizeConversationPart(
						this.dmScope === 'basecall'
							? baseCallFrom(connectionState.stationCallsign)
							: connectionState.stationCallsign
					) +
					'_' +
					sanitizeConversationPart(this.chatTarget.value)
			: conversationIdFor(this.chatTarget)
	);

	effectiveChatStatus = $derived(
		this.dmScope === 'mycall'
			? this.chatStatus
			: aggregateBasecallStatus(this.chatStatus, connectionState.stationCallsign)
	);

	isBroadcastTarget = $derived(
		this.chatTarget.kind === 'channel' && this.chatTarget.value === 'Broadcast'
	);
	unreadIds = $derived(
		new Set(
			Object.entries(this.effectiveChatStatus)
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
				.filter((c) => (this.effectiveChatStatus[c.id]?.unreadCount ?? 0) > 0)
				.map((c) => c.id)
		)
	);
	channelLabels = $derived(this.conversations.filter((c) => c.kind !== 'dm').map((c) => c.label));
	resolvedChannels = $derived(partitionChannels(this.channelLabels));
	contacts = $derived(this.conversations.filter((c) => c.kind === 'dm').map((c) => c.label));
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

	setConversations(conversations: Conversation[], preserveTarget = false) {
		this.conversations = conversations;
		this.fetchedConvIds = new Set();
		if (!preserveTarget) {
			this.chatTarget = loadLastChatTarget(conversations);
		}
		this.conversationsLoaded = true;
	}

	setDmScope(scope: DmScope) {
		if (this.dmScope === scope) return;
		this.dmScope = scope;
		// Clear DM history cache; re-fetched with new scope on next access.
		const nextHistory = { ...this.chatHistory };
		for (const id of Object.keys(nextHistory)) {
			if (id.startsWith('DM_')) delete nextHistory[id];
		}
		this.chatHistory = nextHistory;
		this.fetchedConvIds = new Set();
	}

	async markCurrentRead(convId: string) {
		if ((this.effectiveChatStatus[convId]?.unreadCount ?? 0) === 0) return;

		if (this.dmScope === 'mycall' || !convId.startsWith('DM_')) {
			const prev = this.chatStatus[convId] ?? { lastMsgReceived: '', lastRead: '', unreadCount: 0 };
			this.chatStatus = {
				...this.chatStatus,
				[convId]: { ...prev, unreadCount: 0, lastRead: new Date().toISOString() }
			};
		} else {
			// Basecall scope: optimistically zero all matching SSID status keys.
			const peer = dmPeerFromId(convId);
			const myBase = baseCallFrom(connectionState.stationCallsign);
			const next = { ...this.chatStatus };
			for (const k of Object.keys(next)) {
				if (!k.startsWith('DM_')) continue;
				if (dmPeerFromId(k) !== peer) continue;
				const rest = k.slice(3);
				const sepIdx = rest.lastIndexOf('_');
				if (sepIdx < 0) continue;
				if (baseCallFrom(rest.slice(0, sepIdx)) === myBase) {
					next[k] = { ...next[k], unreadCount: 0, lastRead: new Date().toISOString() };
				}
			}
			this.chatStatus = next;
		}
		try {
			await markConversationRead(convId, this.dmScope);
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
		saveLastChatTarget(this.chatTarget, this.currentConvId);
	}

	toggleChannelsSidebar() {
		this.channelsCollapsed = !this.channelsCollapsed;
		this.saveChannelsCollapsed();
	}

	appendLiveChatRecord(packet: import('$lib/api/types').MeshcomPacket, receivedAt: string) {
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
			const myBase = baseCallFrom(myCall);
			const isBasecall = this.dmScope === 'basecall';
			const matchesMy = isBasecall
				? baseCallFrom(dstUpper) === myBase || baseCallFrom(origin) === myBase
				: dstUpper === myCall || origin === myCall;
			if (myCall && !matchesMy) return;
			const interlocutor = (isBasecall ? baseCallFrom(dstUpper) === myBase : dstUpper === myCall)
				? origin
				: dstUpper;
			const prefix = this.dmScope === 'basecall' ? baseCallFrom(myCall) : myCall;
			convId =
				'DM_' + sanitizeConversationPart(prefix) + '_' + sanitizeConversationPart(interlocutor);
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

		// ACK/reject packets must not update last_seen, preview text, or unread count;
		// they are only stored in history for delivery tracking.
		const recKind = messageKind(rec.msg).kind;
		const isAckOrReject = recKind === 'ack' || recKind === 'reject';

		const idx = this.conversations.findIndex((c) => c.id === convId);
		if (idx === -1) {
			if (!isAckOrReject) {
				let kind: Conversation['kind'] = 'broadcast';
				let label = 'Broadcast';
				if (dst !== '' && dst !== '*') {
					if (/^\d+$/.test(dst)) {
						kind = 'channel';
						label = dst;
					} else {
						kind = 'dm';
						label = dmPeerFromId(convId) || convId.replace(/^DM_/, '');
					}
				}
				this.conversations = [
					{ id: convId, kind, label, last_seen: receivedAt, size: 0 },
					...this.conversations
				];
			}
		} else if (!isAckOrReject) {
			this.conversations = this.conversations.map((c) =>
				c.id === convId ? { ...c, last_seen: receivedAt } : c
			);
		}

		if (!isAckOrReject) {
			this.updateChatStatusOnReceive(convId, receivedAt, rec.msg);
		}
	}

	appendChatRecord(rec: ChatRecord) {
		const convId = conversationIdForRecord(rec, connectionState.stationCallsign, this.dmScope);
		if (!convId) return;
		if (rec.delivery_status === 'failed') {
			this.removeMatchingPendingRecord(convId, rec);
		}
		this.appendChatRecordToConversation(convId, { ...rec, source: 'event-live' });

		const isAckOrRejectRecord = ['ack', 'reject'].includes(messageKind(rec.msg).kind);

		const idx = this.conversations.findIndex((c) => c.id === convId);
		if (idx === -1) {
			if (!isAckOrRejectRecord) {
				this.conversations = [
					{
						id: convId,
						kind:
							convId === 'P_broadcast' ? 'broadcast' : convId.startsWith('P_') ? 'channel' : 'dm',
						label:
							convId === 'P_broadcast'
								? 'Broadcast'
								: convId.startsWith('DM_')
									? dmPeerFromId(convId) || convId.replace(/^DM_/, '')
									: convId.replace(/^P_/, ''),
						last_seen: rec.received_at,
						size: 0
					},
					...this.conversations
				];
			}
		} else if (!isAckOrRejectRecord) {
			const copy = this.conversations.slice();
			copy[idx] = { ...copy[idx], last_seen: rec.received_at };
			this.conversations = copy.sort((a, b) => b.last_seen.localeCompare(a.last_seen));
		}
		if (rec.direction !== 'outbound' && rec.delivery_status !== 'pending' && !isAckOrRejectRecord) {
			this.updateChatStatusOnReceive(convId, rec.received_at, rec.msg);
			if (convId.startsWith('DM_')) {
				if (uiPrefs.dmSoundEnabled) {
					playDmAlert();
				}
				if (uiPrefs.dmToastEnabled) {
					const sender = dmPeerFromId(convId) || convId.replace(/^DM_/, '');
					toastStore.addDm(sender);
				}
			} else if (convId.startsWith('P_') && uiPrefs.mentionToastEnabled) {
				const myBase = baseCallFrom(connectionState.stationCallsign);
				const escaped = myBase?.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
				if (escaped && new RegExp(`@${escaped}(?![A-Z0-9])`, 'i').test(rec.msg)) {
					const channel = convId.replace(/^P_/, '');
					const sender = splitSourcePath(rec.src).origin;
					toastStore.addMention(sender, channel);
				}
			}
		}
	}

	removeChatRecord(rec: ChatRecord) {
		const convId = conversationIdForRecord(rec, connectionState.stationCallsign, this.dmScope);
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
		if (existing.some((r) => isDuplicateChatRecord(r, rec))) return;
		this.chatHistory = {
			...this.chatHistory,
			[convId]: [...existing, rec].sort((a, b) => a.received_at.localeCompare(b.received_at))
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
		const merged: ChatRecord[] = [];
		for (const record of [...records, ...live]) {
			if (merged.some((existing) => isDuplicateChatRecord(existing, record))) continue;
			merged.push(record);
		}
		merged.sort((a, b) => a.received_at.localeCompare(b.received_at));
		this.chatHistory = { ...this.chatHistory, [id]: merged };
		this.fetchedConvIds = new Set([...this.fetchedConvIds, id]);
	}

	resetAfterLogin() {
		this.chatHistory = {};
		this.fetchedConvIds = new Set();
	}

	private updateChatStatusOnReceive(convId: string, receivedAt: string, msg: string) {
		const stationCallsign = connectionState.stationCallsign;

		// chatStatus always uses full-SSID keys for DMs; derive status key from convId.
		let statusKey = convId;
		if (stationCallsign && convId.startsWith('DM_')) {
			const peer = dmPeerFromId(convId);
			statusKey = 'DM_' + sanitizeConversationPart(stationCallsign) + '_' + peer;
		}

		const entry = this.chatStatus[statusKey] ?? {
			lastMsgReceived: '',
			lastRead: '',
			unreadCount: 0
		};
		if (convId === this.currentConvId) {
			this.chatStatus = {
				...this.chatStatus,
				[statusKey]: {
					...entry,
					lastMsgReceived: receivedAt,
					lastRead: receivedAt,
					unreadCount: 0,
					lastMsg: msg
				}
			};
		} else {
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
				[statusKey]: {
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
		if (id.startsWith('DM_')) {
			const peer = dmPeerFromId(id);
			const myBase = baseCallFrom(connectionState.stationCallsign);
			for (const k of Object.keys(nextStatus)) {
				if (!k.startsWith('DM_')) continue;
				if (dmPeerFromId(k) !== peer) continue;
				const rest = k.slice(3);
				const sepIdx = rest.lastIndexOf('_');
				if (sepIdx < 0) continue;
				if (baseCallFrom(rest.slice(0, sepIdx)) === myBase) delete nextStatus[k];
			}
		} else {
			delete nextStatus[id];
		}
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
