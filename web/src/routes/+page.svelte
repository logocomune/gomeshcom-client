<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { login, onUnauthorized, UnauthorizedError } from '$lib/api/auth';
	import {
		applyLiveFreshness,
		connectEvents,
		eventJSON,
		messageKind,
		packetBadge,
		isReplayEvent,
		packetFromEvent,
		prependEvent,
		stationCallsignFromEvent,
		splitSourcePath
	} from '$lib/api/events';
	import { base } from '$app/paths';
	import { stationStore } from '$lib/stores/station';
	import {
		fetchConversations,
		fetchHistory,
		conversationIdFor,
		conversationIdForRecord,
		chatRecordKey,
		sendMessage,
		destinationFor,
		SendError,
		loadReadTimestamps,
		saveReadTimestamp,
		isUnread,
		deleteConversation,
		clearReadTimestamp,
		loadLastChatTarget,
		saveLastChatTarget
	} from '$lib/api/chat';
	import { buildAckIndex } from '$lib/api/acks';
	import logo from '$lib/assets/gomeshcom-logo.png';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import UdpStreamPanel from '$lib/components/UdpStreamPanel.svelte';
	import ChatPanel from '$lib/components/ChatPanel.svelte';
	import ConnectionOverlay from '$lib/components/ConnectionOverlay.svelte';
	import { watchDesktop } from '$lib/responsive';
	import MeshMapPanel from '$lib/map/MeshMapPanel.svelte';
	import type { ConnectionState, StreamEvent, Conversation, ChatRecord } from '$lib/api/types';
	import type { ChatTarget } from '$lib/api/chat';
	import type { MapPosition } from '$lib/map/types';
	import { partitionChannels } from '$lib/api/groups';
	import { loadChatChannelsCollapsed, saveChatChannelsCollapsed } from '$lib/ui/chat-layout';
	import { formatTime } from '$lib/ui/format';
	import { chatMdiIcon, chatRecordMatchesFilter, stripNodeSequence } from '$lib/ui/chat-records';
	import { filterStreamEvents, mdiForEvent, packetTone } from '$lib/ui/stream';
	import { mdiClose } from '@mdi/js';

	let events = $state<StreamEvent[]>([]);
	let connection = $state<ConnectionState>('connecting');
	let rawEvent = $state<StreamEvent | null>(null);
	let selectedEvent = $state<StreamEvent | null>(null);
	let storedPositions = $state<MapPosition[]>([]);
	let stationCallsign = $state('');
	let appVersion = $state('');
	let txDisabled = $state(false);
	let chatTarget = $state<ChatTarget>({
		kind: 'channel',
		value: 'Broadcast'
	});
	let draftMessage = $state('');
	let sending = $state(false);
	let sendError = $state<string | null>(null);
	let sendEchoTimer: ReturnType<typeof setTimeout> | null = null;
	let streamFilter = $state('');
	let chatFilter = $state('');
	let conversations = $state<Conversation[]>([]);
	let chatHistory = $state<Record<string, ChatRecord[]>>({});
	let historyHours = $state(168);
	let rawChatRecord = $state<ChatRecord | null>(null);
	let newDmOpen = $state(false);
	let menuOpen = $state(false);
	let newDmCallsign = $state('');
	let newDmError = $state('');
	let fetchedConvIds = $state(new Set<string>());
	let deleteConfirmOpen = $state(false);
	let deleting = $state(false);
	let deleteError = $state<string | null>(null);
	let authModalOpen = $state(false);
	let authUsername = $state('');
	let authPassword = $state('');
	let authError = $state<string | null>(null);
	let authSubmitting = $state(false);
	let channelsCollapsed = $state(false);

	let stopStream: (() => void) | null = null;
	let stopUnauthorizedWatch: (() => void) | null = null;

	const STORAGE_CHAT_WIDTH = 'meshcom:chatWidthPct';
	const STORAGE_STREAM_HEIGHT = 'meshcom:streamHeightPx';
	const STORAGE_STREAM_REPLAY_FROM = 'meshcom:streamReplayFrom';
	const DEFAULT_CHAT_WIDTH = 50;
	const DEFAULT_STREAM_HEIGHT = 300;

	let chatWidthPct = $state(DEFAULT_CHAT_WIDTH);
	let streamHeightPx = $state(DEFAULT_STREAM_HEIGHT);

	function loadLayout() {
		const w = parseFloat(localStorage.getItem(STORAGE_CHAT_WIDTH) ?? '');
		const h = parseFloat(localStorage.getItem(STORAGE_STREAM_HEIGHT) ?? '');
		if (!isNaN(w) && w >= 20 && w <= 80) chatWidthPct = w;
		if (!isNaN(h) && h >= 160) streamHeightPx = h;
		channelsCollapsed = loadChatChannelsCollapsed(localStorage);
	}

	function saveLayout() {
		localStorage.setItem(STORAGE_CHAT_WIDTH, String(chatWidthPct));
		localStorage.setItem(STORAGE_STREAM_HEIGHT, String(streamHeightPx));
		saveChatChannelsCollapsed(localStorage, channelsCollapsed);
	}

	function startHorizontalDrag(e: PointerEvent) {
		e.preventDefault();
		const startX = e.clientX;
		const startPct = chatWidthPct;
		const row = (e.currentTarget as HTMLElement).closest('[data-panel-row]') as HTMLElement;

		function onMove(ev: PointerEvent) {
			const totalW = row.offsetWidth;
			const delta = ev.clientX - startX;
			chatWidthPct = Math.max(20, Math.min(80, startPct + (delta / totalW) * 100));
		}
		function onUp() {
			saveLayout();
			window.removeEventListener('pointermove', onMove);
			window.removeEventListener('pointerup', onUp);
		}
		window.addEventListener('pointermove', onMove);
		window.addEventListener('pointerup', onUp);
	}

	function startVerticalDrag(e: PointerEvent) {
		e.preventDefault();
		const startY = e.clientY;
		const startH = streamHeightPx;
		const maxH = window.innerHeight * 0.72;

		function onMove(ev: PointerEvent) {
			const delta = startY - ev.clientY;
			streamHeightPx = Math.max(160, Math.min(maxH, startH + delta));
		}
		function onUp() {
			saveLayout();
			window.removeEventListener('pointermove', onMove);
			window.removeEventListener('pointerup', onUp);
		}
		window.addEventListener('pointermove', onMove);
		window.addEventListener('pointerup', onUp);
	}

	function toggleChannelsSidebar() {
		channelsCollapsed = !channelsCollapsed;
		saveLayout();
	}
	let isDesktop = $state(true);
	$effect(() => watchDesktop((v) => (isDesktop = v)));

	let mapPositions = $derived(applyLiveFreshness(storedPositions, events));

	let readTimestamps = $state<Record<string, string>>({});

	let unreadIds = $derived(
		new Set(conversations.filter((c) => isUnread(c, readTimestamps[c.id])).map((c) => c.id))
	);

	let channelLabels = $derived(conversations.filter((c) => c.kind !== 'dm').map((c) => c.label));
	let resolvedChannels = $derived(partitionChannels(channelLabels));
	let contacts = $derived(conversations.filter((c) => c.kind === 'dm').map((c) => c.label));

	let currentConvId = $derived(conversationIdFor(chatTarget));
	let isBroadcastTarget = $derived(
		chatTarget.kind === 'channel' && chatTarget.value === 'Broadcast'
	);
	let ackIndex = $derived(buildAckIndex(events));

	let displayChatRecords = $derived(
		(chatHistory[currentConvId] ?? []).filter((rec) => {
			const kind = messageKind(rec.msg).kind;
			if (kind === 'ack' || kind === 'reject') return false;
			return chatRecordMatchesFilter(rec, chatFilter);
		})
	);

	$effect(() => {
		const id = currentConvId;
		if (!fetchedConvIds.has(id)) {
			fetchHistory(id, historyHours)
				.then((records) => {
					fetchedConvIds = new Set([...fetchedConvIds, id]);
					const live = chatHistory[id] ?? [];
					const seen = new Set<string>();
					const merged = [...records, ...live]
						.filter((r) => {
							const key = chatRecordKey(r);
							if (seen.has(key)) return false;
							seen.add(key);
							return true;
						})
						.sort((a, b) => a.received_at.localeCompare(b.received_at));
					chatHistory = { ...chatHistory, [id]: merged };
				})
				.catch(() => undefined);
		}
	});

	let filteredEvents = $derived(filterStreamEvents(events, streamFilter));

	const statusText: Record<ConnectionState, string> = {
		connecting: 'Connecting',
		connected: 'Connected',
		disconnected: 'Disconnected',
		unauthenticated: 'Locked'
	};

	const statusClass: Record<ConnectionState, string> = {
		connecting: 'bg-amber-400 shadow-[0_0_8px_rgba(251,191,36,0.6)]',
		connected: 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.6)]',
		disconnected: 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.6)]',
		unauthenticated: 'bg-amber-300 shadow-[0_0_8px_rgba(252,211,77,0.6)]'
	};

	onMount(async () => {
		loadLayout();
		readTimestamps = loadReadTimestamps();
		stopUnauthorizedWatch = onUnauthorized(handleUnauthorized);
		startStream();
		try {
			await reloadProtectedData();
		} catch {
			// modal state handled by unauthorized listener
		}
	});

	onDestroy(() => {
		stopStream?.();
		stopUnauthorizedWatch?.();
		clearSendEcho();
	});

	function streamReplayFrom(): string | undefined {
		return localStorage.getItem(STORAGE_STREAM_REPLAY_FROM) ?? undefined;
	}

	function clearUdpStream() {
		localStorage.setItem(STORAGE_STREAM_REPLAY_FROM, new Date().toISOString());
		events = [];
		selectedEvent = null;
		rawEvent = null;
	}

	function startStream() {
		stopStream?.();
		const replayFrom = streamReplayFrom();
		stopStream = connectEvents(
			{
				onState: (state) => {
					connection = state;
				},
				onPositions: (positions) => {
					storedPositions = positions;
				},
				onStation: (station) => {
					stationCallsign = station.callsign;
					if (station.version) appVersion = station.version;
					txDisabled = !!station.txDisabled;
					stationStore.set(station);
				},
				onEvent: (event) => {
					events = prependEvent(events, event);
					selectedEvent ??= event;
					stationCallsign ||= stationCallsignFromEvent(event) ?? '';

					const packet = packetFromEvent(event);
					if (packet?.type === 'msg' && !isReplayEvent(event)) {
						appendLiveChatRecord(packet, event.receivedAt);
						if (sending && splitSourcePath(packet.src).origin === stationCallsign) {
							clearSendEcho();
						}
					}
					if (event.type === 'message.failed') {
						appendChatRecord(event.data as ChatRecord);
						clearSendEcho();
					}
				}
			},
			replayFrom ? { replayFrom } : {}
		);
	}

	async function reloadProtectedData() {
		conversations = await fetchConversations();
		chatTarget = loadLastChatTarget(conversations);
		saveLastChatTarget(chatTarget);
		markRead(conversationIdFor(chatTarget));
		const updates: Record<string, string> = {};
		for (const c of conversations) {
			if (!readTimestamps[c.id] && c.last_seen) {
				saveReadTimestamp(c.id, c.last_seen);
				updates[c.id] = c.last_seen;
			}
		}
		if (Object.keys(updates).length > 0) {
			readTimestamps = { ...readTimestamps, ...updates };
		}
	}

	function handleUnauthorized() {
		clearSendEcho();
		connection = 'unauthenticated';
		authModalOpen = true;
		authError = null;
		authPassword = '';
	}

	async function submitAuth() {
		if (authSubmitting) return;
		authSubmitting = true;
		authError = null;
		try {
			await login(authUsername.trim(), authPassword);
			authModalOpen = false;
			events = [];
			storedPositions = [];
			chatHistory = {};
			fetchedConvIds = new Set();
			startStream();
			await reloadProtectedData();
		} catch (error) {
			authError =
				error instanceof UnauthorizedError ? 'Invalid username or password' : 'Login failed';
		} finally {
			authSubmitting = false;
		}
	}

	function markRead(convId: string) {
		const now = new Date().toISOString();
		saveReadTimestamp(convId, now);
		readTimestamps = { ...readTimestamps, [convId]: now };
	}

	function selectChannel(channel: string) {
		chatTarget = { kind: 'channel', value: channel };
		saveLastChatTarget(chatTarget);
		markRead(conversationIdFor({ kind: 'channel', value: channel }));
	}

	function selectContact(contact: string) {
		chatTarget = { kind: 'contact', value: contact };
		saveLastChatTarget(chatTarget);
		markRead(conversationIdFor({ kind: 'contact', value: contact }));
	}

	function openDeleteConfirm() {
		deleteError = null;
		deleteConfirmOpen = true;
	}

	async function confirmDelete() {
		if (deleting) return;
		const id = currentConvId;
		const wasBroadcast = isBroadcastTarget;
		deleting = true;
		try {
			await deleteConversation(id);

			const nextHistory = { ...chatHistory };
			delete nextHistory[id];
			chatHistory = nextHistory;

			const nextReads = { ...readTimestamps };
			delete nextReads[id];
			readTimestamps = nextReads;
			clearReadTimestamp(id);

			if (wasBroadcast) {
				chatHistory = { ...chatHistory, [id]: [] };
				const now = new Date().toISOString();
				saveReadTimestamp(id, now);
				readTimestamps = { ...readTimestamps, [id]: now };
			} else {
				conversations = conversations.filter((c) => c.id !== id);
				chatTarget = { kind: 'channel', value: 'Broadcast' };
				saveLastChatTarget(chatTarget);
			}
			deleteConfirmOpen = false;
		} catch (e) {
			if (e instanceof UnauthorizedError) return;
			deleteError = e instanceof Error ? e.message : 'Delete failed';
		} finally {
			deleting = false;
		}
	}

	function openNewDm() {
		newDmCallsign = '';
		newDmError = '';
		newDmOpen = true;
	}

	const callsignPattern = /^[A-Z0-9]{3,10}(-[0-9]{1,2})?$/;

	function confirmNewDm() {
		const call = newDmCallsign.trim().toUpperCase();
		if (!callsignPattern.test(call)) {
			newDmError = 'Invalid callsign (e.g. IU5PMP-1)';
			return;
		}
		newDmOpen = false;
		selectContact(call);
	}

	function clearSendEcho() {
		if (sendEchoTimer !== null) {
			clearTimeout(sendEchoTimer);
			sendEchoTimer = null;
		}
		sending = false;
	}

	async function handleSend() {
		const text = draftMessage.trim();
		if (!text || sending || txDisabled) return;
		const dst = destinationFor(chatTarget);
		const pendingRecord = createPendingChatRecord(dst, text);
		sending = true;
		sendError = null;
		appendChatRecord(pendingRecord);
		try {
			await sendMessage(dst, text);
			draftMessage = '';
			// Keep spinner until echo from node or 5s timeout.
			sendEchoTimer = setTimeout(clearSendEcho, 5000);
		} catch (e) {
			removeChatRecord(pendingRecord);
			if (e instanceof UnauthorizedError) return;
			if (e instanceof SendError && e.status === 429) {
				sendError = 'Duplicate ignored (sent <2s ago)';
			} else {
				sendError = e instanceof Error ? e.message : 'Send failed';
			}
			sending = false;
		}
	}

	function createPendingChatRecord(dst: string, msg: string): ChatRecord {
		return {
			received_at: new Date().toISOString(),
			src: stationCallsign || 'Me',
			dst: dst || undefined,
			msg,
			direction: 'outbound',
			delivery_status: 'pending',
			source: 'event-live'
		};
	}

	function appendLiveChatRecord(
		packet: import('$lib/api/types').MeshcomPacket,
		receivedAt: string
	) {
		const dst = packet.dst ?? '';
		const origin = splitSourcePath(packet.src).origin.toUpperCase();

		let convId: string;
		if (dst === '' || dst === '*') {
			convId = 'P_broadcast';
		} else if (/^\d+$/.test(dst)) {
			convId = 'P_' + dst;
		} else {
			// DM — normalise on interlocutor so both directions share one conversation.
			const myCall = stationCallsign ? stationCallsign.toUpperCase() : '';
			const dstUpper = dst.toUpperCase();
			if (myCall && dstUpper !== myCall && origin !== myCall) {
				return; // DM not involving us
			}
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

		removeMatchingPendingRecord(convId, rec);
		appendChatRecordToConversation(convId, rec);

		const idx = conversations.findIndex((c) => c.id === convId);
		if (idx === -1) {
			let kind: Conversation['kind'] = 'broadcast';
			let label = 'Broadcast';
			if (dst !== '' && dst !== '*') {
				if (/^\d+$/.test(dst)) {
					kind = 'channel';
					label = dst;
				} else {
					kind = 'dm';
					// label = interlocutor callsign (already resolved in convId computation above)
					label = convId.replace(/^DM_/, '');
				}
			}
			conversations = [
				{ id: convId, kind, label, last_seen: receivedAt, size: 0 },
				...conversations
			];
		} else {
			conversations = conversations.map((c) =>
				c.id === convId ? { ...c, last_seen: receivedAt } : c
			);
		}

		// Keep active conversation marked read so it doesn't light up while open.
		if (convId === currentConvId) {
			saveReadTimestamp(convId, receivedAt);
			readTimestamps = { ...readTimestamps, [convId]: receivedAt };
		}
	}

	function appendChatRecord(rec: ChatRecord) {
		const convId = conversationIdForRecord(rec, stationCallsign);
		if (!convId) return;
		if (rec.delivery_status === 'failed') {
			removeMatchingPendingRecord(convId, rec);
		}
		appendChatRecordToConversation(convId, { ...rec, source: 'event-live' });

		const idx = conversations.findIndex((c) => c.id === convId);
		if (idx === -1) {
			conversations = [
				{
					id: convId,
					kind: convId === 'P_broadcast' ? 'broadcast' : convId.startsWith('P_') ? 'channel' : 'dm',
					label: convId === 'P_broadcast' ? 'Broadcast' : convId.replace(/^(P_|DM_)/, ''),
					last_seen: rec.received_at,
					size: 0
				},
				...conversations
			];
		} else {
			const copy = conversations.slice();
			copy[idx] = { ...copy[idx], last_seen: rec.received_at };
			conversations = copy.sort((a, b) => b.last_seen.localeCompare(a.last_seen));
		}

		if (convId === currentConvId) {
			saveReadTimestamp(convId, rec.received_at);
			readTimestamps = { ...readTimestamps, [convId]: rec.received_at };
		}
	}

	function removeChatRecord(rec: ChatRecord) {
		const convId = conversationIdForRecord(rec, stationCallsign);
		if (!convId) return;
		const existing = chatHistory[convId] ?? [];
		const key = chatRecordKey(rec);
		chatHistory = {
			...chatHistory,
			[convId]: existing.filter((item) => chatRecordKey(item) !== key)
		};
	}

	function appendChatRecordToConversation(convId: string, rec: ChatRecord) {
		const existing = chatHistory[convId] ?? [];
		const key = chatRecordKey(rec);
		if (existing.some((r) => chatRecordKey(r) === key)) return;
		chatHistory = {
			...chatHistory,
			[convId]: [...existing, rec].sort((a, b) => a.received_at.localeCompare(b.received_at))
		};
	}

	function removeMatchingPendingRecord(convId: string, rec: ChatRecord) {
		const existing = chatHistory[convId] ?? [];
		const recDst = rec.dst ?? '';
		const recMsg = stripNodeSequence(rec.msg);
		const next = existing.filter((item) => {
			if (item.delivery_status !== 'pending') return true;
			if ((item.dst ?? '') !== recDst) return true;
			return item.msg !== recMsg;
		});
		if (next.length === existing.length) return;
		chatHistory = { ...chatHistory, [convId]: next };
	}
</script>

<svelte:head>
	<title>goMeshCom</title>
</svelte:head>

<main class="relative flex h-screen min-h-0 flex-col bg-[#111827] text-gray-100">
	<ConnectionOverlay state={connection} />
	{#if authModalOpen}
		<div
			class="fixed inset-0 z-[10000] flex items-center justify-center bg-[#020617]/75 p-4 backdrop-blur-sm"
		>
			<form
				class="w-full max-w-sm rounded-2xl border border-slate-700 bg-slate-950 p-5 shadow-2xl"
				onsubmit={(event) => {
					event.preventDefault();
					void submitAuth();
				}}
			>
				<div class="mb-4">
					<h2 class="text-lg font-semibold text-slate-100">Sign in</h2>
					<p class="mt-1 text-sm text-slate-400">Protected MeshCom session on local server.</p>
				</div>
				<div class="space-y-3">
					<label class="block text-sm text-slate-300">
						<span class="mb-1 block">Username</span>
						<input
							class="w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-slate-100 outline-none placeholder:text-slate-500 focus:border-amber-300"
							type="text"
							autocomplete="username"
							bind:value={authUsername}
						/>
					</label>
					<label class="block text-sm text-slate-300">
						<span class="mb-1 block">Password</span>
						<input
							class="w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-slate-100 outline-none placeholder:text-slate-500 focus:border-amber-300"
							type="password"
							autocomplete="current-password"
							bind:value={authPassword}
						/>
					</label>
				</div>
				{#if authError}
					<p
						class="mt-3 rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-200"
					>
						{authError}
					</p>
				{/if}
				<button
					class="mt-4 w-full rounded-lg bg-amber-300 px-3 py-2 text-sm font-semibold text-slate-950 transition hover:bg-amber-200 disabled:cursor-not-allowed disabled:opacity-60"
					type="submit"
					disabled={authSubmitting}
				>
					{authSubmitting ? 'Signing in...' : 'Sign in'}
				</button>
			</form>
		</div>
	{/if}

	<!-- Header -->
	<header
		class="flex h-12 shrink-0 items-center justify-between border-b border-gray-700/60 bg-[#2d3345] px-4"
	>
		<div class="flex items-center gap-3">
			<div class="flex items-center gap-2">
				<img src={logo} alt="" class="h-7 w-7 rounded-md" />
				<span class="font-mono text-sm font-bold tracking-wide text-blue-300">goMeshCom</span>
				<!-- mobile-only status dot, no text -->
				<span
					data-testid="status-dot"
					class="md:hidden h-2 w-2 rounded-full {statusClass[connection]}"
				></span>
			</div>
			<div class="hidden md:block h-4 w-px bg-gray-600"></div>
			<div data-testid="status-pill" class="hidden md:flex items-center gap-2 text-xs">
				<span class="h-2.5 w-2.5 rounded-full {statusClass[connection]}"></span>
				<span class="font-medium text-gray-300">{statusText[connection]}</span>
			</div>
		</div>
		<div class="flex items-center gap-3">
			<div
				class="rounded border border-blue-500/40 bg-blue-500/10 px-2 py-1 font-mono text-xs font-semibold text-blue-200"
			>
				{stationCallsign || 'NO-CALL'}
			</div>
			<div data-testid="packet-counter" class="hidden md:block font-mono text-xs text-gray-400">
				{events.length} packets
			</div>
			<!-- menu -->
			<div class="relative">
				<button
					type="button"
					id="app-menu-btn"
					class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-gray-500 hover:text-gray-200"
					aria-label="Menu"
					onclick={() => (menuOpen = !menuOpen)}
				>
					<svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z" />
					</svg>
				</button>
				{#if menuOpen}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div class="fixed inset-0 z-[9999]" onclick={() => (menuOpen = false)}></div>
					<div
						class="fixed right-4 top-12 z-[10000] w-36 rounded-md border border-gray-700/60 bg-[#212735] shadow-xl"
					>
						<a
							href="{base}/about"
							class="block px-4 py-2.5 text-xs text-gray-300 hover:bg-gray-700/40 hover:text-white"
							onclick={() => (menuOpen = false)}>About</a
						>
						<a
							href="{base}/credits"
							class="block px-4 py-2.5 text-xs text-gray-300 hover:bg-gray-700/40 hover:text-white"
							onclick={() => (menuOpen = false)}>Credits</a
						>
					</div>
				{/if}
			</div>
		</div>
	</header>

	<!-- Content wrapper -->
	<div class="flex min-h-0 flex-1 flex-col gap-2 p-2 overflow-y-auto md:overflow-hidden">
		<!-- Top row: Chat + Map -->
		<div class="flex flex-col md:flex-row shrink-0 md:flex-1 md:min-h-0 gap-2" data-panel-row>
			<ChatPanel
				{ackIndex}
				{channelsCollapsed}
				bind:chatFilter
				{chatTarget}
				{chatWidthPct}
				{contacts}
				{displayChatRecords}
				bind:draftMessage
				{isBroadcastTarget}
				{isDesktop}
				{resolvedChannels}
				{sendError}
				{sending}
				{stationCallsign}
				{txDisabled}
				{unreadIds}
				{handleSend}
				onDelete={openDeleteConfirm}
				onNewDm={openNewDm}
				onSelectChannel={selectChannel}
				onSelectContact={selectContact}
				onShowRawRecord={(record) => (rawChatRecord = record)}
				onToggleChannels={toggleChannelsSidebar}
			/>

			<!-- Vertical drag handle -->
			<div
				role="separator"
				aria-orientation="vertical"
				aria-label="Resize chat and map"
				class="hidden md:flex group relative z-10 mx-1 w-2 shrink-0 cursor-col-resize items-center justify-center"
				onpointerdown={startHorizontalDrag}
			>
				<div
					class="h-12 w-0.5 rounded-full bg-gray-700/60 transition-colors group-hover:bg-blue-500/60 group-active:bg-blue-500"
				></div>
			</div>

			<!-- Map panel -->
			<div
				data-testid="map-panel"
				class="flex h-[80vh] md:h-auto md:min-h-0 flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#151a24] shadow-sm"
				style={isDesktop ? 'flex: 1 1 0; min-width: 0' : ''}
			>
				<div
					class="flex h-9 shrink-0 items-center justify-between border-b border-gray-700/60 px-3"
				>
					<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">Map</span>
					<span class="font-mono text-[11px] text-gray-500">{mapPositions.length} nodes</span>
				</div>
				<div class="relative min-h-0 flex-1 overflow-hidden">
					<MeshMapPanel positions={mapPositions} myCall={stationCallsign} />
				</div>
			</div>
		</div>

		<!-- Horizontal drag handle for UDP stream -->
		<div
			role="separator"
			aria-orientation="horizontal"
			aria-label="Resize UDP stream panel"
			class="hidden md:flex group relative z-10 my-0.5 h-2 shrink-0 cursor-row-resize items-center justify-center"
			onpointerdown={startVerticalDrag}
		>
			<div
				class="h-0.5 w-12 rounded-full bg-gray-700/60 transition-colors group-hover:bg-blue-500/60 group-active:bg-blue-500"
			></div>
		</div>

		<UdpStreamPanel
			{events}
			{filteredEvents}
			bind:streamFilter
			{selectedEvent}
			{isDesktop}
			{streamHeightPx}
			onClearEvents={clearUdpStream}
			selectEvent={(event) => (isDesktop ? (selectedEvent = event) : (rawEvent = event))}
			showRawEvent={(event) => (rawEvent = event)}
		/>
	</div>
</main>

{#if rawEvent}
	<div
		class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
	>
		<div
			class="flex max-h-[86vh] w-full max-w-4xl flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
		>
			<div class="flex h-12 shrink-0 items-center justify-between border-b border-gray-700/60 px-4">
				<div class="flex items-center gap-2">
					<span
						class="flex h-7 w-7 items-center justify-center rounded border {packetTone(rawEvent)}"
					>
						<MdiIcon path={mdiForEvent(rawEvent)} size={16} />
					</span>
					<div>
						<div class="text-sm font-semibold text-gray-100">Formatted JSON</div>
						<div class="font-mono text-[11px] text-gray-500">
							{packetBadge(rawEvent)} · {formatTime(rawEvent.receivedAt)}
						</div>
					</div>
				</div>
				<button
					type="button"
					class="flex h-8 w-8 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (rawEvent = null)}
				>
					<MdiIcon path={mdiClose} size={18} />
				</button>
			</div>
			<pre
				class="min-h-0 overflow-auto p-4 font-mono text-xs leading-relaxed text-gray-200">{eventJSON(
					rawEvent
				)}</pre>
		</div>
	</div>
{/if}

{#if rawChatRecord}
	<div
		class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
	>
		<div
			class="flex max-h-[86vh] w-full max-w-4xl flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
		>
			<div class="flex h-12 shrink-0 items-center justify-between border-b border-gray-700/60 px-4">
				<div class="flex items-center gap-2">
					<span
						class="flex h-7 w-7 items-center justify-center rounded border border-blue-500/70 bg-blue-500/25 text-blue-200"
					>
						<MdiIcon path={chatMdiIcon(rawChatRecord)} size={16} />
					</span>
					<div>
						<div class="text-sm font-semibold text-gray-100">Chat Record JSON</div>
						<div class="font-mono text-[11px] text-gray-500">
							msg · {formatTime(rawChatRecord.received_at)}
						</div>
					</div>
				</div>
				<button
					type="button"
					class="flex h-8 w-8 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (rawChatRecord = null)}
				>
					<MdiIcon path={mdiClose} size={18} />
				</button>
			</div>
			<pre
				class="min-h-0 overflow-auto p-4 font-mono text-xs leading-relaxed text-gray-200">{JSON.stringify(
					rawChatRecord,
					null,
					2
				)}</pre>
		</div>
	</div>
{/if}

{#if deleteConfirmOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
	<div
		class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
		onclick={() => (deleteConfirmOpen = false)}
	>
		<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
		<div
			class="w-full max-w-sm rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="flex h-11 items-center justify-between border-b border-gray-700/60 px-4">
				<span class="text-sm font-semibold text-gray-100">
					{isBroadcastTarget ? 'Clear messages' : 'Delete conversation'}
				</span>
				<button
					type="button"
					class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (deleteConfirmOpen = false)}
				>
					<MdiIcon path={mdiClose} size={16} />
				</button>
			</div>
			<div class="p-4">
				<p class="text-sm text-gray-300">
					{#if isBroadcastTarget}
						Clear all messages in <span class="font-semibold text-white">Broadcast</span>?
					{:else}
						Delete <span class="font-semibold text-white">{chatTarget.value}</span>?
					{/if}
					This removes {displayChatRecords.length} message(s) from disk.
				</p>
				{#if deleteError}
					<p class="mt-2 text-xs text-red-400">{deleteError}</p>
				{/if}
				<div class="mt-4 flex justify-end gap-2">
					<button
						type="button"
						class="rounded border border-gray-700/60 px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200"
						onclick={() => (deleteConfirmOpen = false)}
						disabled={deleting}>Cancel</button
					>
					<button
						type="button"
						class="rounded border border-red-500/40 bg-red-600/80 px-3 py-1.5 text-xs text-white hover:bg-red-500 disabled:opacity-50"
						onclick={confirmDelete}
						disabled={deleting}
					>
						{deleting ? '…' : isBroadcastTarget ? 'Clear' : 'Delete'}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

{#if newDmOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
	<div
		class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
		onclick={() => (newDmOpen = false)}
	>
		<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
		<div
			class="w-full max-w-sm rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="flex h-11 items-center justify-between border-b border-gray-700/60 px-4">
				<span class="text-sm font-semibold text-gray-100">New Direct Message</span>
				<button
					type="button"
					class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (newDmOpen = false)}
				>
					<MdiIcon path={mdiClose} size={16} />
				</button>
			</div>
			<div class="p-4">
				<label class="mb-1.5 block text-xs text-gray-400" for="new-dm-callsign">Callsign</label>
				<input
					id="new-dm-callsign"
					class="w-full rounded border border-gray-700/60 bg-[#111827] px-3 py-2 font-mono text-sm text-gray-200 outline-none placeholder:text-gray-600 focus:border-blue-500/60"
					placeholder="IU5PMP-1"
					bind:value={newDmCallsign}
					oninput={() => (newDmError = '')}
					onkeydown={(e) => {
						if (e.key === 'Enter') confirmNewDm();
						if (e.key === 'Escape') newDmOpen = false;
					}}
				/>
				{#if newDmError}
					<p class="mt-1 text-xs text-red-400">{newDmError}</p>
				{/if}
				<div class="mt-3 flex justify-end gap-2">
					<button
						type="button"
						class="rounded border border-gray-700/60 px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200"
						onclick={() => (newDmOpen = false)}
					>
						Cancel
					</button>
					<button
						type="button"
						class="rounded border border-blue-500/40 bg-blue-600/80 px-3 py-1.5 text-xs text-white hover:bg-blue-500"
						onclick={confirmNewDm}
					>
						Open Chat
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
