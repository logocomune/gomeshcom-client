import { page } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { tick } from 'svelte';

vi.mock('$env/dynamic/public', () => ({ env: {} }));

import GraphView from './GraphView.svelte';
import { connectionState } from '$lib/stores/connection.svelte';
import { eventsState } from '$lib/stores/events.svelte';
import type { MapPosition } from '$lib/map/types';

describe('GraphView', () => {
	beforeEach(() => {
		connectionState.stationCallsign = 'MYCALL-1';
		eventsState.events = [];
		eventsState.storedPositions = [];
	});

	afterEach(() => {
		vi.useRealTimers();
		connectionState.stationCallsign = '';
		eventsState.events = [];
		eventsState.storedPositions = [];
	});

	it('renders empty state when no node paths exist', async () => {
		expect.assertions(1);

		render(GraphView);

		await expect.element(page.getByText('No node paths available')).toBeVisible();
	});

	it('renders relayed paths when relay nodes are displayable', async () => {
		expect.assertions(6);

		eventsState.storedPositions = [
			position('R2', [], 45.1, 12),
			position('R1', [], 45.2, 12),
			position('ORIGIN-1', ['R1', 'R2'], 45.3, 12)
		];

		render(GraphView);

		await expect.element(page.getByTestId('graph-node-MYCALL-1')).toBeVisible();
		await expect.element(page.getByTestId('graph-node-R2')).toBeVisible();
		await expect.element(page.getByTestId('graph-node-R1')).toBeVisible();
		await expect.element(page.getByTestId('graph-node-ORIGIN-1')).toBeVisible();
		await expect.element(page.getByText('4/4 nodes')).toBeVisible();
		expect(hasSvgText(/11\.\d km/)).toBe(false);
	});

	it('keeps svg resized inside graph canvas', async () => {
		expect.assertions(3);

		eventsState.storedPositions = [
			position('MYCALL-1', [], 45, 12),
			...Array.from({ length: 18 }, (_, index) =>
				position(`NODE-${index}`, [], 45 + index * 0.03, 12)
			)
		];

		render(GraphView);

		await expect.element(page.getByTestId('graph-svg')).toBeVisible();
		const canvasBox = elementBox('[data-testid="graph-canvas"]');
		const svgBox = elementBox('[data-testid="graph-svg"]');

		expect(svgBox.right).toBeLessThanOrEqual(canvasBox.right);
		expect(svgBox.bottom).toBeLessThanOrEqual(canvasBox.bottom);
	});

	it('highlights edge on pointer hover', async () => {
		expect.assertions(3);

		eventsState.storedPositions = [
			position('MYCALL-1', [], 45, 12),
			position('NODE-1', [], 45.1, 12)
		];

		render(GraphView);

		await expect.element(page.getByTestId('graph-node-NODE-1')).toBeVisible();
		const edge = requireElement('[data-testid="graph-edge-MYCALL-1->NODE-1"]');
		const line = requireElement('[data-testid="graph-edge-line-MYCALL-1->NODE-1"]');

		edge.dispatchEvent(new PointerEvent('pointerenter', { bubbles: true }));
		await tick();

		expect(line.getAttribute('class')).toContain('stroke-cyan-300');
		expect(hasSvgText(/11\.\d km/)).toBe(true);
	});

	it('highlights all root paths when hovering a node', async () => {
		expect.assertions(8);

		eventsState.storedPositions = [
			position('R2', [], 45.1, 12),
			position('R1', [], 45.2, 12),
			position('ORIGIN-1', ['R1', 'R2'], 45.3, 12)
		];

		render(GraphView);
		clickHopMode('all');
		await tick();

		await expect.element(page.getByTestId('graph-node-group-ORIGIN-1')).toBeVisible();
		const node = requireElement('[data-testid="graph-node-group-ORIGIN-1"]');

		node.dispatchEvent(new PointerEvent('pointerenter', { bubbles: false }));
		await tick();

		expect(edgeLineClass('MYCALL-1->R2')).toContain('stroke-cyan-300');
		expect(edgeLineClass('R2->R1')).toContain('stroke-cyan-300');
		expect(edgeLineClass('R1->ORIGIN-1')).toContain('stroke-cyan-300');
		expect(nodeCircleClass('MYCALL-1')).toContain('fill-cyan-400');
		expect(nodeCircleClass('R2')).toContain('fill-cyan-400');
		expect(nodeCircleClass('R1')).toContain('fill-cyan-400');
		expect(nodeCircleClass('ORIGIN-1')).toContain('fill-cyan-400');
	});

	it('shows a selected path summary overlay with total distance', async () => {
		expect.assertions(7);

		eventsState.storedPositions = [
			position('MYCALL-1', [], 45, 12),
			position('R2', [], 45.1, 12),
			position('R1', [], 45.2, 12),
			position('ORIGIN-1', ['R1', 'R2'], 45.3, 12)
		];

		render(GraphView);
		clickHopMode('all');
		await tick();

		await expect.element(page.getByTestId('graph-node-group-ORIGIN-1')).toBeVisible();
		requireElement('[data-testid="graph-node-group-ORIGIN-1"]').dispatchEvent(
			new MouseEvent('click', { bubbles: true })
		);
		await tick();

		await expect.element(page.getByTestId('graph-path-summary-panel')).toBeVisible();
		await expect.element(page.getByText('MYCALL-1 -> ORIGIN-1')).toBeVisible();
		await expect.element(page.getByText('MYCALL-1 -> R2 -> R1 -> ORIGIN-1')).toBeVisible();
		expect(document.querySelector('[data-testid="graph-path-summary-0"]')?.textContent).toContain(
			'total '
		);
		expect(requireElement('[data-testid="graph-path-summary-list"]').className).not.toContain(
			'overflow-y-auto'
		);
		expect(edgeLineClass('R1->ORIGIN-1')).toContain('stroke-cyan-300');
	});

	it('keeps path summary visible for seven seconds and updates when another node activates', async () => {
		expect.assertions(6);
		vi.useFakeTimers();

		eventsState.storedPositions = [
			position('MYCALL-1', [], 45, 12),
			position('NODE-1', [], 45.1, 12),
			position('NODE-2', [], 45.2, 12)
		];

		render(GraphView);

		await expect.element(page.getByTestId('graph-node-group-NODE-1')).toBeVisible();
		const firstNode = requireElement('[data-testid="graph-node-group-NODE-1"]');
		firstNode.dispatchEvent(new PointerEvent('pointerenter', { bubbles: false }));
		await tick();

		await expect.element(page.getByTestId('graph-path-summary-panel')).toBeVisible();
		expect(
			document.querySelector('[data-testid="graph-path-summary-panel"]')?.textContent
		).toContain('MYCALL-1 -> NODE-1');
		firstNode.dispatchEvent(new PointerEvent('pointerleave', { bubbles: false }));
		vi.advanceTimersByTime(6999);
		await tick();

		expect(document.querySelector('[data-testid="graph-path-summary-panel"]')).not.toBeNull();

		const secondNode = requireElement('[data-testid="graph-node-group-NODE-2"]');
		secondNode.dispatchEvent(new PointerEvent('pointerenter', { bubbles: false }));
		await tick();

		expect(
			document.querySelector('[data-testid="graph-path-summary-panel"]')?.textContent
		).toContain('MYCALL-1 -> NODE-2');
		secondNode.dispatchEvent(new PointerEvent('pointerleave', { bubbles: false }));
		vi.advanceTimersByTime(7000);
		await tick();

		expect(document.querySelector('[data-testid="graph-path-summary-panel"]')).toBeNull();
	});

	it('uses Nodes view freshness colors and can hide old grey nodes', async () => {
		expect.assertions(6);
		const nowMs = Date.now();

		eventsState.storedPositions = [
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
		];

		render(GraphView);

		await expect.element(page.getByTestId('graph-node-DIRECT-1')).toBeVisible();
		expect(nodeCircleClass('DIRECT-1')).toContain('fill-emerald-500');
		expect(nodeCircleClass('NORMAL-1')).toContain('fill-blue-500');
		expect(nodeCircleClass('OLD-1')).toContain('fill-gray-500');
		expect(document.querySelector('[data-testid="graph-node-HIDDEN-1"]')).toBeNull();

		requireElement('[data-testid="graph-toggle-stale"]').dispatchEvent(
			new MouseEvent('click', { bubbles: true })
		);
		await tick();

		expect(document.querySelector('[data-testid="graph-node-OLD-1"]')).toBeNull();
	});
});

function position(
	source: string,
	via: string[] = [],
	lat = 45,
	lon = 12,
	overrides: Partial<MapPosition> = {}
): MapPosition {
	const now = new Date().toISOString();
	return {
		id: source,
		source,
		lat,
		lon,
		updatedAt: now,
		lastSeen: now,
		via,
		...overrides
	};
}

function elementBox(selector: string): DOMRect {
	return requireElement(selector).getBoundingClientRect();
}

function hasSvgText(pattern: RegExp): boolean {
	return Array.from(document.querySelectorAll('svg text')).some((element) =>
		pattern.test(element.textContent ?? '')
	);
}

function requireElement(selector: string): HTMLElement | SVGElement {
	const element = document.querySelector(selector);
	if (!(element instanceof HTMLElement || element instanceof SVGElement)) {
		throw new Error(`missing element: ${selector}`);
	}
	return element;
}

function clickHopMode(mode: string) {
	const button = Array.from(document.querySelectorAll('button')).find(
		(element) => element.textContent?.trim() === mode
	);
	if (!(button instanceof HTMLButtonElement)) {
		throw new Error(`missing hop mode: ${mode}`);
	}
	button.click();
}

function edgeLineClass(edgeId: string): string | null {
	return requireElement(`[data-testid="graph-edge-line-${edgeId}"]`).getAttribute('class');
}

function nodeCircleClass(nodeId: string): string | null {
	return requireElement(`[data-testid="graph-node-circle-${nodeId}"]`).getAttribute('class');
}
