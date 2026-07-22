<script lang="ts">
	import { onDestroy } from 'svelte';
	import { eventsState } from '$lib/stores/events.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import {
		buildNodeGraph,
		layoutNodeGraph,
		nodePathSummaries,
		visibleNodeGraph
	} from '$lib/graph/node-graph';
	import {
		initialViewport,
		panViewport,
		zoomViewport,
		type GraphViewport
	} from '$lib/graph/viewport';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { mdiFitToPageOutline, mdiMagnifyMinusOutline, mdiMagnifyPlusOutline } from '@mdi/js';

	const HOP_MODES = ['2', '3', 'all'] as const;
	const EMPTY_EXPANDED_NODE_IDS = new Set<string>();
	const PATH_SUMMARY_HIDE_DELAY_MS = 7000;
	type HopMode = (typeof HOP_MODES)[number];

	let svgElement: SVGSVGElement | null = $state(null);
	let viewport = $state<GraphViewport>(initialViewport(360, 260));
	let viewportKey = $state('');
	let hopMode = $state<HopMode>('2');
	let showStaleNodes = $state(true);
	let hoveredEdgeId = $state<string | null>(null);
	let hoveredNodeId = $state<string | null>(null);
	let selectedNodeId = $state<string | null>(null);
	let pathSummaryNodeId = $state<string | null>(null);
	let pathSummaryHideTimer: ReturnType<typeof setTimeout> | null = null;
	let dragStart = $state<{
		clientX: number;
		clientY: number;
		viewport: GraphViewport;
	} | null>(null);

	const graph = $derived(
		buildNodeGraph(connectionState.stationCallsign, eventsState.mapPositions, {
			includeHidden: false,
			includeStale: showStaleNodes
		})
	);
	const visibleDepth = $derived(hopMode === 'all' ? graph.maxDepth : Number(hopMode));
	const visibleGraph = $derived(
		visibleNodeGraph(graph, {
			maxDepth: visibleDepth,
			expandedNodeIds: EMPTY_EXPANDED_NODE_IDS
		})
	);
	const layout = $derived(layoutNodeGraph(visibleGraph));
	const activeNodeId = $derived(hoveredNodeId ?? selectedNodeId ?? pathSummaryNodeId);
	const activePathSummaries = $derived(
		nodePathSummaries({ graph: layout, rootId: graph.rootId, targetNodeId: activeNodeId })
	);
	const activePathEdgeIds = $derived(new Set(activePathSummaries.flatMap((path) => path.edgeIds)));

	$effect(() => {
		const nextKey = `${layout.width}:${layout.height}:${visibleGraph.nodes.length}:${visibleGraph.edges.length}`;
		if (nextKey !== viewportKey) {
			viewport = initialViewport(layout.width, layout.height);
			viewportKey = nextKey;
		}
	});

	onDestroy(() => {
		clearPathSummaryHideTimer();
	});

	function setHopMode(nextMode: HopMode) {
		hopMode = nextMode;
	}

	function toggleStaleNodes() {
		showStaleNodes = !showStaleNodes;
	}

	function zoomAtCenter(factor: number) {
		viewport = zoomViewport(viewport, { x: 0.5, y: 0.5 }, factor, layout);
	}

	function resetViewport() {
		viewport = initialViewport(layout.width, layout.height);
	}

	function selectNode(nodeId: string) {
		const nextSelectedNodeId = selectedNodeId === nodeId ? null : nodeId;
		selectedNodeId = nextSelectedNodeId;
		if (nextSelectedNodeId) {
			showPathSummary(nodeId);
			return;
		}
		if (!hoveredNodeId) schedulePathSummaryHide(nodeId);
	}

	function selectNodeWithKeyboard(event: KeyboardEvent, nodeId: string) {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		selectNode(nodeId);
	}

	function handleWheel(event: WheelEvent) {
		event.preventDefault();
		if (!svgElement) return;

		const rect = svgElement.getBoundingClientRect();
		const anchor = {
			x: (event.clientX - rect.left) / rect.width,
			y: (event.clientY - rect.top) / rect.height
		};
		const factor = event.deltaY < 0 ? 1.18 : 1 / 1.18;
		viewport = zoomViewport(viewport, anchor, factor, layout);
	}

	function startPan(event: PointerEvent) {
		if (event.button !== 0) return;
		if (event.target === svgElement) clearSelectedNode();
		svgElement?.setPointerCapture(event.pointerId);
		dragStart = { clientX: event.clientX, clientY: event.clientY, viewport };
	}

	function movePan(event: PointerEvent) {
		if (!dragStart || !svgElement) return;
		const rect = svgElement.getBoundingClientRect();
		viewport = panViewport(
			dragStart.viewport,
			{ x: event.clientX - dragStart.clientX, y: event.clientY - dragStart.clientY },
			{ width: rect.width, height: rect.height }
		);
	}

	function stopPan(event: PointerEvent) {
		if (svgElement?.hasPointerCapture(event.pointerId)) {
			svgElement.releasePointerCapture(event.pointerId);
		}
		dragStart = null;
	}

	function formatDistance(km: number | undefined): string {
		if (km == null) return '';
		if (km < 1) return `${Math.round(km * 1000)} m`;
		return `${km.toFixed(1)} km`;
	}

	function formatTotalDistance(km: number | undefined): string {
		return km == null ? 'unknown' : formatDistance(km);
	}

	function labelWidth(label: string): number {
		return label.length * 7 + 14;
	}

	function isEdgeHighlighted(edgeId: string): boolean {
		return hoveredEdgeId === edgeId || activePathEdgeIds.has(edgeId);
	}

	function highlightedNodeIds(): Set<string> {
		const nodeIds = new Set<string>();

		for (const edge of layout.edges) {
			if (isEdgeHighlighted(edge.id)) {
				nodeIds.add(edge.from);
				nodeIds.add(edge.to);
			}
		}
		if (activeNodeId) nodeIds.add(activeNodeId);

		return nodeIds;
	}

	function isNodeHighlighted(nodeId: string): boolean {
		return highlightedNodeIds().has(nodeId);
	}

	function nodeCircleClass(nodeId: string): string {
		if (isNodeHighlighted(nodeId)) return 'fill-cyan-400/95 stroke-white';
		if (nodeId === graph.rootId) return 'fill-red-500/90 stroke-red-200/80';
		const node = layout.nodes.find((candidate) => candidate.id === nodeId);
		if (node?.freshness === 'direct') return 'fill-emerald-500/85 stroke-emerald-200/70';
		if (node?.freshness === 'stale') return 'fill-gray-500/80 stroke-gray-300/70';
		return 'fill-blue-500/85 stroke-blue-200/70';
	}

	function showPathSummary(nodeId: string) {
		clearPathSummaryHideTimer();
		pathSummaryNodeId = nodeId;
	}

	function handleNodeEnter(nodeId: string) {
		hoveredNodeId = nodeId;
		showPathSummary(nodeId);
	}

	function handleNodeLeave(nodeId: string) {
		if (hoveredNodeId === nodeId) hoveredNodeId = null;
		if (selectedNodeId) {
			showPathSummary(selectedNodeId);
			return;
		}
		schedulePathSummaryHide(nodeId);
	}

	function clearSelectedNode() {
		const previousSelectedNodeId = selectedNodeId;
		selectedNodeId = null;
		if (!hoveredNodeId && previousSelectedNodeId) schedulePathSummaryHide(previousSelectedNodeId);
	}

	function schedulePathSummaryHide(nodeId: string) {
		clearPathSummaryHideTimer();
		pathSummaryHideTimer = setTimeout(() => {
			if (!hoveredNodeId && !selectedNodeId && pathSummaryNodeId === nodeId) {
				pathSummaryNodeId = null;
			}
			pathSummaryHideTimer = null;
		}, PATH_SUMMARY_HIDE_DELAY_MS);
	}

	function clearPathSummaryHideTimer() {
		if (pathSummaryHideTimer === null) return;
		clearTimeout(pathSummaryHideTimer);
		pathSummaryHideTimer = null;
	}
</script>

<div class="flex h-full min-h-0 flex-col">
	<div
		class="flex h-9 shrink-0 items-center justify-between gap-3 border-b border-gray-700/60 px-3"
	>
		<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">Graph</span>
		<div class="flex items-center gap-3 font-mono text-[11px] text-gray-500">
			<span>{visibleGraph.nodes.length}/{graph.nodes.length} nodes</span>
			<span>{visibleGraph.edges.length}/{graph.edges.length} links</span>
			<span>{graph.maxDepth} hops</span>
			<div class="flex items-center gap-1">
				{#each HOP_MODES as mode}
					<button
						type="button"
						class="h-6 rounded border px-2 uppercase {hopMode === mode
							? 'border-amber-300/70 bg-amber-500/15 text-amber-100'
							: 'border-gray-700/60 text-gray-400 hover:border-blue-400/60 hover:text-blue-300'}"
						aria-pressed={hopMode === mode}
						onclick={() => setHopMode(mode)}
					>
						{mode}
					</button>
				{/each}
			</div>
			<button
				type="button"
				data-testid="graph-toggle-stale"
				class="h-6 rounded border px-2 uppercase {showStaleNodes
					? 'border-gray-500/70 bg-gray-500/15 text-gray-200'
					: 'border-gray-700/60 text-gray-500 hover:border-gray-500/70 hover:text-gray-300'}"
				aria-pressed={showStaleNodes}
				title="Toggle old nodes"
				onclick={toggleStaleNodes}
			>
				old
			</button>
			<div class="flex items-center gap-1">
				<button
					type="button"
					class="flex h-6 w-6 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-blue-400/60 hover:text-blue-300"
					title="Zoom in"
					aria-label="Zoom in"
					onclick={() => zoomAtCenter(1.22)}
				>
					<MdiIcon path={mdiMagnifyPlusOutline} size={15} />
				</button>
				<button
					type="button"
					class="flex h-6 w-6 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-blue-400/60 hover:text-blue-300"
					title="Zoom out"
					aria-label="Zoom out"
					onclick={() => zoomAtCenter(1 / 1.22)}
				>
					<MdiIcon path={mdiMagnifyMinusOutline} size={15} />
				</button>
				<button
					type="button"
					class="flex h-6 w-6 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-blue-400/60 hover:text-blue-300"
					title="Fit graph"
					aria-label="Fit graph"
					onclick={resetViewport}
				>
					<MdiIcon path={mdiFitToPageOutline} size={15} />
				</button>
			</div>
		</div>
	</div>

	<div data-testid="graph-canvas" class="relative min-h-0 flex-1 overflow-hidden bg-[#0f1520]">
		{#if graph.nodes.length <= 1}
			<div class="flex h-full items-center justify-center px-4 text-sm text-gray-500">
				No node paths available
			</div>
		{:else}
			<svg
				bind:this={svgElement}
				data-testid="graph-svg"
				class="h-full w-full cursor-grab touch-none select-none {dragStart
					? 'cursor-grabbing'
					: ''}"
				viewBox={`${viewport.x} ${viewport.y} ${viewport.width} ${viewport.height}`}
				preserveAspectRatio="xMidYMin meet"
				role="img"
				aria-label="Mesh node graph"
				onwheel={handleWheel}
				onpointerdown={startPan}
				onpointermove={movePan}
				onpointerup={stopPan}
				onpointercancel={stopPan}
			>
				<defs>
					<marker
						id="graph-arrow"
						viewBox="0 0 10 10"
						refX="8"
						refY="5"
						markerWidth="5"
						markerHeight="5"
						orient="auto-start-reverse"
					>
						<path d="M 0 0 L 10 5 L 0 10 z" class="fill-gray-600" />
					</marker>
				</defs>

				{#each layout.edges as edge (edge.id)}
					<g
						data-testid={`graph-edge-${edge.id}`}
						role="presentation"
						onpointerenter={() => (hoveredEdgeId = edge.id)}
						onpointerleave={() => (hoveredEdgeId = null)}
					>
						<path
							d={`M ${edge.fromX} ${edge.fromY + 24} C ${edge.fromX} ${(edge.fromY + edge.toY) / 2}, ${edge.toX} ${(edge.fromY + edge.toY) / 2}, ${edge.toX} ${edge.toY - 24}`}
							class="fill-none stroke-transparent"
							stroke-width="18"
						/>
						<path
							data-testid={`graph-edge-line-${edge.id}`}
							d={`M ${edge.fromX} ${edge.fromY + 24} C ${edge.fromX} ${(edge.fromY + edge.toY) / 2}, ${edge.toX} ${(edge.fromY + edge.toY) / 2}, ${edge.toX} ${edge.toY - 24}`}
							class="fill-none transition-colors {isEdgeHighlighted(edge.id)
								? 'stroke-cyan-300'
								: 'stroke-gray-600/80'}"
							stroke-width={isEdgeHighlighted(edge.id) ? 3 : 2}
							marker-end="url(#graph-arrow)"
						/>
						{#if edge.distanceKm != null && isEdgeHighlighted(edge.id)}
							{@const distanceLabel = formatDistance(edge.distanceKm)}
							{@const labelX = (edge.fromX + edge.toX) / 2}
							{@const labelY = (edge.fromY + edge.toY) / 2 - 8}
							{@const width = labelWidth(distanceLabel)}
							<rect
								x={labelX - width / 2}
								y={labelY - 13}
								{width}
								height="18"
								rx="4"
								class={isEdgeHighlighted(edge.id)
									? 'fill-cyan-950 stroke-cyan-200'
									: 'fill-[#0b1220] stroke-cyan-300/70'}
								stroke-width="1"
							/>
							<text
								x={labelX}
								y={labelY}
								text-anchor="middle"
								class={isEdgeHighlighted(edge.id)
									? 'fill-white font-mono text-[11px] font-semibold'
									: 'fill-cyan-100 font-mono text-[11px] font-semibold'}
							>
								{distanceLabel}
							</text>
						{/if}
					</g>
				{/each}

				{#each layout.nodes as node (node.id)}
					<g
						data-testid={`graph-node-group-${node.id}`}
						transform={`translate(${node.x}, ${node.y})`}
						role="button"
						tabindex="0"
						aria-label={`Select path to ${node.label}`}
						aria-pressed={selectedNodeId === node.id}
						onpointerenter={() => handleNodeEnter(node.id)}
						onpointerleave={() => handleNodeLeave(node.id)}
						onpointerdown={(event) => event.stopPropagation()}
						onclick={(event) => {
							event.stopPropagation();
							selectNode(node.id);
						}}
						onkeydown={(event) => selectNodeWithKeyboard(event, node.id)}
					>
						<circle
							data-testid={`graph-node-circle-${node.id}`}
							r={node.id === graph.rootId ? 26 : 22}
							class={nodeCircleClass(node.id)}
							stroke-width={isNodeHighlighted(node.id) ? 4 : 2}
						/>
						<text
							data-testid={`graph-node-${node.id}`}
							y={node.id === graph.rootId ? 44 : 38}
							text-anchor="middle"
							class="fill-gray-100 font-mono text-[12px] font-semibold"
						>
							{node.label}
						</text>
						<title>{node.label}</title>
					</g>
				{/each}
			</svg>
			{#if activeNodeId && activePathSummaries.length > 0}
				<div
					data-testid="graph-path-summary-panel"
					class="pointer-events-none absolute bottom-3 left-3 z-10 min-h-20 w-[min(34rem,calc(100%-1.5rem))] rounded-2xl border border-ink-dim/20 bg-base/80 px-3 py-2 font-mono text-[11px] text-ink shadow-md backdrop-blur-sm"
				>
					<div class="mb-1 flex items-center justify-between gap-3">
						<span class="text-[10px] font-semibold uppercase tracking-wider text-ink-muted"
							>Highlighted paths</span
						>
						<span class="shrink-0 text-[10px] text-ink-dim">
							{activePathSummaries.length}
							{activePathSummaries.length === 1 ? 'path' : 'paths'}
						</span>
					</div>
					<div class="mb-1 truncate text-cyan-100">{graph.rootId} -> {activeNodeId}</div>
					<div data-testid="graph-path-summary-list" class="flex flex-col gap-1 pr-1">
						{#each activePathSummaries as path, index}
							<div
								data-testid={`graph-path-summary-${index}`}
								class="flex min-w-0 items-center justify-between gap-3 rounded-lg px-1 py-0.5"
							>
								<span class="min-w-0 truncate text-cyan-300">{path.nodeIds.join(' -> ')}</span>
								<span class="shrink-0 text-ink-muted">
									total {formatTotalDistance(path.totalDistanceKm)}
								</span>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		{/if}
	</div>
</div>
