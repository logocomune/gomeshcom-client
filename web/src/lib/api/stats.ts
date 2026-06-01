import { apiFetch } from './auth';

export interface Bucket {
	hour: number; // Unix timestamp truncated to UTC hour
	dm: number;
	dm_ack: number;
	public: number;
	telemetry: number;
	position: number;
	errors: number;
	total: number;
	distance_km?: Record<string, number>;
	channels?: Record<string, number>;
}

export interface StatsResponse {
	from: string;
	to: string;
	hours: number;
	buckets: Bucket[];
}

export async function fetchStats(hours = 24): Promise<StatsResponse> {
	const res = await apiFetch(`/api/stats?hours=${hours}`);
	if (!res.ok) {
		throw new Error(`stats fetch failed: ${res.status}`);
	}
	return (await res.json()) as StatsResponse;
}
