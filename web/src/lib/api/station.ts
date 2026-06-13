import { API_BASE } from './events';
import { apiFetch } from './auth';
import type { StationIdentity } from './types';

export async function getMyCall(): Promise<StationIdentity> {
	const res = await apiFetch(`${API_BASE}/adm/configs/my-call`);
	if (!res.ok) {
		const text = await res.text().catch(() => '');
		throw new Error(`get my-call failed: ${res.status}${text ? ` — ${text}` : ''}`);
	}
	return (await res.json()) as StationIdentity;
}

export async function updateMyCall(callsign: string): Promise<StationIdentity> {
	const res = await apiFetch(`${API_BASE}/adm/configs/my-call`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ callsign })
	});
	if (!res.ok) {
		const text = await res.text().catch(() => '');
		throw new Error(`my-call update failed: ${res.status}${text ? ` — ${text}` : ''}`);
	}
	return (await res.json()) as StationIdentity;
}
