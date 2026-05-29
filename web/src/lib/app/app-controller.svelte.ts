import { login, onUnauthorized, UnauthorizedError } from '$lib/api/auth';
import {
	fetchConversations,
	fetchHistory,
	deleteConversation,
	sendMessage,
	destinationFor,
	SendError
} from '$lib/api/chat';
import { updateChannelShow } from '$lib/api/channelShow';
import { buildAckIndex, buildAckIndexFromChatRecords, mergeAckIndexes } from '$lib/api/acks';
import type { ConnectionState } from '$lib/api/types';
import { chatState } from '$lib/stores/chat.svelte';
import { connectionState } from '$lib/stores/connection.svelte';
import { eventsState } from '$lib/stores/events.svelte';
import { createSseStore } from '$lib/stores/sse.svelte';
import type { AppContext } from '$lib/app/app-context';
import { isValidCallsign, normalizeCallsign } from '$lib/ui/callsign';

export class AppController {
	authModalOpen = $state(false);
	authUsername = $state('');
	authPassword = $state('');
	authError = $state<string | null>(null);
	authSubmitting = $state(false);
	isDesktop = $state(true);

	sendEchoTimer: ReturnType<typeof setTimeout> | null = null;
	stopUnauthorizedWatch: (() => void) | null = null;

	ackIndex = $derived(
		mergeAckIndexes(
			buildAckIndex(eventsState.events),
			buildAckIndexFromChatRecords(chatState.chatHistory[chatState.currentConvId] ?? [])
		)
	);

	sse = createSseStore({ clearSendEcho: () => this.clearSendEcho() });
	context: AppContext;

	statusText: Record<ConnectionState, string> = {
		connecting: 'Connecting',
		connected: 'Connected',
		disconnected: 'Disconnected',
		unauthenticated: 'Locked'
	};

	statusClass: Record<ConnectionState, string> = {
		connecting: 'bg-amber-400 shadow-[0_0_8px_rgba(251,191,36,0.6)]',
		connected: 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.6)]',
		disconnected: 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.6)]',
		unauthenticated: 'bg-amber-300 shadow-[0_0_8px_rgba(252,211,77,0.6)]'
	};

	constructor() {
		const controller = this;
		this.context = {
			sse: this.sse,
			get ackIndex() {
				return controller.ackIndex;
			},
			get isDesktop() {
				return controller.isDesktop;
			},
			handleSend: () => controller.handleSend(),
			openDeleteConfirm: () => controller.openDeleteConfirm(),
			openNewDm: () => controller.openNewDm(),
			openNewChannel: () => controller.openNewChannel()
		};
	}

	async mount() {
		chatState.loadLayout();
		eventsState.loadLayout();
		this.stopUnauthorizedWatch = onUnauthorized(() => this.handleUnauthorized());
		this.sse.connect();
		try {
			await this.reloadProtectedData();
		} catch {
			// modal state handled by unauthorized listener
		}
	}

	destroy() {
		this.sse.disconnect();
		this.stopUnauthorizedWatch?.();
		this.clearSendEcho();
	}

	loadCurrentConversationHistory() {
		const id = chatState.currentConvId;
		if (!chatState.conversationsLoaded) return;
		if (!chatState.fetchedConvIds.has(id)) {
			fetchHistory(id, chatState.historyHours)
				.then((records) => {
					chatState.mergeHistory(id, records);
					void chatState.markCurrentRead(id);
				})
				.catch(() => undefined);
		} else {
			void chatState.markCurrentRead(id);
		}
	}

	async reloadProtectedData() {
		const conversations = await fetchConversations();
		chatState.setConversations(conversations);
	}

	handleUnauthorized() {
		this.clearSendEcho();
		connectionState.setState('unauthenticated');
		this.authModalOpen = true;
		this.authError = null;
		this.authPassword = '';
	}

	async submitAuth() {
		if (this.authSubmitting) return;
		this.authSubmitting = true;
		this.authError = null;
		try {
			await login(this.authUsername.trim(), this.authPassword);
			this.authModalOpen = false;
			eventsState.clear();
			eventsState.storedPositions = [];
			chatState.resetAfterLogin();
			this.sse.restart();
			await this.reloadProtectedData();
		} catch (error) {
			this.authError =
				error instanceof UnauthorizedError ? 'Invalid username or password' : 'Login failed';
		} finally {
			this.authSubmitting = false;
		}
	}

	openDeleteConfirm() {
		chatState.deleteError = null;
		chatState.deleteConfirmOpen = true;
	}

	async confirmDelete() {
		if (chatState.deleting) return;
		const id = chatState.currentConvId;
		chatState.deleting = true;
		try {
			await deleteConversation(id);
			chatState.deleteLocalConversation(id);
			chatState.deleteConfirmOpen = false;
		} catch (e) {
			if (e instanceof UnauthorizedError) return;
			chatState.deleteError = e instanceof Error ? e.message : 'Delete failed';
		} finally {
			chatState.deleting = false;
		}
	}

	openNewDm() {
		chatState.newDmCallsign = '';
		chatState.newDmError = '';
		chatState.newDmOpen = true;
	}

	openNewChannel() {
		chatState.newChannelValue = '';
		chatState.newChannelError = '';
		chatState.newChannelOpen = true;
	}

	async confirmChannelShow() {
		if (chatState.channelShowSaving) return;
		const cfg = {
			mode: chatState.channelShowDraftMode,
			channels: chatState.channelShowDraftMode === 'all' ? [] : [...chatState.channelShowDraftChannels]
		};
		chatState.channelShowSaving = true;
		chatState.channelShowError = '';
		try {
			const normalized = await updateChannelShow(cfg);
			chatState.setChannelShow(normalized);
			chatState.channelShowOpen = false;
		} catch (e) {
			if (e instanceof UnauthorizedError) return;
			chatState.channelShowError = e instanceof Error ? e.message : 'Save failed';
		} finally {
			chatState.channelShowSaving = false;
		}
	}

	async confirmNewChannel() {
		const raw = chatState.newChannelValue.trim();
		if (raw !== '*' && !/^\d+$/.test(raw)) {
			chatState.newChannelError = 'Enter * for broadcast or a numeric channel (e.g. 222)';
			return;
		}
		chatState.newChannelOpen = false;
		chatState.selectChannel(raw === '*' ? 'Broadcast' : raw);
		await this.addChannelToAllowlistIfNeeded(raw);
	}

	private async addChannelToAllowlistIfNeeded(channelId: string) {
		const current = chatState.channelShow;
		if (current.mode !== 'allowlist') return;
		if (current.channels.includes(channelId)) return;
		const updated = { mode: current.mode, channels: [...current.channels, channelId] };
		try {
			const normalized = await updateChannelShow(updated);
			chatState.setChannelShow(normalized);
		} catch {
			// fire-and-forget: next SSE reconnect reconciles
		}
	}

	confirmNewDm() {
		const call = normalizeCallsign(chatState.newDmCallsign);
		if (!isValidCallsign(call)) {
			chatState.newDmError = 'Invalid callsign (e.g. XX5YYY-1 or IU5PMP)';
			return;
		}
		chatState.newDmOpen = false;
		chatState.selectContact(call);
	}

	clearSendEcho() {
		if (this.sendEchoTimer !== null) {
			clearTimeout(this.sendEchoTimer);
			this.sendEchoTimer = null;
		}
		chatState.sending = false;
	}

	async handleSend() {
		const text = chatState.draftMessage.trim();
		if (!text || chatState.sending || connectionState.txDisabled) return;
		const dst = destinationFor(chatState.chatTarget);
		const pendingRecord = this.createPendingChatRecord(dst, text);
		chatState.sending = true;
		chatState.sendError = null;
		chatState.appendChatRecord(pendingRecord);
		try {
			await sendMessage(dst, text);
			chatState.draftMessage = '';
			this.sendEchoTimer = setTimeout(() => this.clearSendEcho(), 5000);
		} catch (e) {
			chatState.removeChatRecord(pendingRecord);
			if (e instanceof UnauthorizedError) return;
			if (e instanceof SendError && e.status === 429) {
				chatState.sendError = 'Duplicate ignored (sent <2s ago)';
			} else {
				chatState.sendError = e instanceof Error ? e.message : 'Send failed';
			}
			chatState.sending = false;
		}
	}

	createPendingChatRecord(dst: string, msg: string): import('$lib/api/types').ChatRecord {
		return {
			received_at: new Date().toISOString(),
			src: connectionState.stationCallsign || 'Me',
			dst: dst || undefined,
			msg,
			direction: 'outbound',
			delivery_status: 'pending',
			source: 'event-live'
		};
	}
}

export function createAppController() {
	return new AppController();
}
