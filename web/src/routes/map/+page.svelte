<script lang="ts">
	import MeshMapPanel from '$lib/map/MeshMapPanel.svelte';
	import MapEventTicker from '$lib/map/MapEventTicker.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { eventsState } from '$lib/stores/events.svelte';

	let dmTracking = $state(false);
</script>

<svelte:head>
	<title>Map - goMeshCom</title>
</svelte:head>

<div
	data-testid="map-panel"
	class="m-2 flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-ink-dim/20 bg-surface shadow-sm"
>
	<div class="flex h-9 shrink-0 items-center justify-between border-b border-ink-dim/20 px-3">
		<span class="text-[11px] font-semibold uppercase tracking-wider text-ink-muted">Map</span>
		<span class="font-mono text-[11px] text-ink-muted">{eventsState.mapPositions.length} nodes</span>
	</div>
	<div class="relative min-h-0 flex-1 overflow-hidden">
		<MeshMapPanel
			positions={eventsState.mapPositions}
			myCall={connectionState.stationCallsign}
			events={eventsState.events}
			bind:dmTracking
		/>
		<div class="pointer-events-none absolute bottom-2 left-2 z-[1100]">
			<div class="pointer-events-auto flex flex-col gap-1">
				<MapEventTicker events={eventsState.events} />
				{#if dmTracking}
				<div class="hidden md:block rounded-xl border border-ink-dim/20 bg-base/80 px-2 py-1.5 text-[10px] text-ink-muted shadow-md backdrop-blur-sm">
					<div class="flex flex-col gap-0.5">
						<div class="flex items-center gap-1.5">
							<span class="inline-block h-2.5 w-2.5 rounded-full border border-[#38bdf8] bg-[#38bdf8]/30"></span><span>DM</span>
							<span class="ml-1 inline-block h-2.5 w-2.5 rounded-full border border-[#a855f7] bg-[#a855f7]/30"></span><span>ACK</span>
							<span class="ml-1 inline-block h-2.5 w-2.5 rounded-full border border-[#f59e0b] bg-[#f59e0b]/30"></span><span>Broadcast</span>
						</div>
						<div class="flex items-center gap-1.5">
							<span class="inline-block h-2.5 w-2.5 rounded-full border border-[#34d399] bg-[#34d399]/30"></span><span>Position</span>
							<span class="ml-1 inline-block h-2.5 w-2.5 rounded-full border border-[#f97316] bg-[#f97316]/30"></span><span>Telemetry</span>
							<span class="ml-1 inline-block h-2.5 w-2.5 rounded-full border border-[#facc15] bg-[#facc15]/30"></span><span>Relay</span>
						</div>
						<div class="mt-0.5 border-t border-ink-dim/20 pt-0.5 flex flex-col gap-0.5">
							<div class="flex items-center gap-1.5">
								<svg width="20" height="4" class="shrink-0"><line x1="0" y1="2" x2="20" y2="2" stroke="#38bdf8" stroke-width="2" stroke-dasharray="4,4"/></svg>
								<span>DM path</span>
							</div>
							<div class="flex items-center gap-1.5">
								<svg width="20" height="4" class="shrink-0"><line x1="0" y1="2" x2="20" y2="2" stroke="#a855f7" stroke-width="2" stroke-dasharray="4,4"/></svg>
								<span>ACK path</span>
							</div>
						</div>
					</div>
				</div>
				{/if}
			</div>
		</div>
	</div>
</div>
