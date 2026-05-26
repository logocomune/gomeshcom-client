<script lang="ts">
	import { tick } from 'svelte';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { chatRecordKey } from '$lib/api/chat';
	import type { AckIndex } from '$lib/api/acks';
	import { ackEntriesForRecord, rejectEntriesForRecord } from '$lib/api/acks';
	import { cleanMessage, splitSourcePath } from '$lib/api/events';
	import { resolveGroup } from '$lib/api/groups';
	import { getSendComposerState } from '$lib/ui/send';
	import { formatRtt, formatTime } from '$lib/ui/format';
	import { chatIconTooltip, chatMdiIcon, chatRecordSeqId } from '$lib/ui/chat-records';
	import {
		mdiArrowLeft,
		mdiNotificationClearAll,
		mdiSignalVariant,
		mdiTrashCanOutline,
		mdiTune
	} from '@mdi/js';
	import { chatState } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';

	interface Props {
		ackIndex: AckIndex;
		handleSend: () => void | Promise<void>;
		showBack?: boolean;
		onBack?: () => void;
	}

	let { ackIndex, handleSend, showBack = false, onBack }: Props = $props();

	let chatScrollEl = $state<HTMLDivElement | null>(null);
	let messageInputEl = $state<HTMLInputElement | null>(null);

	$effect(() => {
		const _deps = [chatState.chatTarget, chatState.displayChatRecords];
		void _deps;
		tick().then(() => chatScrollEl?.scrollTo({ top: chatScrollEl.scrollHeight }));
	});

	$effect(() => {
		void chatState.chatTarget;
		tick().then(() => setTimeout(() => messageInputEl?.focus(), 0));
	});

	function sendOnEnter(event: KeyboardEvent) {
		if (event.key === 'Enter' && !event.shiftKey) {
			event.preventDefault();
			void handleSend();
		}
	}

	let chatTarget = $derived(chatState.chatTarget);
	let isBroadcastTarget = $derived(chatState.isBroadcastTarget);
	let displayChatRecords = $derived(chatState.displayChatRecords);
	let stationCallsign = $derived(connectionState.stationCallsign);
	let txDisabled = $derived(connectionState.txDisabled);
	let draftMessage = $derived(chatState.draftMessage);
	let sending = $derived(chatState.sending);
	let sendError = $derived(chatState.sendError);
	let chatFilter = $derived(chatState.chatFilter);
</script>

<div data-testid="chat-panel" class="flex min-h-0 flex-1 flex-col">
	<!-- Thread header -->
	<div class="flex h-10 shrink-0 items-center justify-between border-b border-gray-700/60 px-3">
		<div class="flex min-w-0 flex-1 items-center gap-2">
			{#if showBack}
				<button
					type="button"
					class="mr-1 flex h-7 w-7 shrink-0 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-gray-500 hover:text-gray-200"
					aria-label="Back to conversations"
					onclick={onBack}
				>
					<MdiIcon path={mdiArrowLeft} size={16} />
				</button>
			{/if}
			{#if chatTarget.kind === 'channel' && chatTarget.value !== 'Broadcast'}
				{@const group = resolveGroup(chatTarget.value)}
				{#if group}
					<span class="text-xl leading-none">{group.flag}</span>
					<div>
						<div class="text-sm font-semibold text-white">{group.note}</div>
						<div class="text-[10px] text-gray-500">Group {group.group} · {group.prefix}</div>
					</div>
				{:else}
					<div>
						<div class="text-sm font-semibold text-white"># {chatTarget.value}</div>
						<div class="text-[10px] text-gray-500">channel</div>
					</div>
				{/if}
			{:else}
				<div>
					<div class="text-sm font-semibold text-white">{chatTarget.value}</div>
					<div class="text-[10px] text-gray-500">
						{chatTarget.kind === 'channel' ? 'channel' : 'direct'}
					</div>
				</div>
			{/if}
		</div>
		<div class="flex items-center gap-1.5">
			<label>
				<span class="sr-only">Filter messages</span>
				<input
					type="search"
					class="h-7 w-28 rounded border border-gray-700/60 bg-[#111827] px-2 text-xs text-gray-200 outline-none placeholder:text-gray-500 focus:border-blue-500/60 md:w-44"
					bind:value={chatState.chatFilter}
					placeholder="Filter"
				/>
			</label>
			<button
				type="button"
				class="flex h-7 w-7 shrink-0 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
				aria-label={isBroadcastTarget ? 'Clear messages' : 'Delete conversation'}
				title={isBroadcastTarget ? 'Clear messages' : 'Delete conversation'}
				onclick={() => {
					chatState.deleteError = null;
					chatState.deleteConfirmOpen = true;
				}}
			>
				<MdiIcon
					path={isBroadcastTarget ? mdiNotificationClearAll : mdiTrashCanOutline}
					size={14}
				/>
			</button>
		</div>
	</div>

	<!-- Message list -->
	<div bind:this={chatScrollEl} class="min-h-0 flex-1 overflow-auto p-2">
		{#if displayChatRecords.length === 0}
			<div class="flex h-full items-center justify-center text-sm text-gray-500">
				{chatFilter.trim() === '' ? 'No messages in this chat' : 'No matching messages'}
			</div>
		{:else}
			<div class="space-y-2">
				{#each displayChatRecords as record (chatRecordKey(record))}
					{@const sourcePath = splitSourcePath(record.src)}
					{@const isSent = sourcePath.origin === stationCallsign}
					{@const sequenceId = isSent ? chatRecordSeqId(record) : null}
					{@const ackEntries = ackEntriesForRecord(ackIndex, sequenceId, record, stationCallsign)}
					{@const rejectEntries = rejectEntriesForRecord(
						ackIndex,
						sequenceId,
						record,
						stationCallsign
					)}
					{@const gatewayAck = ackEntries.find((entry) => entry.ackSource === 'gateway') ?? null}
					{@const loraAck = ackEntries.find((entry) => entry.ackSource === 'lora') ?? null}
					{@const bestAck = gatewayAck ?? loraAck}
					{@const rttMs = bestAck
						? new Date(bestAck.receivedAt).getTime() - new Date(record.received_at).getTime()
						: -1}
					<div
						class="rounded border border-gray-700/60 bg-[#1c2230] p-2 md:p-3 transition-colors hover:border-gray-600/60"
						title={record.delivery_status === 'pending'
							? 'Pending'
							: record.delivery_status === 'failed'
								? 'Failed'
								: undefined}
					>
						<div class="flex items-center justify-between gap-2 md:gap-3">
							<div class="flex min-w-0 items-center gap-2">
								<div class="min-w-0">
									<div class="truncate text-xs md:text-sm font-semibold text-white">
										{sourcePath.origin}
									</div>
								</div>
							</div>
							<div class="flex items-center gap-1.5">
								<div class="font-mono text-[10px] md:text-[11px] text-gray-500">
									{formatTime(record.received_at)}
								</div>
								{#if record.delivery_status === 'pending'}
									<span
										class="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-blue-400/30 border-t-blue-300"
										style="width: 0.875rem; height: 0.875rem; border: 2px solid rgba(96, 165, 250, 0.3); border-top-color: rgb(147, 197, 253);"
										title="Pending"
									></span>
								{:else if record.delivery_status === 'failed'}
									<span class="text-[13px] font-bold text-red-400" title="Failed">✗</span>
								{:else if isSent}
									{#if chatTarget.kind === 'contact'}
										{#if sequenceId && rejectEntries.length > 0}
											<span class="text-[13px] font-bold text-red-400" title="Rejected">✗</span>
										{:else if sequenceId && gatewayAck}
											<span
												class="flex items-center gap-1"
												title="Gateway delivered in {formatRtt(rttMs)}"
											>
												<span class="text-[13px] font-bold text-green-400">☁️✓</span>
												<span class="font-mono text-[10px] text-green-600/80"
													>{formatRtt(rttMs)}</span
												>
											</span>
										{:else if sequenceId && loraAck}
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
											<span class="text-[13px] text-gray-600" title="Sent, waiting for ACK">✓</span>
										{/if}
									{:else if sequenceId && gatewayAck}
										<span
											class="text-[13px] text-green-400"
											title="Delivered via gateway ({gatewayAck.source})">☁️</span
										>
									{:else}
										<span class="text-[13px] text-green-400" title="Node echo observed">☁️</span>
									{/if}
								{/if}
							</div>
						</div>
						<div
							class="mt-2 rounded border border-gray-700/60 bg-[#111827] px-3 py-2 text-[11px] md:text-sm leading-relaxed text-gray-100"
						>
							{cleanMessage(record.msg) || record.msg}
						</div>
						{#if isSent && chatTarget.kind === 'contact' && sequenceId && ackEntries.length > 0}
							<div class="mt-1 space-y-0.5 font-mono text-[10px] text-green-600/70">
								{#each ackEntries as ackEntry, ackEntryIndex (ackEntry.ackSource + '-' + ackEntry.receivedAt + '-' + ackEntryIndex)}
									<div class="flex flex-wrap items-center gap-x-2 gap-y-0.5">
										<span>ack {ackEntry.ackSource}</span>
										<span
											>{formatRtt(
												new Date(ackEntry.receivedAt).getTime() -
													new Date(record.received_at).getTime()
											)}</span
										>
										{#if ackEntry.via.length > 0}
											<span>via {ackEntry.via.join(' → ')}</span>
										{/if}
										{#if ackEntry.rssi != null}
											<span class="flex items-center gap-0.5">
												<MdiIcon path={mdiSignalVariant} size={10} />
												{ackEntry.rssi} dBm
											</span>
										{/if}
										{#if ackEntry.snr != null}
											<span class="flex items-center gap-0.5">
												<MdiIcon path={mdiTune} size={10} />
												SNR {ackEntry.snr}
											</span>
										{/if}
									</div>
								{/each}
							</div>
						{/if}
						{#if sourcePath.relays.length > 0 || (!isSent && (record.rssi != null || record.snr != null))}
							<div
								class="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 font-mono text-[10px] text-gray-500 md:text-[11px]"
							>
								{#if sourcePath.relays.length > 0}
									<span>via {sourcePath.relays.join(' → ')}</span>
								{/if}
								{#if !isSent && record.rssi != null}
									<span class="flex items-center gap-0.5" title="RSSI — signal strength">
										<MdiIcon path={mdiSignalVariant} size={11} />
										{record.rssi} dBm
									</span>
								{/if}
								{#if !isSent && record.snr != null}
									<span class="flex items-center gap-0.5" title="SNR — signal to noise ratio">
										<MdiIcon path={mdiTune} size={11} />
										SNR {record.snr}
									</span>
								{/if}
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<!-- Composer -->
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
					bind:value={chatState.draftMessage}
					placeholder="Type a message…"
					maxlength={149}
					disabled={sending || txDisabled}
					onkeydown={sendOnEnter}
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
					disabled={!getSendComposerState({ draftMessage, sending, txDisabled }).canSend}
					onclick={() => void handleSend()}
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
</div>
