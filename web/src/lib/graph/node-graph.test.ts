import { describe, expect, it } from 'vitest';
import fc from 'fast-check';
import { buildNodeGraph, layoutNodeGraph, nodePathSummaries, visibleNodeGraph } from './node-graph';
import type { MapPosition } from '$lib/map/types';

describe('buildNodeGraph', () => {
	it('builds direct child from position without relays', () => {
		const graph = buildNodeGraph('MYCALL-1', [
			position('MYCALL-1', [], 45, 12),
			position('NODE-1', [], 45.1, 12)
		]);

		expect(graph.nodes.map((node) => node.id)).toEqual(['MYCALL-1', 'NODE-1']);
		expect(graph.edges[0]).toMatchObject({
			id: 'MYCALL-1->NODE-1',
			from: 'MYCALL-1',
			to: 'NODE-1'
		});
		expect(graph.edges[0].distanceKm).toBeGreaterThan(11);
		expect(graph.maxDepth).toBe(1);
	});

	it('builds inbound relay path from last hop down to origin', () => {
		const graph = buildNodeGraph('MYCALL-1', [
			position('MYCALL-1', [], 45, 12),
			position('R2', [], 45.1, 12),
			position('R1', [], 45.2, 12),
			position('ORIGIN-1', ['R1', 'R2'], 45.3, 12)
		]);

		expect(graph.edges.map((edge) => edge.id)).toEqual([
			'MYCALL-1->R1',
			'MYCALL-1->R2',
			'R1->ORIGIN-1',
			'R2->R1'
		]);
		expect(graph.nodes.find((node) => node.id === 'R2')).toMatchObject({ depth: 1, direct: true });
		expect(graph.nodes.find((node) => node.id === 'ORIGIN-1')).toMatchObject({
			depth: 3,
			direct: false
		});
		expect(graph.nodes.find((node) => node.id === 'ORIGIN-1')?.distanceFromRootKm).toBeGreaterThan(
			30
		);
	});

	it('can filter hidden nodes and preserve Nodes view freshness states', () => {
		const nowMs = Date.parse('2026-05-20T12:00:00Z');
		const graph = buildNodeGraph(
			'MYCALL-1',
			[
				position('DIRECT-1', [], 45, 12, {
					lastSeen: new Date(nowMs - 10 * 60 * 1000).toISOString(),
					lastDirectSeen: new Date(nowMs - 10 * 60 * 1000).toISOString()
				}),
				position('NORMAL-1', [], 45.1, 12, {
					lastSeen: new Date(nowMs - 45 * 60 * 1000).toISOString()
				}),
				position('OLD-1', [], 45.2, 12, {
					lastSeen: new Date(nowMs - 2 * 60 * 60 * 1000).toISOString()
				}),
				position('HIDDEN-1', [], 45.3, 12, {
					lastSeen: new Date(nowMs - 49 * 60 * 60 * 1000).toISOString()
				})
			],
			{ nowMs, includeHidden: false }
		);

		expect(graph.nodes.map((node) => node.id)).toEqual([
			'MYCALL-1',
			'DIRECT-1',
			'NORMAL-1',
			'OLD-1'
		]);
		expect(graph.nodes.find((node) => node.id === 'DIRECT-1')?.freshness).toBe('direct');
		expect(graph.nodes.find((node) => node.id === 'NORMAL-1')?.freshness).toBe('indirect');
		expect(graph.nodes.find((node) => node.id === 'OLD-1')?.freshness).toBe('stale');
	});

	it('can hide stale graph nodes when old nodes are disabled', () => {
		const nowMs = Date.parse('2026-05-20T12:00:00Z');
		const graph = buildNodeGraph(
			'MYCALL-1',
			[
				position('NORMAL-1', [], 45.1, 12, {
					lastSeen: new Date(nowMs - 45 * 60 * 1000).toISOString()
				}),
				position('OLD-1', [], 45.2, 12, {
					lastSeen: new Date(nowMs - 2 * 60 * 60 * 1000).toISOString()
				})
			],
			{ nowMs, includeHidden: false, includeStale: false }
		);

		expect(graph.nodes.map((node) => node.id)).toEqual(['MYCALL-1', 'NORMAL-1']);
	});

	it('deduplicates shared relay edges', () => {
		const graph = buildNodeGraph('MYCALL-1', [
			position('NODE-1', ['RELAY-1']),
			position('NODE-2', ['RELAY-1'])
		]);

		expect(graph.edges.filter((edge) => edge.id === 'MYCALL-1->RELAY-1')).toHaveLength(1);
		expect(graph.nodes.find((node) => node.id === 'RELAY-1')).toMatchObject({
			depth: 1,
			direct: true
		});
	});

	it('lays out every edge endpoint on an existing node', () => {
		const graph = buildNodeGraph('MYCALL-1', [
			position('MYCALL-1', [], 45, 12),
			position('NODE-1', [], 45.1, 12),
			position('ORIGIN-1', ['R1', 'R2'], 46, 12)
		]);
		const layout = layoutNodeGraph(graph);
		const nodeIds = new Set(layout.nodes.map((node) => node.id));

		expect(layout.width).toBeGreaterThan(0);
		expect(layout.height).toBeGreaterThan(0);
		expect(layout.edges.every((edge) => nodeIds.has(edge.from) && nodeIds.has(edge.to))).toBe(true);
	});

	it('uses cumulative distance to push far nodes farther from root', () => {
		const graph = buildNodeGraph('MYCALL-1', [
			position('MYCALL-1', [], 45, 12),
			position('NEAR-1', [], 45.01, 12),
			position('FAR-1', [], 47, 12)
		]);
		const layout = layoutNodeGraph(graph);
		const near = layout.nodes.find((node) => node.id === 'NEAR-1');
		const far = layout.nodes.find((node) => node.id === 'FAR-1');

		expect(far?.y).toBeGreaterThan(near?.y ?? 0);
	});

	it('keeps direct edge spacing proportional to measured distance', () => {
		const graph = buildNodeGraph('MYCALL-1', [
			position('MYCALL-1', [], 45, 12),
			position('NEAR-1', [], 45.01, 12),
			position('FAR-1', [], 47, 12)
		]);
		const layout = layoutNodeGraph(graph);
		const root = layout.nodes.find((node) => node.id === 'MYCALL-1');
		const near = layout.nodes.find((node) => node.id === 'NEAR-1');
		const far = layout.nodes.find((node) => node.id === 'FAR-1');

		expect((far?.y ?? 0) - (root?.y ?? 0)).toBeGreaterThan(((near?.y ?? 0) - (root?.y ?? 0)) * 2);
	});

	it('distributes sibling subtrees across horizontal space', () => {
		const graph = buildNodeGraph('MYCALL-1', [position('LEFT-1'), position('RIGHT-1')]);
		const layout = layoutNodeGraph(graph);
		const left = layout.nodes.find((node) => node.id === 'LEFT-1');
		const right = layout.nodes.find((node) => node.id === 'RIGHT-1');

		expect(left?.x).not.toBe(right?.x);
	});

	it('keeps first-hop nodes separated enough to avoid overlap', () => {
		const graph = buildNodeGraph(
			'MYCALL-1',
			Array.from({ length: 8 }, (_, index) => position(`NODE-${index}`))
		);
		const layout = layoutNodeGraph(graph);
		const firstHopNodes = layout.nodes
			.filter((node) => node.depth === 1)
			.sort((left, right) => left.x - right.x);

		for (let index = 1; index < firstHopNodes.length; index++) {
			expect(firstHopNodes[index].x - firstHopNodes[index - 1].x).toBeGreaterThanOrEqual(188);
		}
	});

	it('limits wide deep branches without aggregate nodes', () => {
		const graph = buildNodeGraph(
			'MYCALL-1',
			Array.from({ length: 50 }, (_, index) => position(`NODE-${index}`, ['R1', 'R2']))
		);
		const visibleGraph = visibleNodeGraph(graph, { maxDepth: 2, expandedNodeIds: new Set() });
		const layout = layoutNodeGraph(visibleGraph);

		expect(visibleGraph.nodes.map((node) => node.id)).toEqual(['MYCALL-1', 'R2', 'R1']);
		expect(visibleGraph.nodes.some((node) => node.id.endsWith('::more'))).toBe(false);
		expect(layout.width).toBeLessThanOrEqual(620);
	});

	it('expands one hidden branch to the next depth', () => {
		const graph = buildNodeGraph('MYCALL-1', [
			position('NODE-1', ['R1', 'R2']),
			position('DEEP-1', ['NODE-1', 'R1', 'R2'])
		]);
		const visibleGraph = visibleNodeGraph(graph, { maxDepth: 2, expandedNodeIds: new Set(['R1']) });

		expect(visibleGraph.nodes.map((node) => node.id)).toEqual(['MYCALL-1', 'R2', 'R1', 'NODE-1']);
		expect(visibleGraph.nodes.some((node) => node.id.endsWith('::more'))).toBe(false);
	});

	it('never emits duplicate edges', () => {
		fc.assert(
			fc.property(
				fc.array(
					fc.record({
						source: callsignArbitrary(),
						via: fc.array(callsignArbitrary(), { maxLength: 4 })
					}),
					{ maxLength: 30 }
				),
				(records) => {
					const graph = buildNodeGraph(
						'MYCALL-1',
						records.map((record) => position(record.source, record.via))
					);
					const ids = graph.edges.map((edge) => edge.id);
					expect(new Set(ids).size).toBe(ids.length);
				}
			),
			{ numRuns: 100 }
		);
	});

	it('summarizes every path from root to a selected node with total distance', () => {
		const graph = buildNodeGraph('MYCALL-1', [
			position('MYCALL-1', [], 45, 12),
			position('R1', [], 45.1, 12),
			position('R2', [], 45.2, 12),
			position('TARGET-1', ['R1'], 45.3, 12),
			position('TARGET-1', ['R2'], 45.3, 12)
		]);
		const layout = layoutNodeGraph(graph);

		const paths = nodePathSummaries({
			graph: layout,
			rootId: graph.rootId,
			targetNodeId: 'TARGET-1'
		});

		expect(paths.map((path) => path.nodeIds.join(' -> '))).toEqual([
			'MYCALL-1 -> R1 -> TARGET-1',
			'MYCALL-1 -> R2 -> TARGET-1'
		]);
		expect(paths.every((path) => path.totalDistanceKm != null)).toBe(true);
		expect(paths.every((path) => (path.totalDistanceKm ?? 0) > 0)).toBe(true);
	});
});

function position(
	source: string,
	via: string[] = [],
	lat = 45,
	lon = 12,
	overrides: Partial<MapPosition> = {}
): MapPosition {
	return {
		id: source,
		source,
		lat,
		lon,
		updatedAt: '2026-05-20T10:00:00Z',
		lastSeen: '2026-05-20T10:00:00Z',
		via,
		...overrides
	};
}

function callsignArbitrary() {
	return fc
		.tuple(
			fc.string({
				minLength: 3,
				maxLength: 6,
				unit: fc.constantFrom(...'ABCDEFGHIJKLMNOPQRSTUVWXYZ')
			}),
			fc.integer({ min: 0, max: 15 })
		)
		.map(([base, ssid]) => `${base}-${ssid}`);
}
