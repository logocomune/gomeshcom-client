import type { MapPosition } from '$lib/map/types';
import { calculateDistanceKm } from '$lib/map/ruler';
import { nodeFreshness, type NodeFreshness } from '$lib/map/node-state';

export type NodeGraphNode = {
	id: string;
	label: string;
	depth: number;
	direct: boolean;
	freshness: NodeFreshness;
	distanceFromRootKm?: number;
};

export type NodeGraphEdge = {
	id: string;
	from: string;
	to: string;
	distanceKm?: number;
};

export type NodeGraph = {
	rootId: string;
	nodes: NodeGraphNode[];
	edges: NodeGraphEdge[];
	maxDepth: number;
};

export type BuildNodeGraphOptions = {
	nowMs?: number;
	includeHidden?: boolean;
	includeStale?: boolean;
};

export type LayoutNode = NodeGraphNode & {
	x: number;
	y: number;
};

export type LayoutEdge = NodeGraphEdge & {
	fromX: number;
	fromY: number;
	toX: number;
	toY: number;
};

export type LayoutGraph = {
	width: number;
	height: number;
	nodes: LayoutNode[];
	edges: LayoutEdge[];
};

export type NodePathSummary = {
	nodeIds: string[];
	edgeIds: string[];
	totalDistanceKm?: number;
};

export type NodePathSummaryOptions = {
	graph: LayoutGraph;
	rootId: string;
	targetNodeId: string | null;
};

export type VisibleNodeGraphOptions = {
	maxDepth: number;
	expandedNodeIds: Set<string>;
};

const DEFAULT_ROOT = 'NO-CALL';
const HORIZONTAL_GAP = 148;
const VERTICAL_GAP = 126;
const DISTANCE_PX_PER_KM = 4;
const NODE_MIN_GAP = 188;
const MARGIN_X = 80;
const MARGIN_Y = 58;

export function buildNodeGraph(
	myCallsign: string,
	positions: MapPosition[],
	options: BuildNodeGraphOptions = {}
): NodeGraph {
	const rootId = normalizeCallsign(myCallsign) || DEFAULT_ROOT;
	const nowMs = options.nowMs ?? Date.now();
	const includeHidden = options.includeHidden ?? true;
	const includeStale = options.includeStale ?? true;
	const positionsByCallsign = positionMap(positions);
	const nodes = new Map<string, NodeGraphNode>();
	const edges = new Map<string, NodeGraphEdge>();

	upsertNode(nodes, rootId, 0, true, 'direct', 0);

	for (const position of positions) {
		const source = normalizeCallsign(position.source || position.id);
		if (!source || source === rootId) continue;
		if (!includeHidden && !isVisibleGraphNode(source, positionsByCallsign, nowMs, includeStale)) {
			continue;
		}

		const relays = (position.via ?? []).map(normalizeCallsign).filter(Boolean);
		if (
			!includeHidden &&
			relays.some((relay) => !isVisibleGraphNode(relay, positionsByCallsign, nowMs, includeStale))
		) {
			continue;
		}
		const path = compactPath([rootId, ...relays.reverse(), source]);
		let cumulativeDistanceKm: number | undefined = 0;

		for (let index = 0; index < path.length; index++) {
			if (index === 0) {
				upsertNode(nodes, path[index], index, true, 'direct', cumulativeDistanceKm);
				continue;
			}

			const from = path[index - 1];
			const to = path[index];
			const edgeDistanceKm = distanceBetween(positionsByCallsign, from, to);
			cumulativeDistanceKm =
				cumulativeDistanceKm != null && edgeDistanceKm != null
					? cumulativeDistanceKm + edgeDistanceKm
					: undefined;

			upsertNode(
				nodes,
				to,
				index,
				index <= 1,
				freshnessForNode(to, positionsByCallsign, nowMs),
				cumulativeDistanceKm
			);
			upsertEdge(edges, from, to, edgeDistanceKm);
		}
	}

	const sortedNodes = Array.from(nodes.values()).sort(compareNodes);
	const sortedEdges = Array.from(edges.values()).sort((left, right) =>
		left.id.localeCompare(right.id)
	);
	const maxDepth = sortedNodes.reduce((max, node) => Math.max(max, node.depth), 0);

	return { rootId, nodes: sortedNodes, edges: sortedEdges, maxDepth };
}

export function layoutNodeGraph(graph: NodeGraph): LayoutGraph {
	const height = Math.max(260, MARGIN_Y * 2 + maxNodeY(graph.nodes));
	const layoutNodes = new Map<string, LayoutNode>();
	const xPositions = separateOverlappingNodes(graph, distributeX(graph).xByNode);
	const width = Math.max(360, xPositions.maxX + MARGIN_X);

	for (const node of graph.nodes) {
		layoutNodes.set(node.id, {
			...node,
			x: xPositions.xByNode.get(node.id) ?? width / 2,
			y: nodeY(node)
		});
	}

	const layoutEdges = graph.edges.flatMap((edge) => {
		const from = layoutNodes.get(edge.from);
		const to = layoutNodes.get(edge.to);
		if (!from || !to) return [];
		return [{ ...edge, fromX: from.x, fromY: from.y, toX: to.x, toY: to.y }];
	});

	return {
		width,
		height,
		nodes: Array.from(layoutNodes.values()).sort(compareNodes),
		edges: layoutEdges
	};
}

export function visibleNodeGraph(graph: NodeGraph, options: VisibleNodeGraphOptions): NodeGraph {
	if (options.maxDepth >= graph.maxDepth) return graph;

	const childrenByNode = childrenBySource(graph.edges);
	const nodesById = new Map(graph.nodes.map((node) => [node.id, node]));
	const visibleNodes = new Map<string, NodeGraphNode>();
	const visibleEdges = new Map<string, NodeGraphEdge>();
	const queue = [graph.rootId];
	const visited = new Set<string>();

	while (queue.length > 0) {
		const nodeId = queue.shift();
		if (!nodeId || visited.has(nodeId)) continue;
		visited.add(nodeId);

		const node = nodesById.get(nodeId);
		if (!node) continue;
		visibleNodes.set(node.id, { ...node });

		if (node.depth < options.maxDepth || options.expandedNodeIds.has(node.id)) {
			for (const childId of childrenByNode.get(nodeId) ?? []) {
				const edge = edgeBetween(graph.edges, node.id, childId);
				if (edge) visibleEdges.set(edge.id, edge);
				queue.push(childId);
			}
		}
	}

	const nodes = Array.from(visibleNodes.values()).sort(compareNodes);
	const edges = Array.from(visibleEdges.values()).sort((left, right) =>
		left.id.localeCompare(right.id)
	);
	const maxDepth = nodes.reduce((max, node) => Math.max(max, node.depth), 0);

	return { rootId: graph.rootId, nodes, edges, maxDepth };
}

export function nodePathSummaries(options: NodePathSummaryOptions): NodePathSummary[] {
	if (!options.targetNodeId || options.targetNodeId === options.rootId) return [];

	const edgesBySource = new Map<string, LayoutEdge[]>();
	for (const edge of options.graph.edges) {
		const edges = edgesBySource.get(edge.from) ?? [];
		edges.push(edge);
		edgesBySource.set(edge.from, edges);
	}
	for (const edges of edgesBySource.values()) {
		edges.sort((left, right) => left.to.localeCompare(right.to));
	}

	const summaries: NodePathSummary[] = [];
	collectNodePathSummaries(options, edgesBySource, summaries);

	return summaries.sort(comparePathSummaries);
}

function normalizeCallsign(value: string): string {
	return value.trim().toUpperCase();
}

function isVisibleGraphNode(
	nodeId: string,
	positionsByCallsign: Map<string, MapPosition>,
	nowMs: number,
	includeStale: boolean
): boolean {
	const position = positionsByCallsign.get(nodeId);
	if (!position) return false;
	const freshness = nodeFreshness(position, nowMs);
	return freshness !== 'hidden' && (includeStale || freshness !== 'stale');
}

function freshnessForNode(
	nodeId: string,
	positionsByCallsign: Map<string, MapPosition>,
	nowMs: number
): NodeFreshness {
	const position = positionsByCallsign.get(nodeId);
	return position ? nodeFreshness(position, nowMs) : 'indirect';
}

function collectNodePathSummaries(
	options: NodePathSummaryOptions,
	edgesBySource: Map<string, LayoutEdge[]>,
	summaries: NodePathSummary[]
) {
	function visit(
		nodeId: string,
		nodeIds: string[],
		edgeIds: string[],
		totalDistanceKm: number,
		hasCompleteDistance: boolean,
		visiting: Set<string>
	) {
		if (nodeId === options.targetNodeId) {
			summaries.push({
				nodeIds,
				edgeIds,
				totalDistanceKm: hasCompleteDistance ? totalDistanceKm : undefined
			});
			return;
		}
		if (visiting.has(nodeId)) return;

		const nextVisiting = new Set(visiting);
		nextVisiting.add(nodeId);

		for (const edge of edgesBySource.get(nodeId) ?? []) {
			if (nextVisiting.has(edge.to)) continue;
			const nextHasCompleteDistance = hasCompleteDistance && edge.distanceKm != null;
			const nextDistance =
				nextHasCompleteDistance && edge.distanceKm != null
					? totalDistanceKm + edge.distanceKm
					: totalDistanceKm;
			visit(
				edge.to,
				[...nodeIds, edge.to],
				[...edgeIds, edge.id],
				nextDistance,
				nextHasCompleteDistance,
				nextVisiting
			);
		}
	}

	visit(options.rootId, [options.rootId], [], 0, true, new Set());
}

function comparePathSummaries(left: NodePathSummary, right: NodePathSummary): number {
	if (left.totalDistanceKm != null && right.totalDistanceKm != null) {
		return (
			left.totalDistanceKm - right.totalDistanceKm ||
			pathLabel(left).localeCompare(pathLabel(right))
		);
	}
	if (left.totalDistanceKm != null) return -1;
	if (right.totalDistanceKm != null) return 1;
	return pathLabel(left).localeCompare(pathLabel(right));
}

function pathLabel(path: NodePathSummary): string {
	return path.nodeIds.join(' -> ');
}

function compactPath(path: string[]): string[] {
	const seen = new Set<string>();
	const compacted: string[] = [];
	for (const callsign of path) {
		if (!callsign || seen.has(callsign)) continue;
		seen.add(callsign);
		compacted.push(callsign);
	}
	return compacted;
}

function upsertNode(
	nodes: Map<string, NodeGraphNode>,
	id: string,
	depth: number,
	direct: boolean,
	freshness: NodeFreshness,
	distanceFromRootKm?: number
) {
	const current = nodes.get(id);
	if (!current) {
		nodes.set(id, { id, label: id, depth, direct, freshness, distanceFromRootKm });
		return;
	}
	current.depth = Math.min(current.depth, depth);
	current.direct = current.direct || direct;
	current.freshness = mergeFreshness(current.freshness, freshness);
	if (distanceFromRootKm != null) {
		current.distanceFromRootKm =
			current.distanceFromRootKm == null
				? distanceFromRootKm
				: Math.min(current.distanceFromRootKm, distanceFromRootKm);
	}
}

function mergeFreshness(left: NodeFreshness, right: NodeFreshness): NodeFreshness {
	const rank: Record<NodeFreshness, number> = { direct: 3, indirect: 2, stale: 1, hidden: 0 };
	return rank[right] > rank[left] ? right : left;
}

function upsertEdge(
	edges: Map<string, NodeGraphEdge>,
	from: string,
	to: string,
	distanceKm?: number
) {
	if (from === to) return;
	const id = `${from}->${to}`;
	const current = edges.get(id);
	if (!current) {
		edges.set(id, { id, from, to, distanceKm });
		return;
	}
	if (distanceKm != null) {
		current.distanceKm =
			current.distanceKm == null ? distanceKm : Math.min(current.distanceKm, distanceKm);
	}
}

function childrenBySource(edges: NodeGraphEdge[]): Map<string, string[]> {
	const childrenByNode = new Map<string, string[]>();
	for (const edge of edges) {
		const children = childrenByNode.get(edge.from) ?? [];
		children.push(edge.to);
		childrenByNode.set(edge.from, children);
	}
	for (const children of childrenByNode.values()) {
		children.sort();
	}
	return childrenByNode;
}

function edgeBetween(edges: NodeGraphEdge[], from: string, to: string): NodeGraphEdge | undefined {
	return edges.find((edge) => edge.from === from && edge.to === to);
}

function positionMap(positions: MapPosition[]): Map<string, MapPosition> {
	const map = new Map<string, MapPosition>();
	for (const position of positions) {
		const source = normalizeCallsign(position.source || position.id);
		if (source) map.set(source, position);
	}
	return map;
}

function distanceBetween(
	positions: Map<string, MapPosition>,
	from: string,
	to: string
): number | undefined {
	const fromPosition = positions.get(from);
	const toPosition = positions.get(to);
	if (!hasCoordinates(fromPosition) || !hasCoordinates(toPosition)) return undefined;
	return calculateDistanceKm(fromPosition.lat, fromPosition.lon, toPosition.lat, toPosition.lon);
}

function hasCoordinates(position: MapPosition | undefined): position is MapPosition {
	return position != null && Number.isFinite(position.lat) && Number.isFinite(position.lon);
}

function nodeY(node: NodeGraphNode): number {
	const hopY = node.depth * VERTICAL_GAP;
	const distanceY =
		node.distanceFromRootKm != null ? node.distanceFromRootKm * DISTANCE_PX_PER_KM : 0;
	return MARGIN_Y + hopY + distanceY;
}

function maxNodeY(nodes: NodeGraphNode[]): number {
	return nodes.reduce((max, node) => Math.max(max, nodeY(node) - MARGIN_Y), 0);
}

function compareNodes(left: NodeGraphNode, right: NodeGraphNode): number {
	return left.depth - right.depth || left.label.localeCompare(right.label);
}

function separateOverlappingNodes(
	graph: NodeGraph,
	xByNode: Map<string, number>
): { xByNode: Map<string, number>; maxX: number } {
	const adjusted = new Map(xByNode);
	const nodesByDepth = new Map<number, NodeGraphNode[]>();

	for (const node of graph.nodes) {
		const nodes = nodesByDepth.get(node.depth) ?? [];
		nodes.push(node);
		nodesByDepth.set(node.depth, nodes);
	}

	for (const nodes of nodesByDepth.values()) {
		nodes.sort((left, right) => (adjusted.get(left.id) ?? 0) - (adjusted.get(right.id) ?? 0));

		let previous: NodeGraphNode | undefined;
		for (const node of nodes) {
			if (!previous) {
				previous = node;
				continue;
			}

			const previousX = adjusted.get(previous.id) ?? MARGIN_X;
			const currentX = adjusted.get(node.id) ?? MARGIN_X;
			const minGap = minGapBetween(previous, node);
			if (currentX - previousX < minGap) {
				adjusted.set(node.id, previousX + minGap);
			}
			previous = node;
		}
	}

	const maxX = Math.max(MARGIN_X, ...Array.from(adjusted.values()));
	return { xByNode: adjusted, maxX };
}

function minGapBetween(left: NodeGraphNode, right: NodeGraphNode): number {
	const labelGap = (estimatedLabelWidth(left.label) + estimatedLabelWidth(right.label)) / 2 + 42;
	return Math.max(NODE_MIN_GAP, labelGap);
}

function estimatedLabelWidth(label: string): number {
	return label.length * 8 + 20;
}

function distributeX(graph: NodeGraph): { xByNode: Map<string, number>; leafCount: number } {
	const childrenByNode = childrenBySource(graph.edges);
	const xByNode = new Map<string, number>();
	let leafIndex = 0;

	function place(nodeId: string, visiting: Set<string>): number {
		const existing = xByNode.get(nodeId);
		if (existing != null) return existing;
		if (visiting.has(nodeId)) return nextLeafX();

		visiting.add(nodeId);
		const children = childrenByNode.get(nodeId) ?? [];
		if (children.length === 0) {
			const x = nextLeafX();
			xByNode.set(nodeId, x);
			visiting.delete(nodeId);
			return x;
		}

		const childXs = children.map((child) => place(child, visiting));
		const x = childXs.reduce((sum, childX) => sum + childX, 0) / childXs.length;
		xByNode.set(nodeId, x);
		visiting.delete(nodeId);
		return x;
	}

	function nextLeafX(): number {
		const x = MARGIN_X + leafIndex * HORIZONTAL_GAP;
		leafIndex += 1;
		return x;
	}

	place(graph.rootId, new Set());
	for (const node of graph.nodes) {
		place(node.id, new Set());
	}

	return { xByNode, leafCount: Math.max(1, leafIndex) };
}
