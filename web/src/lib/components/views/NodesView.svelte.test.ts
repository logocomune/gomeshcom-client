import { page } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

vi.mock('$env/dynamic/public', () => ({ env: {} }));

import NodesView from './NodesView.svelte';
import { connectionState } from '$lib/stores/connection.svelte';
import { eventsState } from '$lib/stores/events.svelte';
import type { MapPosition } from '$lib/map/types';

describe('NodesView', () => {
	beforeEach(() => {
		vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-05-14T19:20:00Z'));
		connectionState.stationCallsign = 'MYCALL-1';
		eventsState.events = [];
		eventsState.storedPositions = [];
	});

	afterEach(() => {
		vi.restoreAllMocks();
		connectionState.stationCallsign = '';
		eventsState.events = [];
		eventsState.storedPositions = [];
	});

	it('shows direct when last direct contact is fresh even if latest position path is relayed', async () => {
		expect.assertions(3);

		eventsState.storedPositions = [
			position('MYCALL-1', {
				lat: 45,
				lon: 10,
				lastSeen: '2026-05-14T19:20:00Z',
				lastDirectSeen: '2026-05-14T19:20:00Z'
			}),
			position('NODE-1', {
				lat: 46,
				lon: 11,
				lastSeen: '2026-05-14T19:20:00Z',
				lastDirectSeen: '2026-05-14T19:10:00Z',
				via: ['RELAY-1']
			})
		];

		render(NodesView);

		await expect.element(page.getByRole('cell', { name: 'NODE-1', exact: true })).toBeVisible();
		await expect.element(page.getByText('NODE-1 → RELAY-1')).toBeVisible();
		await expect.element(page.getByText('direct')).toBeVisible();
	});
});

function position(source: string, overrides: Partial<MapPosition> = {}): MapPosition {
	return {
		id: source,
		source,
		lat: 0,
		lon: 0,
		lastSeen: '2026-05-14T19:00:00Z',
		updatedAt: '2026-05-14T19:00:00Z',
		...overrides
	};
}
