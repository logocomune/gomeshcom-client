<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import { login, onUnauthorized, UnauthorizedError } from '$lib/api/auth';
	import {
		applyLiveFreshness,
		cleanMessage,
		connectEvents,
		eventJSON,
		eventSummary,
		messageKind,
		packetBadge,
		packetFromEvent,
		prependEvent,
		stationCallsignFromEvent,
		splitSourcePath,
		stripMessagePrefix
	} from '$lib/api/events';
	import { base } from '$app/paths';
	import { stationStore } from '$lib/stores/station';
	import {
		fetchConversations,
		fetchHistory,
		conversationIdFor,
		chatRecordKey,
		sendMessage,
		destinationFor,
		SendError,
		loadReadTimestamps,
		saveReadTimestamp,
		isUnread,
		deleteConversation,
		clearReadTimestamp
	} from '$lib/api/chat';
	import { buildAckIndex } from '$lib/api/acks';
	import { hardwareHumanName } from '$lib/api/hardware';
	import logo from '$lib/assets/gomeshcom-logo.png';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import ConnectionOverlay from '$lib/components/ConnectionOverlay.svelte';
	import { watchDesktop } from '$lib/responsive';
	import MeshMapPanel from '$lib/map/MeshMapPanel.svelte';
	import { getSendComposerState } from '$lib/ui/send';
	import type { ConnectionState, StreamEvent, Conversation, ChatRecord } from '$lib/api/types';
	import type { MapPosition } from '$lib/map/types';
	import { partitionChannels, groupTooltip, resolveGroup } from '$lib/api/groups';
	import {
		chatSidebarGridColumns,
		chatSidebarNewDmLabel,
		loadChatChannelsCollapsed,
		saveChatChannelsCollapsed
	} from '$lib/ui/chat-layout';
	import {
		mdiAccountOutline,
		mdiAlertCircleOutline,
		mdiArrowRight,
		mdiBroadcast,
		mdiChevronLeft,
		mdiChevronRight,
		mdiNotificationClearAll,
		mdiTrashCanOutline,
		mdiCheckCircleOutline,
		mdiChip,
		mdiClockOutline,
		mdiClose,
		mdiCodeJson,
		mdiCogOutline,
		mdiEmailOutline,
		mdiMapMarkerRadiusOutline,
		mdiBattery,
		mdiGauge,
		mdiPlus,
		mdiPound,
		mdiSignalVariant,
		mdiThermometer,
		mdiTune,
		mdiWaterPercent
	} from '@mdi/js';

	let events = $state<StreamEvent[]>([]);
	let connection = $state<ConnectionState>('connecting');
	let rawEvent = $state<StreamEvent | null>(null);
	let selectedEvent = $state<StreamEvent | null>(null);
	let storedPositions = $state<MapPosition[]>([]);
	let stationCallsign = $state('');
	let appVersion = $state('');
	let txDisabled = $state(false);
	let chatTarget = $state<{ kind: 'channel' | 'contact'; value: string }>({
		kind: 'channel',
		value: 'Broadcast'
	});
	let draftMessage = $state('');
	let sending = $state(false);
	let sendError = $state<string | null>(null);
	let sendEchoTimer: ReturnType<typeof setTimeout> | null = null;
	let streamFilter = $state('');
	let chatScrollEl = $state<HTMLDivElement | null>(null);
	let messageInputEl = $state<HTMLInputElement | null>(null);
	let conversations = $state<Conversation[]>([]);
	let chatHistory = $state<Record<string, ChatRecord[]>>({});
	let historyHours = $state(24);
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

	$effect(() => {
		const _deps = [chatTarget, displayChatRecords];
		void _deps;
		tick().then(() => chatScrollEl?.scrollTo({ top: chatScrollEl.scrollHeight }));
	});

	$effect(() => {
		void chatTarget;
		tick().then(() => setTimeout(() => messageInputEl?.focus(), 0));
	});
	let stopStream: (() => void) | null = null;
	let stopUnauthorizedWatch: (() => void) | null = null;

	const STORAGE_CHAT_WIDTH = 'meshcom:chatWidthPct';
	const STORAGE_STREAM_HEIGHT = 'meshcom:streamHeightPx';
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
			return kind !== 'ack' && kind !== 'reject';
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

	let filteredEvents = $derived(
		streamFilter.trim() === ''
			? events
			: (() => {
					const term = streamFilter.trim().toLowerCase();
					return events.filter((event) => {
						const packet = packetFromEvent(event);
						if (eventSummary(event).toLowerCase().includes(term)) return true;
						if (packet?.src?.toLowerCase().includes(term)) return true;
						if (packet?.dst?.toLowerCase().includes(term)) return true;
						if (packet?.msg?.toLowerCase().includes(term)) return true;
						if (packetBadge(event).toLowerCase().includes(term)) return true;
						return false;
					});
				})()
	);

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

	function startStream() {
		stopStream?.();
		stopStream = connectEvents({
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
				if (packet?.type === 'msg') {
					appendLiveChatRecord(packet, event.receivedAt);
					if (sending && splitSourcePath(packet.src).origin === stationCallsign) {
						clearSendEcho();
					}
				}
			}
		});
	}

	async function reloadProtectedData() {
		conversations = await fetchConversations();
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

	function formatTime(value: string): string {
		return new Date(value).toLocaleTimeString('it-IT', {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});
	}

	function packetTone(event: StreamEvent): string {
		const badge = packetBadge(event);
		if (badge === 'error') return 'border-red-500/70 bg-red-500/25 text-red-200';
		if (badge === 'msg') return 'border-blue-500/70 bg-blue-500/25 text-blue-200';
		if (badge === 'pos') return 'border-emerald-500/70 bg-emerald-500/25 text-emerald-200';
		if (badge === 'tele') return 'border-purple-500/70 bg-purple-500/25 text-purple-200';
		return 'border-gray-500/50 bg-gray-500/20 text-gray-200';
	}

	function packetField(event: StreamEvent, key: string): string {
		const packet = packetFromEvent(event);
		const value = packet?.[key];
		return value == null || value === '' ? '—' : String(value);
	}

	function isMessageEvent(event: StreamEvent): boolean {
		return packetFromEvent(event)?.type === 'msg';
	}

	function messageRoute(event: StreamEvent): {
		origin: string;
		relays: string[];
		destination: string;
	} {
		const packet = packetFromEvent(event);
		const source = splitSourcePath(packet?.src);
		return {
			origin: source.origin,
			relays: source.relays,
			destination: packet?.dst === '*' ? 'Broadcast' : (packet?.dst ?? 'unknown')
		};
	}

	function mdiForEvent(event: StreamEvent): string {
		const packet = packetFromEvent(event);
		if (packet?.type === 'msg') {
			const kind = messageKind(packet.msg).kind;
			if (kind === 'time') return mdiClockOutline;
			if (kind === 'ack') return mdiCheckCircleOutline;
			if (kind === 'reject') return mdiAlertCircleOutline;
			if (kind === 'config') return mdiCogOutline;
			return mdiEmailOutline;
		}
		if (packet?.type === 'pos') return mdiMapMarkerRadiusOutline;
		if (packet?.type === 'tele') return mdiBroadcast;
		return mdiAlertCircleOutline;
	}

	function iconTooltip(event: StreamEvent): string {
		const packet = packetFromEvent(event);
		if (!packet) return 'Parse error';
		if (packet.type === 'msg') {
			const kind = messageKind(packet.msg).kind;
			if (kind === 'ack') return 'ACK — message acknowledged';
			if (kind === 'reject') return 'REJ — message rejected';
			if (kind === 'time') return 'Network time sync';
			if (kind === 'config') return 'Config update';
			return 'Text message';
		}
		if (packet.type === 'pos') return 'Position report';
		if (packet.type === 'tele') return 'Telemetry';
		return `Raw packet · ${packet.src_type ?? 'unknown'}`;
	}

	function formatRtt(ms: number): string {
		if (ms < 0) return '';
		if (ms < 1000) return `${ms}ms`;
		const sec = Math.round(ms / 1000);
		if (sec < 60) return `${sec}s`;
		return `${Math.floor(sec / 60)}m ${sec % 60}s`;
	}

	function markRead(convId: string) {
		const now = new Date().toISOString();
		saveReadTimestamp(convId, now);
		readTimestamps = { ...readTimestamps, [convId]: now };
	}

	function selectChannel(channel: string) {
		chatTarget = { kind: 'channel', value: channel };
		markRead(conversationIdFor({ kind: 'channel', value: channel }));
	}

	function selectContact(contact: string) {
		chatTarget = { kind: 'contact', value: contact };
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
			const msgs = chatHistory[id];
			if (msgs === undefined || msgs.length > 0) {
				await deleteConversation(id);
			}

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

	function chatRecordSeqId(rec: ChatRecord): string | null {
		const match = (rec.msg ?? '').match(/\{(\d+)\s*$/);
		return match?.[1] ?? null;
	}

	function chatMdiIcon(rec: ChatRecord): string {
		const kind = messageKind(rec.msg).kind;
		if (kind === 'time') return mdiClockOutline;
		if (kind === 'ack') return mdiCheckCircleOutline;
		if (kind === 'reject') return mdiAlertCircleOutline;
		if (kind === 'config') return mdiCogOutline;
		return mdiEmailOutline;
	}

	function chatIconTooltip(rec: ChatRecord): string {
		const kind = messageKind(rec.msg).kind;
		if (kind === 'ack') return 'ACK — message acknowledged';
		if (kind === 'reject') return 'REJ — message rejected';
		if (kind === 'time') return 'Network time sync';
		if (kind === 'config') return 'Config update';
		return 'Text message';
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
		sending = true;
		sendError = null;
		try {
			await sendMessage(dst, text);
			draftMessage = '';
			// Keep spinner until echo from node or 5s timeout.
			sendEchoTimer = setTimeout(clearSendEcho, 5000);
		} catch (e) {
			if (e instanceof UnauthorizedError) return;
			if (e instanceof SendError && e.status === 429) {
				sendError = 'Duplicate ignored (sent <2s ago)';
			} else {
				sendError = e instanceof Error ? e.message : 'Send failed';
			}
			sending = false;
		}
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

		const existing = chatHistory[convId] ?? [];
		const key = chatRecordKey(rec);
		if (!existing.some((r) => chatRecordKey(r) === key)) {
			chatHistory = {
				...chatHistory,
				[convId]: [...existing, rec].sort((a, b) => a.received_at.localeCompare(b.received_at))
			};
		}

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
				<span data-testid="status-dot" class="md:hidden h-2 w-2 rounded-full {statusClass[connection]}"></span>
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
			<div data-testid="packet-counter" class="hidden md:block font-mono text-xs text-gray-400">{events.length} packets</div>
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
			<!-- Chat panel -->
			<div
				data-testid="chat-panel"
				class="flex h-[80vh] md:h-auto md:min-h-0 flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#212735] shadow-sm"
				style={isDesktop ? `flex: 0 0 ${chatWidthPct}%; min-width: 0` : ''}
			>
				<div
					class="flex h-9 shrink-0 items-center justify-between border-b border-gray-700/60 px-3"
				>
					<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">Chat</span>
					<span class="font-mono text-[11px] text-gray-500"
						>{displayChatRecords.length} messages</span
					>
				</div>

				<div
					class="grid min-h-0 flex-1"
					style={isDesktop ? `grid-template-columns: ${chatSidebarGridColumns(channelsCollapsed)}` : ''}
				>
					<!-- Destinations sidebar -->
					<aside class="flex min-h-0 flex-col border-r border-gray-700/60 bg-[#1c2230]">
						<div
							class="flex items-center border-b border-gray-700/60 py-2 text-[10px] font-semibold uppercase tracking-wider text-gray-500 {channelsCollapsed
								? 'justify-center px-1'
								: 'justify-between px-3'}"
						>
							<span class={channelsCollapsed ? 'sr-only' : ''}>Channels</span>
							<button
								type="button"
								class="flex h-6 w-6 shrink-0 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-gray-500 hover:text-gray-200"
								aria-label={channelsCollapsed ? 'Expand channels' : 'Collapse channels'}
								title={channelsCollapsed ? 'Expand channels' : 'Collapse channels'}
								onclick={toggleChannelsSidebar}
							>
								<MdiIcon path={channelsCollapsed ? mdiChevronRight : mdiChevronLeft} size={14} />
							</button>
						</div>
						<div class="min-h-0 flex-1 overflow-auto p-1.5">
							<!-- Broadcast (always shown) -->
							<button
								type="button"
								class="flex w-full items-center rounded px-2 py-1.5 text-left text-xs {channelsCollapsed
									? 'justify-center gap-0'
									: 'gap-2'} {chatTarget.kind ===
									'channel' && chatTarget.value === 'Broadcast'
									? 'bg-blue-500/15 text-blue-200'
									: 'text-gray-300 hover:bg-white/[0.04] hover:text-gray-100'}"
								aria-label="Broadcast channel"
								title="Broadcast channel"
								onclick={() => selectChannel('Broadcast')}
							>
								<MdiIcon path={mdiBroadcast} size={14} />
								{#if unreadIds.has('P_broadcast')}
									<span
										class="mr-0.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"
										aria-label="unread"
									></span>
								{/if}
								<span class={channelsCollapsed ? 'sr-only' : `truncate ${unreadIds.has('P_broadcast') ? 'font-semibold' : ''}`}
									>Broadcast</span
								>
							</button>

							<!-- Known groups from live traffic -->
							{#if resolvedChannels.known.length > 0}
								<div class="mt-1.5 space-y-0.5 border-t border-gray-700/60 pt-1.5">
									{#each resolvedChannels.known as { channel, group } (channel)}
										{@const knownCid = conversationIdFor({ kind: 'channel', value: channel })}
										<button
											type="button"
											title={groupTooltip(group)}
											class="flex w-full items-center rounded px-2 py-1.5 text-left text-xs {channelsCollapsed
												? 'justify-center gap-0'
												: 'gap-2'} {chatTarget.kind ===
												'channel' && chatTarget.value === channel
												? 'bg-blue-500/15 text-blue-200'
												: 'text-gray-300 hover:bg-white/[0.04] hover:text-gray-100'}"
											aria-label={group.note}
											onclick={() => selectChannel(channel)}
										>
											<span class="shrink-0 text-sm leading-none">{group.flag}</span>
											{#if unreadIds.has(knownCid)}
												<span
													class="mr-0.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"
													aria-label="unread"
												></span>
											{/if}
											<span class={channelsCollapsed ? 'sr-only' : `truncate ${unreadIds.has(knownCid) ? 'font-semibold' : ''}`}
												>{group.note}</span
											>
										</button>
									{/each}
								</div>
							{/if}

							<!-- Unknown group numbers -->
							{#if resolvedChannels.unknown.length > 0}
								<div class="mt-1.5 space-y-0.5 border-t border-gray-700/60 pt-1.5">
									{#each resolvedChannels.unknown as channel (channel)}
										{@const unknownCid = conversationIdFor({ kind: 'channel', value: channel })}
										<button
											type="button"
											class="flex w-full items-center rounded px-2 py-1.5 text-left text-xs {channelsCollapsed
												? 'justify-center gap-0'
												: 'gap-2'} {chatTarget.kind ===
												'channel' && chatTarget.value === channel
												? 'bg-blue-500/15 text-blue-200'
												: 'text-gray-300 hover:bg-white/[0.04] hover:text-gray-100'}"
											aria-label={channel}
											title={channel}
											onclick={() => selectChannel(channel)}
										>
											<MdiIcon path={mdiPound} size={14} />
											{#if unreadIds.has(unknownCid)}
												<span
													class="mr-0.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"
													aria-label="unread"
												></span>
											{/if}
											<span class={channelsCollapsed ? 'sr-only' : `truncate ${unreadIds.has(unknownCid) ? 'font-semibold' : ''}`}
												>{channel}</span
											>
										</button>
									{/each}
								</div>
							{/if}

							<!-- Direct Messages -->
							<div class="mt-2 border-t border-gray-700/60 pt-1.5">
								<div
									class="px-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-gray-600"
								>
									Direct Messages
								</div>
								{#if contacts.length === 0}
									<div class="px-2 py-1 text-[10px] text-gray-600">No DMs yet</div>
								{:else}
									<div class="space-y-0.5">
										{#each contacts as contact (contact)}
											{@const dmCid = conversationIdFor({ kind: 'contact', value: contact })}
											<button
												type="button"
												class="flex w-full items-center rounded px-2 py-1.5 text-left text-xs {channelsCollapsed
													? 'justify-center gap-0'
													: 'gap-2'} {chatTarget.kind ===
													'contact' && chatTarget.value === contact
													? 'bg-blue-500/15 text-blue-200'
													: 'text-gray-300 hover:bg-white/[0.04] hover:text-gray-100'}"
												aria-label={contact}
												title={contact}
												onclick={() => selectContact(contact)}
											>
												<MdiIcon path={mdiAccountOutline} size={14} />
												{#if unreadIds.has(dmCid)}
													<span
														class="mr-0.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"
														aria-label="unread"
													></span>
												{/if}
												<span class={channelsCollapsed ? 'sr-only' : `truncate ${unreadIds.has(dmCid) ? 'font-semibold' : ''}`}
													>{contact}</span
												>
											</button>
										{/each}
									</div>
								{/if}
								<button
									type="button"
									class="mt-1.5 flex w-full items-center justify-center gap-1.5 rounded border border-dashed border-blue-500/40 px-2 py-1.5 text-[11px] font-medium text-blue-400/70 hover:border-blue-400/70 hover:bg-blue-500/10 hover:text-blue-300"
									aria-label="New direct message"
									title="New direct message"
									onclick={openNewDm}
								>
									<MdiIcon path={mdiPlus} size={14} />
									<span>{chatSidebarNewDmLabel(channelsCollapsed)}</span>
								</button>
							</div>
						</div>
					</aside>

					<!-- Messages section -->
					<section class="flex min-h-0 flex-col">
						<div
							class="flex h-10 shrink-0 items-center justify-between border-b border-gray-700/60 px-3"
						>
							<div class="min-w-0 flex-1">
								{#if chatTarget.kind === 'channel' && chatTarget.value !== 'Broadcast'}
									{@const group = resolveGroup(chatTarget.value)}
									<div class="flex items-center gap-2">
										{#if group}
											<span class="text-xl leading-none">{group.flag}</span>
											<div>
												<div class="text-sm font-semibold text-white">{group.note}</div>
												<div class="text-[10px] text-gray-500">
													Group {group.group} · {group.prefix}
												</div>
											</div>
										{:else}
											<div>
												<div class="text-sm font-semibold text-white"># {chatTarget.value}</div>
												<div class="text-[10px] text-gray-500">channel · receive view</div>
											</div>
										{/if}
									</div>
								{:else}
									<div>
										<div class="text-sm font-semibold text-white">{chatTarget.value}</div>
										<div class="text-[10px] text-gray-500">
											{chatTarget.kind === 'channel' ? 'channel' : 'direct'} · receive view
										</div>
									</div>
								{/if}
							</div>
							<button
								type="button"
								class="ml-2 flex h-7 w-7 shrink-0 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
								aria-label={isBroadcastTarget ? 'Clear messages' : 'Delete conversation'}
								title={isBroadcastTarget ? 'Clear messages' : 'Delete conversation'}
								onclick={openDeleteConfirm}
							>
								<MdiIcon
									path={isBroadcastTarget ? mdiNotificationClearAll : mdiTrashCanOutline}
									size={14}
								/>
							</button>
						</div>

						<div bind:this={chatScrollEl} class="min-h-0 flex-1 overflow-auto p-2">
							{#if displayChatRecords.length === 0}
								<div class="flex h-full items-center justify-center text-sm text-gray-500">
									No messages in this chat
								</div>
							{:else}
								<div class="space-y-2">
									{#each displayChatRecords as rec (chatRecordKey(rec))}
										{@const srcPath = splitSourcePath(rec.src)}
										{@const isSent = srcPath.origin === stationCallsign}
										{@const seqId = isSent ? chatRecordSeqId(rec) : null}
										{@const ackEntries = seqId ? (ackIndex.acked.get(seqId) ?? []) : []}
										{@const rejEntries = seqId ? (ackIndex.rejected.get(seqId) ?? []) : []}
										{@const gatewayAck = ackEntries.find((a) => a.ackSource === 'gateway') ?? null}
										{@const loraAck = ackEntries.find((a) => a.ackSource === 'lora') ?? null}
										{@const bestAck = gatewayAck ?? loraAck}
										{@const rttMs = bestAck
											? new Date(bestAck.receivedAt).getTime() - new Date(rec.received_at).getTime()
											: -1}
										<div
											class="rounded border border-gray-700/60 bg-[#1c2230] p-2 md:p-3 transition-colors hover:border-gray-600/60"
										>
											<div class="flex items-center justify-between gap-2 md:gap-3">
												<div class="flex min-w-0 items-center gap-2">
													<span
														class="flex h-7 w-7 shrink-0 items-center justify-center rounded border border-blue-500/70 bg-blue-500/25 text-blue-200"
														title={chatIconTooltip(rec)}
													>
														<MdiIcon path={chatMdiIcon(rec)} size={16} />
													</span>
													<div class="min-w-0">
														<div class="truncate text-xs md:text-sm font-semibold text-white">
															{srcPath.origin}
														</div>
														<div class="truncate text-[10px] text-gray-500">
															{messageKind(rec.msg).label.toUpperCase()}
														</div>
													</div>
												</div>
												<div class="flex items-center gap-1.5">
													<div class="font-mono text-[10px] md:text-[11px] text-gray-500">
														{formatTime(rec.received_at)}
													</div>
													{#if isSent}
														{#if chatTarget.kind === 'contact'}
															{#if seqId && rejEntries.length > 0}
																<span class="text-[13px] font-bold text-red-400" title="Rejected"
																	>✗</span
																>
															{:else if seqId && gatewayAck}
																<span
																	class="flex items-center gap-1"
																	title="Gateway delivered in {formatRtt(rttMs)}"
																>
																	<span class="text-[13px] font-bold text-green-400">☁️✓</span>
																	<span class="font-mono text-[10px] text-green-600/80"
																		>{formatRtt(rttMs)}</span
																	>
																</span>
															{:else if seqId && loraAck}
																<span
																	class="flex items-center gap-1"
																	title="Acknowledged in {formatRtt(rttMs)}"
																>
																	<span class="text-[13px] font-bold text-green-400">✓✓</span>
																	<span class="font-mono text-[10px] text-green-600/80"
																		>{formatRtt(rttMs)}</span
																	>
																</span>
															{:else}
																<span
																	class="text-[13px] text-gray-600"
																	title="Sent, waiting for ACK">✓</span
																>
															{/if}
														{:else if seqId && gatewayAck}
															<span
																class="text-[13px] text-blue-400"
																title="Delivered via gateway ({gatewayAck.source})">☁️</span
															>
														{/if}
													{/if}
												</div>
											</div>
											<div
												class="mt-2 rounded border border-gray-700/60 bg-[#111827] px-3 py-2 text-[11px] md:text-sm leading-relaxed text-gray-100"
											>
												{cleanMessage(rec.msg) || rec.msg}
											</div>
											{#if isSent && chatTarget.kind === 'contact' && seqId && bestAck}
												<div
													class="mt-1 flex items-center gap-2 font-mono text-[10px] text-green-600/70"
												>
													<span>ack</span>
													{#if bestAck.via.length > 0}
														<span>via {bestAck.via.join(' → ')}</span>
													{/if}
													{#if bestAck.rssi != null}
														<span class="flex items-center gap-0.5">
															<MdiIcon path={mdiSignalVariant} size={10} />
															{bestAck.rssi} dBm
														</span>
													{/if}
													{#if bestAck.snr != null}
														<span class="flex items-center gap-0.5">
															<MdiIcon path={mdiTune} size={10} />
															SNR {bestAck.snr}
														</span>
													{/if}
												</div>
											{/if}
											<div
												class="mt-1.5 flex items-center justify-between gap-2 font-mono text-[10px] md:text-[11px]"
											>
												<div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-gray-500">
													{#if srcPath.relays.length > 0}
														<span>via {srcPath.relays.join(' → ')}</span>
													{/if}
													{#if rec.rssi != null}
														<span class="flex items-center gap-0.5" title="RSSI — signal strength">
															<MdiIcon path={mdiSignalVariant} size={11} />
															{rec.rssi} dBm
														</span>
													{/if}
													{#if rec.snr != null}
														<span
															class="flex items-center gap-0.5"
															title="SNR — signal to noise ratio"
														>
															<MdiIcon path={mdiTune} size={11} />
															SNR {rec.snr}
														</span>
													{/if}
												</div>
												<button
													type="button"
													class="text-gray-600 hover:text-blue-300"
													onclick={() => (rawChatRecord = rec)}
												>
													<MdiIcon path={mdiCodeJson} size={16} />
												</button>
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</div>

						<div class="shrink-0 border-t border-gray-700/60 p-2">
							{#if sendError}
								<div
									class="mb-1.5 rounded px-2 py-1 text-xs {sendError.startsWith('Duplicate')
										? 'bg-amber-900/40 text-amber-300'
										: 'bg-red-900/40 text-red-300'}"
								>
									{sendError}
								</div>
							{/if}
							<div class="flex gap-2">
								<div class="relative min-w-0 flex-1">
									<input
										class="h-10 w-full rounded border border-gray-700/60 bg-[#111827] px-3 text-sm text-gray-200 outline-none placeholder:text-gray-500 focus:border-blue-500/60"
										bind:this={messageInputEl}
										bind:value={draftMessage}
										placeholder="Type a message…"
										maxlength={149}
										disabled={sending || txDisabled}
										onkeydown={(e) => {
											if (e.key === 'Enter' && !e.shiftKey) {
												e.preventDefault();
												handleSend();
											}
										}}
									/>
									<span
										class="pointer-events-none absolute bottom-1.5 right-2 text-[10px] tabular-nums {draftMessage.length >=
										140
											? 'text-red-400'
											: 'text-gray-600'}"
									>
										{draftMessage.length}/149
									</span>
								</div>
								<div class="flex flex-col items-center gap-1">
									<button
										class="h-10 rounded border border-gray-700/60 bg-blue-600/80 px-4 text-sm text-white hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-50"
										disabled={
											!getSendComposerState({ draftMessage, sending, txDisabled }).canSend
										}
										onclick={handleSend}
									>
										{getSendComposerState({ draftMessage, sending, txDisabled }).label}
									</button>
									{#if getSendComposerState({ draftMessage, sending, txDisabled }).hint}
										<span class="text-[10px] font-semibold uppercase tracking-wide text-amber-300">
											{getSendComposerState({ draftMessage, sending, txDisabled }).hint}
										</span>
									{/if}
								</div>
							</div>
						</div>
					</section>
				</div>
			</div>

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

		<!-- UDP stream panel -->
		<div
			data-testid="udp-panel"
			class="flex shrink-0 flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#212735] shadow-sm h-[80vh] md:h-auto md:min-h-[160px]"
			style={isDesktop ? `height: ${streamHeightPx}px` : ''}
		>
			<div class="flex h-9 shrink-0 items-center justify-between border-b border-gray-700/60 px-3">
				<div class="flex items-center gap-2">
					<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400"
						>UDP stream</span
					>
					<span class="rounded bg-gray-700/40 px-1.5 py-0.5 font-mono text-[10px] text-gray-500"
						>/api/events</span
					>
				</div>
				<div class="flex items-center gap-2">
					<div class="relative">
						<input
							type="text"
							bind:value={streamFilter}
							placeholder="Filter…"
							class="h-6 w-36 rounded border border-gray-700/60 bg-[#1c2230] py-0 pl-2 pr-6 text-[11px] text-gray-200 placeholder:text-gray-600 focus:border-blue-500/60 focus:outline-none"
						/>
						{#if streamFilter}
							<button
								type="button"
								class="absolute right-1 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-200"
								aria-label="Clear filter"
								onclick={() => (streamFilter = '')}
							>
								<MdiIcon path={mdiClose} size={12} />
							</button>
						{/if}
					</div>
					<span class="font-mono text-[11px] text-gray-500">
						{filteredEvents.length}{streamFilter ? `/${events.length}` : ''}
					</span>
				</div>
			</div>

			<div class="min-h-0 flex-1 overflow-auto">
				{#if events.length === 0}
					<div class="flex h-full items-center justify-center text-sm text-gray-500">
						Waiting for UDP packets
					</div>
				{:else if filteredEvents.length === 0}
					<div class="flex h-full items-center justify-center text-sm text-gray-500">
						No results for "{streamFilter}"
					</div>
				{:else}
					<div class="divide-y divide-gray-700/50">
						{#each filteredEvents as event (event.id)}
							<div
								role="button"
								tabindex="0"
								class="grid w-full grid-cols-[3rem_1fr] md:grid-cols-[4.5rem_1fr_2rem] gap-2 md:gap-3 px-3 py-2 text-left hover:bg-white/[0.03] {selectedEvent?.id ===
								event.id
									? 'bg-white/[0.04]'
									: ''}"
								onclick={() => (isDesktop ? (selectedEvent = event) : (rawEvent = event))}
								onkeydown={(keyEvent) => {
									if (keyEvent.key === 'Enter' || keyEvent.key === ' ')
										isDesktop ? (selectedEvent = event) : (rawEvent = event);
								}}
							>
								<div class="font-mono text-[10px] md:text-[11px] text-gray-500">
									{formatTime(event.receivedAt)}
								</div>
								<div class="min-w-0">
									{#if isMessageEvent(event)}
										{@const route = messageRoute(event)}
										{@const packet = packetFromEvent(event)}
										{@const text =
											stripMessagePrefix(packet?.msg ?? '') || packetField(event, 'msg')}
										<div class="flex min-w-0 items-center gap-1.5">
											<span
												class="flex h-6 w-6 shrink-0 items-center justify-center rounded border {packetTone(
													event
												)}"
												title={iconTooltip(event)}
											>
												<MdiIcon path={mdiForEvent(event)} size={17} />
											</span>
											<span class="shrink-0 text-xs md:text-sm font-bold text-white">{route.origin}</span>
											{#if route.relays.length > 0}
												<span class="hidden md:inline shrink-0 text-[11px] text-gray-400"
													>(via {route.relays.join(', ')})</span
												>
											{/if}
											<span class="hidden md:inline shrink-0 text-gray-500"
												><MdiIcon path={mdiArrowRight} size={13} /></span
											>
											<span class="hidden md:inline shrink-0 text-sm text-gray-200">{route.destination}</span>
											<span class="mx-0.5 shrink-0 text-gray-400">·</span>
											<span class="min-w-0 truncate italic text-xs md:text-sm text-white">"{text}"</span>
											{#if packet?.rssi != null || packet?.snr != null}
												<span class="ml-auto flex shrink-0 items-center gap-2 pl-2">
													{#if packet?.rssi != null}
														<span
															class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
															title="RSSI — received signal strength (dBm)"
														>
															<span class="text-gray-300"
																><MdiIcon path={mdiSignalVariant} size={14} /></span
															>
															{packet.rssi}
														</span>
													{/if}
													{#if packet?.snr != null}
														<span
															class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
															title="SNR — signal-to-noise ratio (dB)"
														>
															<span class="text-gray-300"><MdiIcon path={mdiTune} size={14} /></span
															>
															{packet.snr}
														</span>
													{/if}
												</span>
											{/if}
										</div>
									{:else if packetFromEvent(event)?.type === 'tele'}
										{@const packet = packetFromEvent(event)}
										{@const source = splitSourcePath(packet?.src)}
										<div class="flex min-w-0 items-center gap-1.5">
											<span
												class="flex h-6 w-6 shrink-0 items-center justify-center rounded border {packetTone(
													event
												)}"
												title={iconTooltip(event)}
											>
												<MdiIcon path={mdiForEvent(event)} size={17} />
											</span>
											<span class="shrink-0 text-xs md:text-sm font-bold text-white">{source.origin}</span>
											{#if source.relays.length > 0}
												<span class="hidden md:inline shrink-0 text-[11px] text-gray-400"
													>(via {source.relays.join(', ')})</span
												>
											{/if}
											<span class="mx-0.5 shrink-0 text-gray-400">·</span>
											<span class="flex min-w-0 items-center gap-2">
												{#if packet?.batt != null}
													<span
														class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
													>
														<span class="text-gray-500"
															><MdiIcon path={mdiBattery} size={12} /></span
														>
														{packet.batt}%
													</span>
												{/if}
												{#if packet?.temp1 != null}
													<span
														class="hidden md:flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
													>
														<span class="text-gray-500"
															><MdiIcon path={mdiThermometer} size={12} /></span
														>
														{packet.temp1}°C
													</span>
												{/if}
												{#if packet?.hum != null}
													<span
														class="hidden md:flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
													>
														<span class="text-gray-500"
															><MdiIcon path={mdiWaterPercent} size={12} /></span
														>
														{packet.hum}%
													</span>
												{/if}
												{#if packet?.qnh != null || packet?.qfe != null}
													<span
														class="hidden md:flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
													>
														<span class="text-gray-500"><MdiIcon path={mdiGauge} size={12} /></span>
														{packet.qnh ?? packet.qfe} hPa
													</span>
												{/if}
											</span>
											{#if packet?.rssi != null || packet?.snr != null}
												<span class="ml-auto flex shrink-0 items-center gap-2 pl-2">
													{#if packet?.rssi != null}
														<span
															class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
															title="RSSI — received signal strength (dBm)"
														>
															<span class="text-gray-300"
																><MdiIcon path={mdiSignalVariant} size={14} /></span
															>
															{packet.rssi}
														</span>
													{/if}
													{#if packet?.snr != null}
														<span
															class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
															title="SNR — signal-to-noise ratio (dB)"
														>
															<span class="text-gray-300"><MdiIcon path={mdiTune} size={14} /></span
															>
															{packet.snr}
														</span>
													{/if}
												</span>
											{/if}
										</div>
									{:else if packetFromEvent(event)?.type === 'pos'}
										{@const packet = packetFromEvent(event)}
										{@const source = splitSourcePath(packet?.src)}
										{@const hardware = hardwareHumanName(packet?.hw_id)}
										<div class="flex min-w-0 items-center gap-1.5">
											<span
												class="flex h-6 w-6 shrink-0 items-center justify-center rounded border {packetTone(
													event
												)}"
												title={iconTooltip(event)}
											>
												<MdiIcon path={mdiForEvent(event)} size={17} />
											</span>
											<span class="shrink-0 text-xs md:text-sm font-bold text-white">{source.origin}</span>
											{#if source.relays.length > 0}
												<span class="hidden md:inline shrink-0 text-[11px] text-gray-400"
													>(via {source.relays.join(', ')})</span
												>
											{/if}
											<span class="mx-0.5 shrink-0 text-gray-400">·</span>
											<span class="flex min-w-0 items-center gap-2">
												{#if packet?.lat != null && packet?.long != null}
													<span
														class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
													>
														<span class="text-gray-500"
															><MdiIcon path={mdiMapMarkerRadiusOutline} size={12} /></span
														>
														{packet.lat.toFixed(4)}, {packet.long.toFixed(4)}
													</span>
												{/if}
												{#if packet?.alt != null}
													<span class="hidden md:inline shrink-0 font-mono text-[11px] text-gray-400"
														>{packet.alt} m</span
													>
												{/if}
												{#if packet?.batt != null}
													<span
														class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
													>
														<span class="text-gray-500"
															><MdiIcon path={mdiBattery} size={12} /></span
														>
														{packet.batt}%
													</span>
												{/if}
												{#if hardware}
													<span
														class="hidden md:flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
														title="Hardware ID {packet?.hw_id}"
													>
														<span class="text-gray-500"><MdiIcon path={mdiChip} size={12} /></span>
														{hardware}
													</span>
												{/if}
											</span>
											{#if packet?.rssi != null || packet?.snr != null}
												<span class="ml-auto flex shrink-0 items-center gap-2 pl-2">
													{#if packet?.rssi != null}
														<span
															class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
															title="RSSI — received signal strength (dBm)"
														>
															<span class="text-gray-300"
																><MdiIcon path={mdiSignalVariant} size={14} /></span
															>
															{packet.rssi}
														</span>
													{/if}
													{#if packet?.snr != null}
														<span
															class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
															title="SNR — signal-to-noise ratio (dB)"
														>
															<span class="text-gray-300"><MdiIcon path={mdiTune} size={14} /></span
															>
															{packet.snr}
														</span>
													{/if}
												</span>
											{/if}
										</div>
									{:else}
										{@const packet = packetFromEvent(event)}
										{@const source = splitSourcePath(packet?.src)}
										<div class="flex min-w-0 items-center gap-1.5">
											<span
												class="flex h-6 w-6 shrink-0 items-center justify-center rounded border {packetTone(
													event
												)}"
												title={iconTooltip(event)}
											>
												<MdiIcon path={mdiForEvent(event)} size={17} />
											</span>
											{#if packet}
												<span class="shrink-0 text-xs md:text-sm font-bold text-white">{source.origin}</span>
												{#if source.relays.length > 0}
													<span class="hidden md:inline shrink-0 text-[11px] text-gray-400"
														>(via {source.relays.join(', ')})</span
													>
												{/if}
												<span class="mx-0.5 shrink-0 text-gray-400">·</span>
												<span class="flex min-w-0 items-center gap-2">
													{#if packet.lat != null && packet.long != null}
														<span
															class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
														>
															<span class="text-gray-500"
																><MdiIcon path={mdiMapMarkerRadiusOutline} size={12} /></span
															>
															{packet.lat.toFixed(4)}, {packet.long.toFixed(4)}
														</span>
													{/if}
													{#if packet.alt != null}
														<span class="hidden md:inline shrink-0 font-mono text-[11px] text-gray-400"
															>{packet.alt} m</span
														>
													{/if}
													{#if packet.batt != null}
														<span
															class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200"
														>
															<span class="text-gray-500"
																><MdiIcon path={mdiBattery} size={12} /></span
															>
															{packet.batt}%
														</span>
													{/if}
													{#if packet.lat == null && packet.batt == null}
														<span
															class="rounded border px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase {packetTone(
																event
															)}">{packetBadge(event)}</span
														>
													{/if}
												</span>
											{:else}
												<span class="min-w-0 truncate text-sm text-gray-300"
													>{eventSummary(event)}</span
												>
											{/if}
											{#if packet?.rssi != null || packet?.snr != null}
												<span class="ml-auto flex shrink-0 items-center gap-2 pl-2">
													{#if packet?.rssi != null}
														<span
															class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
															title="RSSI — received signal strength (dBm)"
														>
															<span class="text-gray-300"
																><MdiIcon path={mdiSignalVariant} size={14} /></span
															>
															{packet.rssi}
														</span>
													{/if}
													{#if packet?.snr != null}
														<span
															class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
															title="SNR — signal-to-noise ratio (dB)"
														>
															<span class="text-gray-300"><MdiIcon path={mdiTune} size={14} /></span
															>
															{packet.snr}
														</span>
													{/if}
												</span>
											{/if}
										</div>
									{/if}
								</div>
								<button
									type="button"
									class="hidden md:flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 bg-[#1c2230] text-gray-400 hover:border-blue-500/50 hover:text-blue-300"
									title="Show JSON"
									aria-label="Show JSON"
									onclick={(clickEvent) => {
										clickEvent.stopPropagation();
										rawEvent = event;
									}}
								>
									<MdiIcon path={mdiCodeJson} size={14} />
								</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
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
