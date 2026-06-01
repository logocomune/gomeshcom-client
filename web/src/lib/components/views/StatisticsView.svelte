<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchStats } from '$lib/api/stats';
	import type { Bucket } from '$lib/api/stats';
	import { buildHourlySeries, buildDailySeries, sumKpis, buildDistanceHistogram, buildChannelTable } from '$lib/stats/transform';
	import BarChart from '$lib/components/charts/BarChart.svelte';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import {
		mdiSigma,
		mdiEmailOutline,
		mdiCheckCircleOutline,
		mdiEmailMultipleOutline,
		mdiBroadcast,
		mdiMapMarkerRadiusOutline
	} from '@mdi/js';

	const CHART_SERIES = [
		{ key: 'dm', label: 'DM', color: '#60a5fa' },
		{ key: 'dm_ack', label: 'DM Ack', color: '#34d399' },
		{ key: 'public', label: 'Public', color: '#a78bfa' },
		{ key: 'telemetry', label: 'Telemetry', color: '#fbbf24' },
		{ key: 'position', label: 'Position', color: '#f87171' }
	];

	const DIST_COLOR = '#60a5fa';

	const RANGE_OPTIONS = [
		{ label: '6 h', hours: 6 },
		{ label: '24 h', hours: 24 },
		{ label: '7 d', hours: 168 },
		{ label: '30 d', hours: 720 }
	];

	const STORAGE_KEY = 'stats.range.hours';
	let selectedHours = $state(Number(localStorage.getItem(STORAGE_KEY)) || 24);
	let buckets = $state<Bucket[]>([]);
	let rangeFrom = $state('');
	let rangeTo = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);

	let kpis = $derived(sumKpis(buckets));

	let hourlySeries = $derived(
		selectedHours >= 168
			? buildDailySeries(buckets, rangeFrom, rangeTo)
			: buildHourlySeries(buckets, rangeFrom, rangeTo)
	);
	let channelRows = $derived(buildChannelTable(buckets));
	let distanceBins = $derived(buildDistanceHistogram(buckets));
	let distChartData = $derived(distanceBins.map((b) => ({ label: b.label, count: b.count })));
	let distChartSeries = $derived([{ key: 'count', label: 'Packets', color: DIST_COLOR }]);

	async function load(hours: number) {
		loading = true;
		error = null;
		try {
			const res = await fetchStats(hours);
			buckets = res.buckets ?? [];
			rangeFrom = res.from;
			rangeTo = res.to;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load statistics';
		} finally {
			loading = false;
		}
	}

	async function selectRange(hours: number) {
		selectedHours = hours;
		localStorage.setItem(STORAGE_KEY, String(hours));
		await load(hours);
	}

	onMount(() => {
		load(selectedHours);
		const interval = setInterval(() => load(selectedHours), 60_000);
		return () => clearInterval(interval);
	});
</script>

<div class="flex h-full flex-col gap-3 overflow-y-auto p-3">
	<!-- Range selector -->
	<div class="flex items-center gap-2">
		<span class="text-xs text-ink-muted">Range:</span>
		{#each RANGE_OPTIONS as opt (opt.hours)}
			<button
				onclick={() => selectRange(opt.hours)}
				class="rounded-lg px-2 py-0.5 text-xs transition-colors {selectedHours === opt.hours
					? 'bg-warm text-base font-semibold'
					: 'bg-surface-hi text-ink-muted hover:bg-surface-hi/80'}"
			>
				{opt.label}
			</button>
		{/each}
		{#if loading}
			<span class="ml-2 text-xs text-ink-muted">Loading…</span>
		{/if}
		{#if error}
			<span class="ml-2 text-xs text-coral">{error}</span>
		{/if}
	</div>

	<!-- KPI cards -->
	<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-6">
		{#each [
			{ label: 'Total', value: kpis.total, color: 'text-ink', icon: mdiSigma, pct: null },
			{ label: 'DM received', value: kpis.dm, color: 'text-azure', icon: mdiEmailOutline },
			{ label: 'DM ack', value: kpis.dm_ack, color: 'text-mint', icon: mdiCheckCircleOutline },
			{ label: 'Public', value: kpis.public, color: 'text-lavender', icon: mdiEmailMultipleOutline },
			{ label: 'Telemetry', value: kpis.telemetry, color: 'text-warm', icon: mdiBroadcast },
			{ label: 'Position', value: kpis.position, color: 'text-coral', icon: mdiMapMarkerRadiusOutline }
		] as card (card.label)}
			<div class="flex flex-col rounded-2xl border border-ink-dim/20 bg-surface px-3 py-2">
				<div class="flex items-center gap-1 {card.color}">
					<MdiIcon path={card.icon} size={12} />
					<span class="text-xs text-ink-muted">{card.label}</span>
				</div>
				<span class="text-lg font-semibold {card.color}">{card.value}</span>
			</div>
		{/each}
	</div>

	<!-- Messages per hour chart -->
	<div class="rounded-2xl border border-ink-dim/20 bg-surface p-3">
		<h2 class="mb-2 text-xs font-medium uppercase tracking-[0.2em] text-ink-muted">
			Messages per hour
		</h2>
		{#if hourlySeries.length > 0}
			<BarChart data={hourlySeries} series={CHART_SERIES} height={200} />
		{:else}
			<p class="py-6 text-center text-xs text-ink-muted">No data for selected range</p>
		{/if}
	</div>

	<!-- Per-channel / per-DM breakdown -->
	<div class="rounded-2xl border border-ink-dim/20 bg-surface p-3">
		<h2 class="mb-2 text-xs font-medium uppercase tracking-[0.2em] text-ink-muted">
			Messages by channel / contact
		</h2>
		{#if channelRows.length > 0}
			<table class="w-full text-xs">
				<thead>
					<tr class="border-b border-ink-dim/20 text-left text-ink-muted">
						<th class="pb-1 pr-4 font-medium">Channel / Contact</th>
						<th class="pb-1 pr-4 font-medium">Type</th>
						<th class="pb-1 text-right font-medium">Messages</th>
					</tr>
				</thead>
				<tbody>
					{#each channelRows as row (row.key)}
						<tr class="border-b border-ink-dim/10 hover:bg-surface-hi">
							<td class="py-1 pr-4 font-mono text-ink">{row.label}</td>
							<td class="py-1 pr-4">
								{#if row.kind === 'dm'}
									<span class="rounded bg-azure/15 px-1.5 py-0.5 text-azure">DM</span>
								{:else if row.kind === 'broadcast'}
									<span class="rounded bg-lavender/15 px-1.5 py-0.5 text-lavender">Broadcast</span>
								{:else}
									<span class="rounded bg-ink-dim/20 px-1.5 py-0.5 text-ink-muted">Channel</span>
								{/if}
							</td>
							<td class="py-1 text-right font-semibold text-ink">{row.count}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{:else}
			<p class="py-4 text-center text-xs text-ink-muted">No data for selected range</p>
		{/if}
	</div>

	<!-- Distance histogram -->
	<div class="rounded-2xl border border-ink-dim/20 bg-surface p-3">
		<h2 class="mb-2 text-xs font-medium uppercase tracking-[0.2em] text-ink-muted">
			Position packets by distance from own station (km)
		</h2>
		{#if distanceBins.length > 0}
			<BarChart data={distChartData} series={distChartSeries} height={160} showLegend={false} />
		{:else}
			<p class="py-6 text-center text-xs text-ink-muted">
				No position data — own station position required
			</p>
		{/if}
	</div>
</div>
