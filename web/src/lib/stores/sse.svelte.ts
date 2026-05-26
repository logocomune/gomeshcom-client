import {
	connectEvents,
	isReplayEvent,
	packetFromEvent,
	splitSourcePath,
	stationCallsignFromEvent,
	type ConnectEventsOptions
} from '$lib/api/events';
import type {
	ChannelShowConfig,
	ChatStatusSnapshot,
	ConnectionState,
	StationIdentity,
	StreamEvent
} from '$lib/api/types';
import type { MapPosition } from '$lib/map/types';
import { chatState } from '$lib/stores/chat.svelte';
import { connectionState } from '$lib/stores/connection.svelte';
import { eventsState } from '$lib/stores/events.svelte';

type EventHandlers = {
	onState: (state: ConnectionState) => void;
	onEvent: (event: StreamEvent) => void;
	onPositions?: (positions: MapPosition[]) => void;
	onStation?: (station: StationIdentity) => void;
	onChatStatus?: (snapshot: ChatStatusSnapshot) => void;
	onChannelShow?: (cfg: ChannelShowConfig) => void;
};

type ConnectEvents = (handlers: EventHandlers, options?: ConnectEventsOptions) => () => void;

export type SseStore = ReturnType<typeof createSseStore>;

export function createSseStore(
	callbacks: { clearSendEcho: () => void },
	connect: ConnectEvents = connectEvents
) {
	let stopStream: (() => void) | null = null;
	let active = $state(false);

	function connectStream() {
		if (stopStream !== null) return;

		const replayFrom = eventsState.replayFrom();
		stopStream = connect(createHandlers(callbacks), replayFrom ? { replayFrom } : {});
		active = true;
	}

	function disconnect() {
		stopStream?.();
		stopStream = null;
		active = false;
	}

	function restart() {
		disconnect();
		connectStream();
	}

	return {
		get active() {
			return active;
		},
		connect: connectStream,
		disconnect,
		restart
	};
}

function createHandlers(callbacks: { clearSendEcho: () => void }): EventHandlers {
	return {
		onState: (state) => {
			connectionState.setState(state);
		},
		onPositions: (positions) => {
			eventsState.storedPositions = positions;
		},
		onStation: (station) => {
			connectionState.setStation(station);
		},
		onChatStatus: (snapshot) => {
			chatState.setChatStatus(snapshot);
		},
		onChannelShow: (cfg) => {
			chatState.setChannelShow(cfg);
		},
		onEvent: (event) => {
			eventsState.prependEvent(event);
			updateStationFromEvent(event);
			appendLiveChat(event, callbacks.clearSendEcho);
		}
	};
}

function updateStationFromEvent(event: StreamEvent) {
	if (connectionState.stationCallsign) return;
	const callsign = stationCallsignFromEvent(event);
	if (callsign) connectionState.stationCallsign = callsign;
}

function appendLiveChat(event: StreamEvent, clearSendEcho: () => void) {
	const packet = packetFromEvent(event);
	if (packet?.type === 'msg' && !isReplayEvent(event)) {
		chatState.appendLiveChatRecord(packet, event.receivedAt);
		if (
			chatState.sending &&
			splitSourcePath(packet.src).origin === connectionState.stationCallsign
		) {
			clearSendEcho();
		}
	}

	if (event.type === 'message.failed') {
		chatState.appendChatRecord(event.data as import('$lib/api/types').ChatRecord);
		clearSendEcho();
	}
}
