<script lang="ts">
	import { mdiSortAscending, mdiSortDescending, mdiMapMarkerOutline } from '@mdi/js';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { eventsState } from '$lib/stores/events.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { goto } from '$app/navigation';
	import type { MapPosition } from '$lib/map/types';
	import { calculateDistanceKm } from '$lib/map/ruler';

	type SortKey = 'callsign' | 'lastHeard' | 'hops' | 'rssi' | 'snr' | 'distance';
	type SortDir = 'asc' | 'desc';

	let sortKey = $state<SortKey>('lastHeard');
	let sortDir = $state<SortDir>('desc');
	let filterText = $state('');

	interface NodeRow {
		callsign: string;
		lastHeard: string;
		hops: number;
		rssi: number | null;
		snr: number | null;
		lat: number | null;
		lng: number | null;
		sourcePath: string;
		distanceKm: number | null;
	}

	// Own station position — derived reactively from the same mapPositions list.
	let myPosition = $derived(
		connectionState.stationCallsign !== ''
			? (eventsState.mapPositions.find(
					(p) => p.source.toUpperCase() === connectionState.stationCallsign.toUpperCase()
				) ?? null)
			: null
	);

	let rows = $derived(buildRows(eventsState.mapPositions, myPosition));

	function buildRows(positions: MapPosition[], origin: MapPosition | null): NodeRow[] {
		return positions.map((pos) => ({
			callsign: pos.id,
			lastHeard: pos.lastSeen ?? '',
			hops: pos.via?.length ?? 0,
			rssi: pos.rssi ? pos.rssi : null,
			snr: pos.snr ? pos.snr : null,
			lat: pos.lat,
			lng: pos.lon,
			sourcePath: pos.via ? pos.via.join(' → ') : '',
			distanceKm:
				origin != null && pos.lat != null && pos.lon != null
					? calculateDistanceKm(origin.lat, origin.lon, pos.lat, pos.lon)
					: null
		}));
	}

	function formatDist(km: number): string {
		if (km < 1) return `${Math.round(km * 1000)} m`;
		return `${km.toFixed(1)} km`;
	}

	let filtered = $derived(
		filterText.trim() === ''
			? rows
			: rows.filter((r) => r.callsign.toUpperCase().includes(filterText.trim().toUpperCase()))
	);

	let sorted = $derived(
		[...filtered].sort((a, b) => {
			if (sortKey === 'distance') {
				// Nodes without a fix always sort last regardless of direction.
				if (a.distanceKm == null && b.distanceKm == null) return 0;
				if (a.distanceKm == null) return 1;
				if (b.distanceKm == null) return -1;
				const cmp = a.distanceKm - b.distanceKm;
				return sortDir === 'asc' ? cmp : -cmp;
			}
			let cmp = 0;
			switch (sortKey) {
				case 'callsign':
					cmp = a.callsign.localeCompare(b.callsign);
					break;
				case 'lastHeard':
					cmp = a.lastHeard.localeCompare(b.lastHeard);
					break;
				case 'hops':
					cmp = a.hops - b.hops;
					break;
				case 'rssi':
					cmp = (a.rssi ?? -999) - (b.rssi ?? -999);
					break;
				case 'snr':
					cmp = (a.snr ?? -999) - (b.snr ?? -999);
					break;
			}
			return sortDir === 'asc' ? cmp : -cmp;
		})
	);

	function toggleSort(key: SortKey) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'desc';
		}
	}

	function formatHeard(isoTs: string): string {
		if (!isoTs) return '—';
		const diff = Date.now() - new Date(isoTs).getTime();
		if (isNaN(diff)) return '—';
		const secs = Math.floor(diff / 1000);
		if (secs < 60) return `${secs}s ago`;
		const mins = Math.floor(secs / 60);
		if (mins < 60) return `${mins}m ago`;
		const hours = Math.floor(mins / 60);
		if (hours < 24) return `${hours}h ago`;
		return `${Math.floor(hours / 24)}d ago`;
	}

	function focusOnMap(row: NodeRow) {
		if (row.lat == null || row.lng == null) return;
		eventsState.focusOnNode(row.callsign, row.lat, row.lng);
		void goto('/map');
	}
</script>

<div class="flex h-full flex-col">
	<!-- Header -->
	<div
		class="flex h-9 shrink-0 items-center justify-between gap-3 border-b border-gray-700/60 px-3"
	>
		<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">Nodes</span>
		<div class="flex items-center gap-2">
			<input
				type="search"
				class="h-6 w-36 rounded border border-gray-700/60 bg-[#111827] px-2 text-xs text-gray-200 outline-none placeholder:text-gray-500 focus:border-blue-500/60"
				placeholder="Filter callsign…"
				bind:value={filterText}
			/>
			<span class="font-mono text-[11px] text-gray-500">
				{filtered.length}/{rows.length}
			</span>
		</div>
	</div>

	{#if rows.length === 0}
		<div class="flex flex-1 items-center justify-center text-sm text-gray-600">
			No nodes heard yet — waiting for traffic…
		</div>
	{:else}
		<div class="min-h-0 flex-1 overflow-auto">
			<table class="w-full min-w-[680px] border-collapse text-xs">
				<thead class="sticky top-0 bg-[#1c2230]">
					<tr class="border-b border-gray-700/60">
						{#each [{ key: 'callsign', label: 'Callsign' }, { key: 'lastHeard', label: 'Last Heard' }, { key: 'rssi', label: 'RSSI' }, { key: 'snr', label: 'SNR' }, { key: 'hops', label: 'Hops' }, { key: 'distance', label: 'Distance' }] as col (col.key)}
							<th
								class="cursor-pointer select-none px-3 py-2 text-left font-semibold uppercase tracking-wider text-gray-500 hover:text-gray-300"
								onclick={() => toggleSort(col.key as SortKey)}
							>
								<span class="flex items-center gap-1">
									{col.label}
									{#if sortKey === col.key}
										<MdiIcon
											path={sortDir === 'asc' ? mdiSortAscending : mdiSortDescending}
											size={13}
										/>
									{/if}
								</span>
							</th>
						{/each}
						<th class="px-3 py-2 text-left font-semibold uppercase tracking-wider text-gray-500">
							Path
						</th>
						<th class="px-3 py-2"></th>
					</tr>
				</thead>
				<tbody>
					{#each sorted as row (row.callsign)}
						<tr class="border-b border-gray-700/30 transition-colors hover:bg-gray-700/20">
							<td
								class="px-3 py-2 font-mono font-semibold {row.hops === 0
									? 'text-emerald-400'
									: 'text-blue-300'}"
							>
								{row.callsign}
							</td>
							<td class="px-3 py-2 text-gray-400">
								{formatHeard(row.lastHeard)}
							</td>
							<td class="px-3 py-2 font-mono text-gray-400">
								{row.rssi != null ? `${row.rssi} dBm` : '—'}
							</td>
							<td class="px-3 py-2 font-mono text-gray-400">
								{row.snr != null ? row.snr : '—'}
							</td>
							<td class="px-3 py-2">
								{#if row.hops === 0}
									<span class="font-semibold text-emerald-400">direct</span>
								{:else}
									<span class="text-gray-400">{row.hops}</span>
								{/if}
							</td>
							<td class="px-3 py-2 font-mono text-gray-400">
								{row.distanceKm != null ? formatDist(row.distanceKm) : '—'}
							</td>
							<td class="px-3 py-2 text-gray-500">
								{row.sourcePath || '—'}
							</td>
							<td class="px-3 py-2">
								{#if row.lat != null && row.lng != null}
									<button
										type="button"
										class="flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] text-blue-400 hover:bg-blue-500/10 hover:text-blue-300"
										title="Show on map"
										onclick={() => focusOnMap(row)}
									>
										<MdiIcon path={mdiMapMarkerOutline} size={13} />
										Map
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
