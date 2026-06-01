<script lang="ts">
	import MeshMapPanel from '$lib/map/MeshMapPanel.svelte';
	import MapEventTicker from '$lib/map/MapEventTicker.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { eventsState } from '$lib/stores/events.svelte';
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
		/>
		<div class="pointer-events-none absolute bottom-10 left-2 z-[1100]">
			<div class="pointer-events-auto">
				<MapEventTicker events={eventsState.events} />
			</div>
		</div>
	</div>
</div>
