<script lang="ts">
	import {
		mdiSortAscending,
		mdiSortDescending,
		mdiMapMarkerOutline,
		mdiChatOutline
	} from '@mdi/js';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { eventsState } from '$lib/stores/events.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { chatState } from '$lib/stores/chat.svelte';
	import { goto } from '$app/navigation';
	import type { MapPosition } from '$lib/map/types';
	import { calculateDistanceKm } from '$lib/map/ruler';

	type SortKey = 'callsign' | 'lastHeard' | 'hops' | 'rssi' | 'snr' | 'distance';
	type SortDir = 'asc' | 'desc';

	let sortKey = $state<SortKey>('lastHeard');
	let sortDir = $state<SortDir>('desc');
	let filterText = $state('');
	let recentOnly = $state(true);
	const THREE_DAYS_MS = 3 * 24 * 60 * 60 * 1000;

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

	let rows = $derived(
		buildRows(eventsState.mapPositions, myPosition, connectionState.stationCallsign)
	);

	function buildRows(
		positions: MapPosition[],
		origin: MapPosition | null,
		myCallsign: string
	): NodeRow[] {
		const posMap = new Map(positions.map((p) => [p.id.toUpperCase(), p]));

		return positions
			.filter((pos) => pos.id.toUpperCase() !== myCallsign.toUpperCase())
			.map((pos) => {
				const distanceKm =
					origin != null && pos.lat != null && pos.lon != null
						? calculateDistanceKm(origin.lat, origin.lon, pos.lat, pos.lon)
						: null;

				const viaCallsigns = pos.via ?? [];
				const parts: string[] = [pos.id, ...viaCallsigns];

				// Cumulative path distance: sum of each segment IU1ABC→HOP1→…→myPos.
				// If any node along the path lacks a known position, skip the suffix.
				let pathDistKm: number | null = null;
				if (viaCallsigns.length > 0 && origin != null && origin.lat != null && origin.lon != null) {
					const hopNodes: Array<{ lat: number | null; lon: number | null } | null> = [
						pos,
						...viaCallsigns.map((cs) => posMap.get(cs.toUpperCase()) ?? null),
						origin
					];
					const allKnown = hopNodes.every((n) => n != null && n.lat != null && n.lon != null);
					if (allKnown) {
						pathDistKm = 0;
						for (let i = 0; i < hopNodes.length - 1; i++) {
							const a = hopNodes[i]!;
							const b = hopNodes[i + 1]!;
							pathDistKm += calculateDistanceKm(a.lat!, a.lon!, b.lat!, b.lon!);
						}
					}
				}

				const distSuffix = pathDistKm != null ? ` [${formatDist(pathDistKm)}]` : '';
				const sourcePath = parts.join(' → ') + distSuffix;

				return {
					callsign: pos.id,
					lastHeard: pos.lastSeen ?? '',
					hops: viaCallsigns.length,
					rssi: pos.rssi ? pos.rssi : null,
					snr: pos.snr ? pos.snr : null,
					lat: pos.lat,
					lng: pos.lon,
					sourcePath,
					distanceKm
				};
			});
	}

	function formatDist(km: number): string {
		if (km < 1) return `${Math.round(km * 1000)} m`;
		return `${km.toFixed(1)} km`;
	}

	let filtered = $derived(
		rows.filter((r) => {
			if (recentOnly && r.lastHeard && Date.now() - new Date(r.lastHeard).getTime() > THREE_DAYS_MS)
				return false;
			if (
				filterText.trim() !== '' &&
				!r.callsign.toUpperCase().includes(filterText.trim().toUpperCase())
			)
				return false;
			return true;
		})
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

	function openChat(row: NodeRow) {
		chatState.selectContact(row.callsign);
		void goto('/chat');
	}
</script>

<div class="flex h-full flex-col">
	<!-- Header -->
	<div
		class="flex h-9 shrink-0 items-center justify-between gap-3 border-b border-gray-700/60 px-3"
	>
		<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">Nodes</span>
		<div class="flex items-center gap-2">
			<label class="flex cursor-pointer items-center gap-1.5 text-[11px] text-gray-400 select-none">
				<input type="checkbox" class="accent-blue-500" bind:checked={recentOnly} />
				last 3d
			</label>
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
			<table class="w-full min-w-[480px] border-collapse text-xs">
				<thead class="sticky top-0 bg-[#1c2230]">
					<tr class="border-b border-gray-700/60">
						{#each [{ key: 'callsign', label: 'Callsign', smOnly: false }, { key: 'lastHeard', label: 'Last Heard', smOnly: false }, { key: 'rssi', label: 'RSSI', smOnly: true }, { key: 'snr', label: 'SNR', smOnly: true }, { key: 'hops', label: 'Hops', smOnly: false }, { key: 'distance', label: 'Distance', smOnly: false }] as col (col.key)}
							<th
								class="cursor-pointer select-none px-3 py-2 text-left font-semibold uppercase tracking-wider text-gray-500 hover:text-gray-300 {col.smOnly
									? 'hidden sm:table-cell'
									: ''}"
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
							<td class="hidden px-3 py-2 font-mono text-gray-400 sm:table-cell">
								{row.rssi != null ? `${row.rssi} dBm` : '—'}
							</td>
							<td class="hidden px-3 py-2 font-mono text-gray-400 sm:table-cell">
								{row.snr != null ? row.snr : '—'}
							</td>
							<td class="px-3 py-2">
								{#if row.hops === 0 && row.lastHeard && Date.now() - new Date(row.lastHeard).getTime() < 2 * 60 * 60 * 1000}
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
								<div class="flex items-center gap-1">
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
									<button
										type="button"
										class="flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] text-emerald-400 hover:bg-emerald-500/10 hover:text-emerald-300"
										title="Open DM chat"
										onclick={() => openChat(row)}
									>
										<MdiIcon path={mdiChatOutline} size={13} />
										Chat
									</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
