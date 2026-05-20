import { nodeFreshness } from './node-state';
import type { MapPosition } from './types';

export type RulerLink = {
	from: MapPosition;
	to: MapPosition;
	distanceKm: number;
	label: string;
};

export function calculateDistanceKm(
	lat1: number,
	lon1: number,
	lat2: number,
	lon2: number
): number {
	const earthRadiusKm = 6371;
	const dLat = (lat2 - lat1) * (Math.PI / 180);
	const dLon = (lon2 - lon1) * (Math.PI / 180);
	const a =
		Math.sin(dLat / 2) * Math.sin(dLat / 2) +
		Math.cos(lat1 * (Math.PI / 180)) *
			Math.cos(lat2 * (Math.PI / 180)) *
			Math.sin(dLon / 2) *
			Math.sin(dLon / 2);
	const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
	return earthRadiusKm * c;
}

export function formatDistanceKm(distanceKm: number): string {
	return `${distanceKm.toFixed(1)}km`;
}

export function buildRulerLinks(
	myCallPosition: MapPosition | null,
	positions: MapPosition[],
	nowMs: number
): RulerLink[] {
	if (!myCallPosition) return [];
	const myCall = myCallPosition.source.toUpperCase();
	return positions
		.filter((position) => position.source.toUpperCase() !== myCall)
		.filter((position) => nodeFreshness(position, nowMs) === 'direct')
		.map((position) => {
			const distanceKm = calculateDistanceKm(
				myCallPosition.lat,
				myCallPosition.lon,
				position.lat,
				position.lon
			);
			return {
				from: myCallPosition,
				to: position,
				distanceKm,
				label: formatDistanceKm(distanceKm)
			};
		});
}
