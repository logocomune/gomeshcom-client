import type { MapPosition } from './types';

export type NodeFreshness = 'direct' | 'indirect' | 'stale' | 'hidden';

const DIRECT_WINDOW_MS = 30 * 60 * 1000;
const INDIRECT_WINDOW_MS = 60 * 60 * 1000;
const STALE_WINDOW_MS = 48 * 60 * 60 * 1000;

export function nodeFreshness(position: MapPosition, nowMs: number): NodeFreshness {
	const lastSeenMs = position.lastSeen ? Date.parse(position.lastSeen) : NaN;
	if (!Number.isFinite(lastSeenMs)) return 'hidden';
	const sinceLast = nowMs - lastSeenMs;
	if (sinceLast > STALE_WINDOW_MS) return 'hidden';

	if (position.lastDirectSeen) {
		const directMs = Date.parse(position.lastDirectSeen);
		if (Number.isFinite(directMs) && nowMs - directMs <= DIRECT_WINDOW_MS) return 'direct';
	}

	if (sinceLast <= INDIRECT_WINDOW_MS) return 'indirect';
	return 'stale';
}

export const FRESHNESS_FILL: Record<NodeFreshness, string> = {
	direct: 'rgba(34,197,94,0.9)',
	indirect: 'rgba(59,130,246,0.9)',
	stale: 'rgba(156,163,175,0.85)',
	hidden: 'rgba(0,0,0,0)'
};

export const FRESHNESS_ZINDEX: Record<NodeFreshness, number> = {
	direct: 3,
	indirect: 2,
	stale: 1,
	hidden: 0
};

export const MYCALL_FILL = 'rgba(239,68,68,0.95)';
export const MYCALL_ZINDEX = 4;
