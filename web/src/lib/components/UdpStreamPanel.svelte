<script lang="ts">
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { eventSummary, packetBadge, packetFromEvent, splitSourcePath } from '$lib/api/events';
	import { hardwareHumanName } from '$lib/api/hardware';
	import type { StreamEvent } from '$lib/api/types';
	import { formatTime } from '$lib/ui/format';
	import {
		iconTooltip,
		isMessageEvent,
		mdiForEvent,
		messageRoute,
		messageText,
		packetTone
	} from '$lib/ui/stream';
	import {
		mdiArrowRight,
		mdiBattery,
		mdiChip,
		mdiClose,
		mdiDeleteSweepOutline,
		mdiCodeJson,
		mdiGauge,
		mdiMapMarkerRadiusOutline,
		mdiSignalVariant,
		mdiThermometer,
		mdiTune,
		mdiWaterPercent
	} from '@mdi/js';

	let {
		events,
		filteredEvents,
		streamFilter = $bindable(),
		selectedEvent,
		isDesktop,
		streamHeightPx,
		selectEvent,
		onClearEvents,
		showRawEvent
	}: {
		events: StreamEvent[];
		filteredEvents: StreamEvent[];
		streamFilter: string;
		selectedEvent: StreamEvent | null;
		isDesktop: boolean;
		streamHeightPx: number;
		selectEvent: (event: StreamEvent) => void;
		onClearEvents: () => void;
		showRawEvent: (event: StreamEvent) => void;
	} = $props();

	function handleRowKeydown(keyEvent: KeyboardEvent, event: StreamEvent) {
		if (keyEvent.key === 'Enter' || keyEvent.key === ' ') {
			selectEvent(event);
		}
	}
</script>

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
			<button
				type="button"
				class="flex h-6 w-6 items-center justify-center rounded border border-gray-700/60 text-gray-500 hover:border-red-500/50 hover:text-red-300"
				aria-label="Clear UDP stream"
				title="Clear UDP stream"
				onclick={onClearEvents}
			>
				<MdiIcon path={mdiDeleteSweepOutline} size={14} />
			</button>
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
						onclick={() => selectEvent(event)}
						onkeydown={(keyEvent) => handleRowKeydown(keyEvent, event)}
					>
						<div class="font-mono text-[10px] md:text-[11px] text-gray-500">
							{formatTime(event.receivedAt)}
						</div>
						<div class="min-w-0">
							{#if isMessageEvent(event)}
								{@const route = messageRoute(event)}
								{@const packet = packetFromEvent(event)}
								{@const text = messageText(event)}
								<div class="flex min-w-0 items-center gap-1.5">
									<span
										class="flex h-6 w-6 shrink-0 items-center justify-center rounded border {packetTone(
											event
										)}"
										title={iconTooltip(event)}
									>
										<MdiIcon path={mdiForEvent(event)} size={17} />
									</span>
									<span class="shrink-0 text-xs md:text-sm font-bold text-white"
										>{route.origin}</span
									>
									{#if route.relays.length > 0}
										<span class="hidden md:inline shrink-0 text-[11px] text-gray-400"
											>(via {route.relays.join(', ')})</span
										>
									{/if}
									<span class="hidden md:inline shrink-0 text-gray-500"
										><MdiIcon path={mdiArrowRight} size={13} /></span
									>
									<span class="hidden md:inline shrink-0 text-sm text-gray-200"
										>{route.destination}</span
									>
									<span class="mx-0.5 shrink-0 text-gray-400">·</span>
									<span class="min-w-0 truncate italic text-xs md:text-sm text-white">"{text}"</span
									>
									{#if packet?.rssi != null || packet?.snr != null}
										{@render QualityValues(packet)}
									{/if}
								</div>
							{:else if packetFromEvent(event)?.type === 'tele'}
								{@const packet = packetFromEvent(event)}
								{@const source = splitSourcePath(packet?.src)}
								<div class="flex min-w-0 items-center gap-1.5">
									{@render EventIcon(event)}
									<span class="shrink-0 text-xs md:text-sm font-bold text-white"
										>{source.origin}</span
									>
									{#if source.relays.length > 0}
										<span class="hidden md:inline shrink-0 text-[11px] text-gray-400"
											>(via {source.relays.join(', ')})</span
										>
									{/if}
									<span class="mx-0.5 shrink-0 text-gray-400">·</span>
									<span class="flex min-w-0 items-center gap-2">
										{#if packet?.batt != null}
											<span class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200">
												<span class="text-gray-500"><MdiIcon path={mdiBattery} size={12} /></span>
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
										{@render QualityValues(packet)}
									{/if}
								</div>
							{:else if packetFromEvent(event)?.type === 'pos'}
								{@const packet = packetFromEvent(event)}
								{@const source = splitSourcePath(packet?.src)}
								{@const hardware = hardwareHumanName(packet?.hw_id)}
								<div class="flex min-w-0 items-center gap-1.5">
									{@render EventIcon(event)}
									<span class="shrink-0 text-xs md:text-sm font-bold text-white"
										>{source.origin}</span
									>
									{#if source.relays.length > 0}
										<span class="hidden md:inline shrink-0 text-[11px] text-gray-400"
											>(via {source.relays.join(', ')})</span
										>
									{/if}
									<span class="mx-0.5 shrink-0 text-gray-400">·</span>
									<span class="flex min-w-0 items-center gap-2">
										{#if packet?.lat != null && packet?.long != null}
											<span class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200">
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
											<span class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200">
												<span class="text-gray-500"><MdiIcon path={mdiBattery} size={12} /></span>
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
										{@render QualityValues(packet)}
									{/if}
								</div>
							{:else}
								{@const packet = packetFromEvent(event)}
								{@const source = splitSourcePath(packet?.src)}
								<div class="flex min-w-0 items-center gap-1.5">
									{@render EventIcon(event)}
									{#if packet}
										<span class="shrink-0 text-xs md:text-sm font-bold text-white"
											>{source.origin}</span
										>
										{#if source.relays.length > 0}
											<span class="hidden md:inline shrink-0 text-[11px] text-gray-400"
												>(via {source.relays.join(', ')})</span
											>
										{/if}
										<span class="mx-0.5 shrink-0 text-gray-400">·</span>
										<span class="flex min-w-0 items-center gap-2">
											{#if packet.lat != null && packet.long != null}
												<span class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200">
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
												<span class="flex shrink-0 items-center gap-0.5 text-[11px] text-gray-200">
													<span class="text-gray-500"><MdiIcon path={mdiBattery} size={12} /></span>
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
										<span class="min-w-0 truncate text-sm text-gray-300">{eventSummary(event)}</span
										>
									{/if}
									{#if packet?.rssi != null || packet?.snr != null}
										{@render QualityValues(packet)}
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
								showRawEvent(event);
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

{#snippet EventIcon(event: StreamEvent)}
	<span
		class="flex h-6 w-6 shrink-0 items-center justify-center rounded border {packetTone(event)}"
		title={iconTooltip(event)}
	>
		<MdiIcon path={mdiForEvent(event)} size={17} />
	</span>
{/snippet}

{#snippet QualityValues(packet: NonNullable<ReturnType<typeof packetFromEvent>>)}
	<span class="ml-auto flex shrink-0 items-center gap-2 pl-2">
		{#if packet.rssi != null}
			<span
				class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
				title="RSSI — received signal strength (dBm)"
			>
				<span class="text-gray-300"><MdiIcon path={mdiSignalVariant} size={14} /></span>
				{packet.rssi}
			</span>
		{/if}
		{#if packet.snr != null}
			<span
				class="flex items-center gap-0.5 font-mono text-[10px] text-gray-300"
				title="SNR — signal-to-noise ratio (dB)"
			>
				<span class="text-gray-300"><MdiIcon path={mdiTune} size={14} /></span>
				{packet.snr}
			</span>
		{/if}
	</span>
{/snippet}
