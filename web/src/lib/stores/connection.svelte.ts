import type { ConnectionState, StationIdentity } from '$lib/api/types';
import { stationStore } from '$lib/stores/station';

class Connection {
	state = $state<ConnectionState>('connecting');
	stationCallsign = $state('');
	appVersion = $state('');
	txDisabled = $state(false);

	setState(s: ConnectionState) {
		this.state = s;
	}

	setStation(station: StationIdentity) {
		this.stationCallsign = station.callsign;
		if (station.version) this.appVersion = station.version;
		this.txDisabled = !!station.txDisabled;
		stationStore.set(station);
	}
}

export const connectionState = new Connection();
