import { nodeFreshness } from './node-state';
import { calculateDistanceKm, formatDistanceKm } from './ruler';
import type { MapPosition } from './types';

const MAX_HIGHLIGHT_HOPS = 5;

export type ActiveMapPathSegment = {
	id: string;
	pathNodeIds: string[];
	from: MapPosition;
	to: MapPosition;
	distanceKm: number;
	label: string;
	highlighted: boolean;
};

export function buildActiveMapPathSegments(
	myCall: string,
	positions: MapPosition[],
	nowMs: number,
	selectedNodeId: string | null = null
): ActiveMapPathSegment[] {
	const rootId = normalizeCallsign(myCall);
	if (!rootId) return [];

	const activeBySource = activePositionMap(positions, nowMs);
	const root = activeBySource.get(rootId);
	if (!root) return [];

	const segments = new Map<string, ActiveMapPathSegment>();
	const selectedId = selectedNodeId ? normalizeCallsign(selectedNodeId) : null;

	for (const position of activeBySource.values()) {
		const source = normalizeCallsign(position.source);
		if (!source || source === rootId) continue;
		if ((position.via ?? []).length === 0 && nodeFreshness(position, nowMs) !== 'direct') continue;

		const path = compactPath([
			rootId,
			...(position.via ?? []).map(normalizeCallsign).reverse(),
			source
		]);
		for (let index = 1; index < path.length; index++) {
			const from = activeBySource.get(path[index - 1]);
			const to = activeBySource.get(path[index]);
			if (!from || !to) continue;

			const distanceKm = calculateDistanceKm(from.lat, from.lon, to.lat, to.lon);
			const id = `${from.source.toUpperCase()}->${to.source.toUpperCase()}`;
			const current = segments.get(id);
			if (current) {
				if (!current.pathNodeIds.includes(source)) current.pathNodeIds.push(source);
				continue;
			}
			segments.set(id, newSegment(id, source, from, to, distanceKm));
		}
	}

	const sortedSegments = Array.from(segments.values()).sort((left, right) =>
		left.id.localeCompare(right.id)
	);
	const highlightedEdgeIds = selectedId
		? edgeIdsReachingSelected(rootId, selectedId, sortedSegments)
		: new Set<string>();

	for (const segment of sortedSegments) {
		segment.highlighted = highlightedEdgeIds.has(segment.id);
	}

	return sortedSegments;
}

function newSegment(
	id: string,
	source: string,
	from: MapPosition,
	to: MapPosition,
	distanceKm: number
): ActiveMapPathSegment {
	return {
		id,
		pathNodeIds: [source],
		from,
		to,
		distanceKm,
		label: formatDistanceKm(distanceKm),
		highlighted: false
	};
}

function edgeIdsReachingSelected(
	rootId: string,
	selectedId: string,
	segments: ActiveMapPathSegment[]
): Set<string> {
	const highlighted = new Set<string>();
	const segmentsByNode = new Map<string, ActiveMapPathSegment[]>();

	for (const segment of segments) {
		const from = normalizeCallsign(segment.from.source);
		const to = normalizeCallsign(segment.to.source);
		appendSegment(segmentsByNode, from, segment);
		appendSegment(segmentsByNode, to, segment);
	}

	function visit(nodeId: string, visiting: Set<string>, path: string[]) {
		if (path.length > MAX_HIGHLIGHT_HOPS) return;
		if (nodeId === selectedId) {
			for (const edgeId of path) highlighted.add(edgeId);
			return;
		}
		if (visiting.has(nodeId)) return;

		visiting.add(nodeId);
		for (const segment of segmentsByNode.get(nodeId) ?? []) {
			const nextNodeId = otherEndpoint(segment, nodeId);
			if (!nextNodeId || visiting.has(nextNodeId)) continue;
			visit(nextNodeId, visiting, [...path, segment.id]);
		}
		visiting.delete(nodeId);
	}

	visit(rootId, new Set(), []);
	return highlighted;
}

function appendSegment(
	segmentsByNode: Map<string, ActiveMapPathSegment[]>,
	nodeId: string,
	segment: ActiveMapPathSegment
) {
	const segments = segmentsByNode.get(nodeId) ?? [];
	segments.push(segment);
	segmentsByNode.set(nodeId, segments);
}

function otherEndpoint(segment: ActiveMapPathSegment, nodeId: string): string | null {
	const from = normalizeCallsign(segment.from.source);
	const to = normalizeCallsign(segment.to.source);
	if (nodeId === from) return to;
	if (nodeId === to) return from;
	return null;
}

function activePositionMap(positions: MapPosition[], nowMs: number): Map<string, MapPosition> {
	const active = new Map<string, MapPosition>();
	for (const position of positions) {
		const source = normalizeCallsign(position.source);
		if (!source) continue;
		const freshness = nodeFreshness(position, nowMs);
		if (freshness !== 'direct' && freshness !== 'indirect') continue;
		if (!hasCoordinates(position)) continue;
		active.set(source, position);
	}
	return active;
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

function hasCoordinates(position: MapPosition): boolean {
	return Number.isFinite(position.lat) && Number.isFinite(position.lon);
}

function normalizeCallsign(value: string): string {
	return value.trim().toUpperCase();
}
