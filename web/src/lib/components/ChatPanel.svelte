<script lang="ts">
	import { tick } from 'svelte';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { conversationIdFor, chatRecordKey } from '$lib/api/chat';
	import type { ChatTarget } from '$lib/api/chat';
	import type { AckIndex } from '$lib/api/acks';
	import { cleanMessage, messageKind, splitSourcePath } from '$lib/api/events';
	import type { ChatRecord } from '$lib/api/types';
	import { groupTooltip, partitionChannels, resolveGroup } from '$lib/api/groups';
	import { getSendComposerState } from '$lib/ui/send';
	import { chatSidebarGridStyle, chatSidebarNewDmLabel } from '$lib/ui/chat-layout';
	import { formatRtt, formatTime } from '$lib/ui/format';
	import { chatIconTooltip, chatMdiIcon, chatRecordSeqId } from '$lib/ui/chat-records';
	import {
		mdiAccountOutline,
		mdiBroadcast,
		mdiChevronLeft,
		mdiChevronRight,
		mdiCodeJson,
		mdiNotificationClearAll,
		mdiPlus,
		mdiPound,
		mdiSignalVariant,
		mdiTrashCanOutline,
		mdiTune
	} from '@mdi/js';

	let {
		ackIndex,
		channelsCollapsed,
		chatFilter = $bindable(),
		chatTarget,
		chatWidthPct,
		contacts,
		displayChatRecords,
		draftMessage = $bindable(),
		isBroadcastTarget,
		isDesktop,
		resolvedChannels,
		sendError,
		sending,
		stationCallsign,
		txDisabled,
		unreadIds,
		handleSend,
		onDelete,
		onNewDm,
		onSelectChannel,
		onSelectContact,
		onShowRawRecord,
		onToggleChannels
	}: {
		ackIndex: AckIndex;
		channelsCollapsed: boolean;
		chatFilter: string;
		chatTarget: ChatTarget;
		chatWidthPct: number;
		contacts: string[];
		displayChatRecords: ChatRecord[];
		draftMessage: string;
		isBroadcastTarget: boolean;
		isDesktop: boolean;
		resolvedChannels: ReturnType<typeof partitionChannels>;
		sendError: string | null;
		sending: boolean;
		stationCallsign: string;
		txDisabled: boolean;
		unreadIds: Set<string>;
		handleSend: () => void | Promise<void>;
		onDelete: () => void;
		onNewDm: () => void;
		onSelectChannel: (channel: string) => void;
		onSelectContact: (contact: string) => void;
		onShowRawRecord: (record: ChatRecord) => void;
		onToggleChannels: () => void;
	} = $props();

	let chatScrollEl = $state<HTMLDivElement | null>(null);
	let messageInputEl = $state<HTMLInputElement | null>(null);

	$effect(() => {
		const _deps = [chatTarget, displayChatRecords];
		void _deps;
		tick().then(() => chatScrollEl?.scrollTo({ top: chatScrollEl.scrollHeight }));
	});

	$effect(() => {
		void chatTarget;
		tick().then(() => setTimeout(() => messageInputEl?.focus(), 0));
	});

	function sendOnEnter(event: KeyboardEvent) {
		if (event.key === 'Enter' && !event.shiftKey) {
			event.preventDefault();
			void handleSend();
		}
	}
</script>

<div
	data-testid="chat-panel"
	class="flex h-[80vh] md:h-auto md:min-h-0 flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#212735] shadow-sm"
	style={isDesktop ? `flex: 0 0 ${chatWidthPct}%; min-width: 0` : ''}
>
	<div class="flex h-9 shrink-0 items-center justify-between border-b border-gray-700/60 px-3">
		<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">Chat</span>
		<span class="font-mono text-[11px] text-gray-500">{displayChatRecords.length} messages</span>
	</div>

	<div class="grid min-h-0 flex-1" style={chatSidebarGridStyle(channelsCollapsed, isDesktop)}>
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
					onclick={onToggleChannels}
				>
					<MdiIcon path={channelsCollapsed ? mdiChevronRight : mdiChevronLeft} size={14} />
				</button>
			</div>
			<div class="min-h-0 flex-1 overflow-auto p-1.5">
				<button
					type="button"
					class="flex w-full items-center rounded px-2 py-1.5 text-left text-xs {channelsCollapsed
						? 'justify-center gap-0'
						: 'gap-2'} {chatTarget.kind === 'channel' && chatTarget.value === 'Broadcast'
						? 'bg-blue-500/15 text-blue-200'
						: 'text-gray-300 hover:bg-white/[0.04] hover:text-gray-100'}"
					aria-label="Broadcast channel"
					title="Broadcast channel"
					onclick={() => onSelectChannel('Broadcast')}
				>
					<MdiIcon path={mdiBroadcast} size={14} />
					{#if unreadIds.has('P_broadcast')}
						<span
							class="mr-0.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"
							aria-label="unread"
						></span>
					{/if}
					<span
						class={channelsCollapsed
							? 'sr-only'
							: `truncate ${unreadIds.has('P_broadcast') ? 'font-semibold' : ''}`}>Broadcast</span
					>
				</button>

				{#if resolvedChannels.known.length > 0}
					<div class="mt-1.5 space-y-0.5 border-t border-gray-700/60 pt-1.5">
						{#each resolvedChannels.known as { channel, group } (channel)}
							{@const knownCid = conversationIdFor({ kind: 'channel', value: channel })}
							<button
								type="button"
								title={groupTooltip(group)}
								class="flex w-full items-center rounded px-2 py-1.5 text-left text-xs {channelsCollapsed
									? 'justify-center gap-0'
									: 'gap-2'} {chatTarget.kind === 'channel' && chatTarget.value === channel
									? 'bg-blue-500/15 text-blue-200'
									: 'text-gray-300 hover:bg-white/[0.04] hover:text-gray-100'}"
								aria-label={group.note}
								onclick={() => onSelectChannel(channel)}
							>
								<span class="shrink-0 text-sm leading-none">{group.flag}</span>
								{#if unreadIds.has(knownCid)}
									<span
										class="mr-0.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"
										aria-label="unread"
									></span>
								{/if}
								<span
									class={channelsCollapsed
										? 'sr-only'
										: `truncate ${unreadIds.has(knownCid) ? 'font-semibold' : ''}`}
									>{group.note}</span
								>
							</button>
						{/each}
					</div>
				{/if}

				{#if resolvedChannels.unknown.length > 0}
					<div class="mt-1.5 space-y-0.5 border-t border-gray-700/60 pt-1.5">
						{#each resolvedChannels.unknown as channel (channel)}
							{@const unknownCid = conversationIdFor({ kind: 'channel', value: channel })}
							<button
								type="button"
								class="flex w-full items-center rounded px-2 py-1.5 text-left text-xs {channelsCollapsed
									? 'justify-center gap-0'
									: 'gap-2'} {chatTarget.kind === 'channel' && chatTarget.value === channel
									? 'bg-blue-500/15 text-blue-200'
									: 'text-gray-300 hover:bg-white/[0.04] hover:text-gray-100'}"
								aria-label={channel}
								title={channel}
								onclick={() => onSelectChannel(channel)}
							>
								<MdiIcon path={mdiPound} size={14} />
								{#if unreadIds.has(unknownCid)}
									<span
										class="mr-0.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"
										aria-label="unread"
									></span>
								{/if}
								<span
									class={channelsCollapsed
										? 'sr-only'
										: `truncate ${unreadIds.has(unknownCid) ? 'font-semibold' : ''}`}
									>{channel}</span
								>
							</button>
						{/each}
					</div>
				{/if}

				<div class="mt-2 border-t border-gray-700/60 pt-1.5">
					<div class="px-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-gray-600">
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
										: 'gap-2'} {chatTarget.kind === 'contact' && chatTarget.value === contact
										? 'bg-blue-500/15 text-blue-200'
										: 'text-gray-300 hover:bg-white/[0.04] hover:text-gray-100'}"
									aria-label={contact}
									title={contact}
									onclick={() => onSelectContact(contact)}
								>
									<MdiIcon path={mdiAccountOutline} size={14} />
									{#if unreadIds.has(dmCid)}
										<span
											class="mr-0.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"
											aria-label="unread"
										></span>
									{/if}
									<span
										class={channelsCollapsed
											? 'sr-only'
											: `truncate ${unreadIds.has(dmCid) ? 'font-semibold' : ''}`}>{contact}</span
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
						onclick={onNewDm}
					>
						<MdiIcon path={mdiPlus} size={14} />
						<span>{chatSidebarNewDmLabel(channelsCollapsed)}</span>
					</button>
				</div>
			</div>
		</aside>

		<section class="flex min-h-0 flex-col">
			<div class="flex h-10 shrink-0 items-center justify-between border-b border-gray-700/60 px-3">
				<div class="min-w-0 flex-1">
					{#if chatTarget.kind === 'channel' && chatTarget.value !== 'Broadcast'}
						{@const group = resolveGroup(chatTarget.value)}
						<div class="flex items-center gap-2">
							{#if group}
								<span class="text-xl leading-none">{group.flag}</span>
								<div>
									<div class="text-sm font-semibold text-white">{group.note}</div>
									<div class="text-[10px] text-gray-500">Group {group.group} · {group.prefix}</div>
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
				<label class="ml-2 min-w-0 shrink">
					<span class="sr-only">Filter messages</span>
					<input
						type="search"
						class="h-7 w-28 rounded border border-gray-700/60 bg-[#111827] px-2 text-xs text-gray-200 outline-none placeholder:text-gray-500 focus:border-blue-500/60 md:w-44"
						bind:value={chatFilter}
						placeholder="Filter"
					/>
				</label>
				<button
					type="button"
					class="ml-2 flex h-7 w-7 shrink-0 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label={isBroadcastTarget ? 'Clear messages' : 'Delete conversation'}
					title={isBroadcastTarget ? 'Clear messages' : 'Delete conversation'}
					onclick={onDelete}
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
						{chatFilter.trim() === '' ? 'No messages in this chat' : 'No matching messages'}
					</div>
				{:else}
					<div class="space-y-2">
						{#each displayChatRecords as record (chatRecordKey(record))}
							{@const sourcePath = splitSourcePath(record.src)}
							{@const isSent = sourcePath.origin === stationCallsign}
							{@const sequenceId = isSent ? chatRecordSeqId(record) : null}
							{@const ackEntries = sequenceId ? (ackIndex.acked.get(sequenceId) ?? []) : []}
							{@const rejectEntries = sequenceId ? (ackIndex.rejected.get(sequenceId) ?? []) : []}
							{@const gatewayAck =
								ackEntries.find((entry) => entry.ackSource === 'gateway') ?? null}
							{@const loraAck = ackEntries.find((entry) => entry.ackSource === 'lora') ?? null}
							{@const bestAck = gatewayAck ?? loraAck}
							{@const rttMs = bestAck
								? new Date(bestAck.receivedAt).getTime() - new Date(record.received_at).getTime()
								: -1}
							<div
								class="rounded border border-gray-700/60 bg-[#1c2230] p-2 md:p-3 transition-colors hover:border-gray-600/60"
							>
								<div class="flex items-center justify-between gap-2 md:gap-3">
									<div class="flex min-w-0 items-center gap-2">
										<span
											class="flex h-7 w-7 shrink-0 items-center justify-center rounded border border-blue-500/70 bg-blue-500/25 text-blue-200"
											title={chatIconTooltip(record)}
										>
											<MdiIcon path={chatMdiIcon(record)} size={16} />
										</span>
										<div class="min-w-0">
											<div class="truncate text-xs md:text-sm font-semibold text-white">
												{sourcePath.origin}
											</div>
											<div class="truncate text-[10px] text-gray-500">
												{messageKind(record.msg).label.toUpperCase()}
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
													<span class="text-[13px] text-gray-600" title="Sent, waiting for ACK"
														>✓</span
													>
												{/if}
											{:else if sequenceId && gatewayAck}
												<span
													class="text-[13px] text-green-400"
													title="Delivered via gateway ({gatewayAck.source})">☁️</span
												>
											{:else}
												<span class="text-[13px] text-green-400" title="Node echo observed">☁️</span
												>
											{/if}
										{/if}
									</div>
								</div>
								<div
									class="mt-2 rounded border border-gray-700/60 bg-[#111827] px-3 py-2 text-[11px] md:text-sm leading-relaxed text-gray-100"
								>
									{cleanMessage(record.msg) || record.msg}
								</div>
								{#if isSent && chatTarget.kind === 'contact' && sequenceId && bestAck}
									<div class="mt-1 flex items-center gap-2 font-mono text-[10px] text-green-600/70">
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
										{#if sourcePath.relays.length > 0}
											<span>via {sourcePath.relays.join(' → ')}</span>
										{/if}
										{#if record.rssi != null}
											<span class="flex items-center gap-0.5" title="RSSI — signal strength">
												<MdiIcon path={mdiSignalVariant} size={11} />
												{record.rssi} dBm
											</span>
										{/if}
										{#if record.snr != null}
											<span class="flex items-center gap-0.5" title="SNR — signal to noise ratio">
												<MdiIcon path={mdiTune} size={11} />
												SNR {record.snr}
											</span>
										{/if}
									</div>
									<button
										type="button"
										class="text-gray-600 hover:text-blue-300"
										onclick={() => onShowRawRecord(record)}
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
		</section>
	</div>
</div>
