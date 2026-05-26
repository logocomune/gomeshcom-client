import type { StreamEvent } from '$lib/api/types';
import type { MapPosition } from '$lib/map/types';
import { applyLiveFreshness, prependEvent as doPrepend } from '$lib/api/events';
import { filterStreamEvents } from '$lib/ui/stream';

const DEFAULT_STREAM_HEIGHT = 300;
const STORAGE_STREAM_HEIGHT = 'meshcom:streamHeightPx';
const STORAGE_STREAM_REPLAY_FROM = 'meshcom:streamReplayFrom';

export type MapFocusTarget = { callsign: string; lat: number; lng: number; ts: number };

class EventsStore {
	events = $state<StreamEvent[]>([]);
	storedPositions = $state<MapPosition[]>([]);
	streamFilter = $state('');
	selectedEvent = $state<StreamEvent | null>(null);
	rawEvent = $state<StreamEvent | null>(null);
	streamHeightPx = $state(DEFAULT_STREAM_HEIGHT);
	mapFocusTarget = $state<MapFocusTarget | null>(null);

	mapPositions = $derived(applyLiveFreshness(this.storedPositions, this.events));
	filteredEvents = $derived(filterStreamEvents(this.events, this.streamFilter));

	loadLayout() {
		const h = parseFloat(localStorage.getItem(STORAGE_STREAM_HEIGHT) ?? '');
		if (!isNaN(h) && h >= 160) this.streamHeightPx = h;
	}

	saveStreamHeight() {
		localStorage.setItem(STORAGE_STREAM_HEIGHT, String(this.streamHeightPx));
	}

	replayFrom(): string | undefined {
		return localStorage.getItem(STORAGE_STREAM_REPLAY_FROM) ?? undefined;
	}

	prependEvent(event: StreamEvent) {
		this.events = doPrepend(this.events, event);
		this.selectedEvent ??= event;
	}

	focusOnNode(callsign: string, lat: number, lng: number) {
		this.mapFocusTarget = { callsign, lat, lng, ts: Date.now() };
	}

	clear() {
		this.events = [];
		this.selectedEvent = null;
		this.rawEvent = null;
	}

	clearAndSaveReplayCursor() {
		localStorage.setItem(STORAGE_STREAM_REPLAY_FROM, new Date().toISOString());
		this.clear();
	}
}

export const eventsState = new EventsStore();
