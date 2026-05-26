<script lang="ts">
	import MeshMapPanel from '$lib/map/MeshMapPanel.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { eventsState } from '$lib/stores/events.svelte';
</script>

<svelte:head>
	<title>Map - goMeshCom</title>
</svelte:head>

<div
	data-testid="map-panel"
	class="m-2 flex min-h-0 flex-1 flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#151a24] shadow-sm"
>
	<div class="flex h-9 shrink-0 items-center justify-between border-b border-gray-700/60 px-3">
		<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">Map</span>
		<span class="font-mono text-[11px] text-gray-500">{eventsState.mapPositions.length} nodes</span>
	</div>
	<div class="relative min-h-0 flex-1 overflow-hidden">
		<MeshMapPanel
			positions={eventsState.mapPositions}
			myCall={connectionState.stationCallsign}
			events={eventsState.events}
		/>
	</div>
</div>
