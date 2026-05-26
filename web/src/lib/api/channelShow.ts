import { API_BASE } from './events';
import { apiFetch } from './auth';

export type ChannelShowMode = 'all' | 'allowlist';

export type ChannelShowConfig = {
	mode: ChannelShowMode;
	channels: string[];
};

export const DEFAULT_CHANNEL_SHOW: ChannelShowConfig = { mode: 'all', channels: [] };

export function isValidChannelShowChannel(value: string): boolean {
	if (value === '*') return true;
	return value.length > 0 && /^\d+$/.test(value);
}

export function isConvHidden(convId: string, cfg: ChannelShowConfig): boolean {
	if (cfg.mode !== 'allowlist') return false;
	if (convId.startsWith('DM_')) return false;
	const channelId = convId === 'P_broadcast' ? '*' : convId.replace(/^P_/, '');
	return !cfg.channels.includes(channelId);
}

export async function updateChannelShow(cfg: ChannelShowConfig): Promise<ChannelShowConfig> {
	const res = await apiFetch(`${API_BASE}/channel-show`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(cfg)
	});
	if (!res.ok) {
		const text = await res.text().catch(() => '');
		throw new Error(`channel-show update failed: ${res.status}${text ? ` — ${text}` : ''}`);
	}
	return (await res.json()) as ChannelShowConfig;
}
