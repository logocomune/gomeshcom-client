<script lang="ts">
	/** Seed string (callsign) used to generate the avatar. */
	interface Props {
		seed: string;
		size?: number;
	}

	let { seed, size = 32 }: Props = $props();

	const GRID = 8;

	/** FNV-1a 32-bit hash — fast and well-distributed for short strings. */
	function fnv1a(str: string): number {
		let hash = 0x811c9dc5;
		for (let i = 0; i < str.length; i++) {
			hash ^= str.charCodeAt(i);
			hash = (hash * 0x01000193) >>> 0;
		}
		return hash;
	}

	/** Derives a deterministic HSL foreground color from the seed. */
	function seedColor(s: string): string {
		const h = fnv1a(s + 'hue') % 360;
		const sat = 50 + (fnv1a(s + 'sat') % 30);
		const l = 45 + (fnv1a(s + 'lit') % 20);
		return `hsl(${h},${sat}%,${l}%)`;
	}

	/** Builds an 8×8 boolean grid. Left 4 columns mirrored to the right. */
	function buildGrid(s: string): boolean[][] {
		const rows: boolean[][] = [];
		for (let row = 0; row < GRID; row++) {
			const half: boolean[] = [];
			for (let col = 0; col < GRID / 2; col++) {
				const bit = (fnv1a(s + row + col) >> (col % 32)) & 1;
				half.push(bit === 1);
			}
			rows.push([...half, ...[...half].reverse()]);
		}
		return rows;
	}

	let color = $derived(seedColor(seed));
	let grid = $derived(buildGrid(seed));
	let cellSize = $derived(size / GRID);
</script>

<svg
	width={size}
	height={size}
	viewBox="0 0 {size} {size}"
	xmlns="http://www.w3.org/2000/svg"
	shape-rendering="crispEdges"
>
	<rect width={size} height={size} fill="#1e2330" />
	{#each grid as row, rowIdx}
		{#each row as filled, colIdx}
			{#if filled}
				<rect
					x={colIdx * cellSize}
					y={rowIdx * cellSize}
					width={cellSize}
					height={cellSize}
					fill={color}
				/>
			{/if}
		{/each}
	{/each}
</svg>
