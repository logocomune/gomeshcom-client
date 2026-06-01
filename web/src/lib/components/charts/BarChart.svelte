<script lang="ts">
	interface Serie {
		key: string;
		label: string;
		color: string;
	}

	interface Props {
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		data: Array<Record<string, any>>;
		series: Serie[];
		height?: number;
		showLegend?: boolean;
	}

	let { data, series, height = 200, showLegend = true }: Props = $props();

	const W = 600; // fixed SVG coordinate width
	const GAP = 1; // px gap between bars in SVG units
	const LABEL_AREA = 40; // px reserved below bars for rotated labels
	const Y_PAD_TOP = 8;
	const FONT_SIZE = 10;

	// Rotate labels when bars are narrow (< 18 SVG units each).
	let barW = $derived(Math.max(1, W / Math.max(data.length, 1) - GAP));
	let rotateLabels = $derived(barW < 18);

	// Show every Nth label to avoid overlap.
	// Rotated: ~14px per char at angle, approx 20px pitch needed.
	// Straight: need barW * labelStep >= ~28px.
	let labelStep = $derived(
		rotateLabels
			? Math.max(1, Math.ceil((20 * data.length) / W))
			: Math.max(1, Math.ceil((28 * data.length) / W))
	);

	let drawH = $derived(height - LABEL_AREA - Y_PAD_TOP);

	let chartMax = $derived(
		data.reduce((mx, d) => {
			const total = series.reduce((s, sr) => s + (Number(d[sr.key]) || 0), 0);
			return Math.max(mx, total);
		}, 1)
	);
</script>

<div class="w-full overflow-hidden">
	{#if showLegend}
		<div class="mb-2 flex flex-wrap gap-3">
			{#each series as sr (sr.key)}
				<span class="flex items-center gap-1 text-xs text-gray-400">
					<span class="inline-block h-2.5 w-2.5 rounded-sm" style="background:{sr.color}"></span>
					{sr.label}
				</span>
			{/each}
		</div>
	{/if}

	<svg
		viewBox="0 0 {W} {height}"
		preserveAspectRatio="xMidYMid meet"
		class="w-full"
		style="height:{height}px"
	>
		{#each data as d, i (i)}
			{@const x = i * (W / Math.max(data.length, 1)) + GAP / 2}
			{@const cx = x + barW / 2}

			{#each series as sr, si (sr.key)}
				{@const below = series.slice(0, si).reduce((s, p) => s + (Number(d[p.key]) || 0), 0)}
				{@const val = Number(d[sr.key]) || 0}
				{@const barH = (val / chartMax) * drawH}
				{@const belowH = (below / chartMax) * drawH}
				{@const y = Y_PAD_TOP + drawH - belowH - barH}

				{#if barH > 0}
					<rect x={x} y={y} width={barW} height={barH} fill={sr.color} rx="0.5">
						<title>{sr.label}: {val}</title>
					</rect>
				{/if}
			{/each}

			{#if i % labelStep === 0}
				{#if rotateLabels}
					<text
						x={cx}
						y={Y_PAD_TOP + drawH + 4}
						text-anchor="end"
						font-size={FONT_SIZE}
						fill="var(--color-ink-dim)"
						transform="rotate(-45, {cx}, {Y_PAD_TOP + drawH + 4})"
					>{d.label}</text>
				{:else}
					<text
						x={cx}
						y={height - 4}
						text-anchor="middle"
						font-size={FONT_SIZE}
						fill="var(--color-ink-dim)"
					>{d.label}</text>
				{/if}
			{/if}
		{/each}
	</svg>
</div>
