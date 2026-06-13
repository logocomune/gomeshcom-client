import { apiFetch } from './auth';

export interface DmStatsEntry {
	sent: number;
	ack: number;
}

export type DmStatsResponse = Record<string, DmStatsEntry>;

export async function fetchDMStats(): Promise<DmStatsResponse> {
	const res = await apiFetch('/api/stats/dm');
	if (!res.ok) {
		throw new Error(`dm stats fetch failed: ${res.status}`);
	}
	return (await res.json()) as DmStatsResponse;
}
