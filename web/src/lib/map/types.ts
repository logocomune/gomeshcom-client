export type MapPosition = {
	id: string;
	source: string;
	lat: number;
	lon: number;
	altitude?: number;
	battery?: number;
	rssi?: number;
	snr?: number;
	hwId?: string;
	firstSeen?: string;
	lastSeen?: string;
	lastDirectSeen?: string;
	via?: string[];
	updatedAt: string;
};
