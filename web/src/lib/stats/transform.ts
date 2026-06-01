import type { Bucket } from '$lib/api/stats';
import { resolveGroup } from '$lib/api/groups';

export interface SeriesPoint {
	label: string; // HH:00
	dm: number;
	dm_ack: number;
	public: number;
	telemetry: number;
	position: number;
}

export interface KpiTotals {
	dm: number;
	dm_ack: number;
	public: number;
	telemetry: number;
	position: number;
	errors: number;
	total: number;
}

export interface DistanceBin {
	label: string;
	count: number;
}

// buildHourlySeries converts a bucket array into a dense series of
// SeriesPoints covering the full [fromIso, toIso) range (ISO-8601 strings
// from the API response). Missing hours get zero values.
// Each label is "HH:00" in UTC.
export function buildHourlySeries(
	buckets: Bucket[],
	fromIso: string,
	toIso: string
): SeriesPoint[] {
	const byHour = new Map<number, Bucket>();
	for (const b of buckets) {
		byHour.set(b.hour, b);
	}

	const fromMs = new Date(fromIso).getTime();
	const toMs = new Date(toIso).getTime();
	if (isNaN(fromMs) || isNaN(toMs) || fromMs >= toMs) return [];

	// Truncate to UTC hour boundaries (inclusive on both ends).
	const startHour = Math.floor(fromMs / 3_600_000) * 3600;
	const endHour = Math.floor(toMs / 3_600_000) * 3600; // current hour, inclusive

	const result: SeriesPoint[] = [];
	for (let h = startHour; h <= endHour; h += 3600) {
		const b = byHour.get(h);
		const d = new Date(h * 1000);
		const label = d.getHours().toString().padStart(2, '0') + ':00';
		result.push({
			label,
			dm: b?.dm ?? 0,
			dm_ack: b?.dm_ack ?? 0,
			public: b?.public ?? 0,
			telemetry: b?.telemetry ?? 0,
			position: b?.position ?? 0
		});
	}
	return result;
}

// buildDailySeries aggregates buckets by local calendar day, covering the full
// [fromIso, toIso) range. Used for 7d / 30d views.
// Label format: "DD/MM".
export function buildDailySeries(
	buckets: Bucket[],
	fromIso: string,
	toIso: string
): SeriesPoint[] {
	// Group bucket data by local-date string "YYYY-MM-DD".
	const byDay = new Map<string, SeriesPoint>();

	for (const b of buckets) {
		const d = new Date(b.hour * 1000);
		const key =
			d.getFullYear() +
			'-' +
			String(d.getMonth() + 1).padStart(2, '0') +
			'-' +
			String(d.getDate()).padStart(2, '0');
		const label =
			String(d.getDate()).padStart(2, '0') + '/' + String(d.getMonth() + 1).padStart(2, '0');
		const prev = byDay.get(key) ?? { label, dm: 0, dm_ack: 0, public: 0, telemetry: 0, position: 0 };
		byDay.set(key, {
			label,
			dm: prev.dm + b.dm,
			dm_ack: prev.dm_ack + b.dm_ack,
			public: prev.public + b.public,
			telemetry: prev.telemetry + b.telemetry,
			position: prev.position + b.position
		});
	}

	// Build the full range of days so empty days show as zero bars.
	const fromMs = new Date(fromIso).getTime();
	const toMs = new Date(toIso).getTime();
	if (isNaN(fromMs) || isNaN(toMs)) return [];

	const result: SeriesPoint[] = [];
	const dayMs = 86_400_000;
	// Truncate fromMs to local midnight.
	const startDay = new Date(fromMs);
	startDay.setHours(0, 0, 0, 0);

	// Include today (current partial day).
	const endDay = new Date(toMs);
	endDay.setHours(0, 0, 0, 0);
	for (let t = startDay.getTime(); t <= endDay.getTime(); t += dayMs) {
		const d = new Date(t);
		const key =
			d.getFullYear() +
			'-' +
			String(d.getMonth() + 1).padStart(2, '0') +
			'-' +
			String(d.getDate()).padStart(2, '0');
		const label =
			String(d.getDate()).padStart(2, '0') + '/' + String(d.getMonth() + 1).padStart(2, '0');
		result.push(byDay.get(key) ?? { label, dm: 0, dm_ack: 0, public: 0, telemetry: 0, position: 0 });
	}
	return result;
}

// sumKpis computes totals across all buckets.
export function sumKpis(buckets: Bucket[]): KpiTotals {
	return buckets.reduce(
		(acc, b) => ({
			dm: acc.dm + b.dm,
			dm_ack: acc.dm_ack + b.dm_ack,
			public: acc.public + b.public,
			telemetry: acc.telemetry + b.telemetry,
			position: acc.position + b.position,
			errors: acc.errors + b.errors,
			total: acc.total + b.total
		}),
		{ dm: 0, dm_ack: 0, public: 0, telemetry: 0, position: 0, errors: 0, total: 0 }
	);
}

export interface ChannelRow {
	key: string; // "broadcast" | "ch:N" | "dm:CALLSIGN"
	label: string; // human-readable
	kind: 'broadcast' | 'channel' | 'dm';
	count: number;
}

// buildChannelTable merges Channels maps from all buckets and returns rows
// sorted by count descending. All dm: entries are aggregated into one DM row.
export function buildChannelTable(buckets: Bucket[]): ChannelRow[] {
	const merged = new Map<string, number>();
	for (const b of buckets) {
		if (!b.channels) continue;
		for (const [key, count] of Object.entries(b.channels)) {
			merged.set(key, (merged.get(key) ?? 0) + count);
		}
	}

	let dmTotal = 0;
	const rows: ChannelRow[] = [];

	for (const [key, count] of merged.entries()) {
		if (key.startsWith('dm:')) {
			dmTotal += count;
		} else if (key === 'broadcast') {
			rows.push({ key, label: '📡 Broadcast', kind: 'broadcast', count });
		} else if (key.startsWith('ch:')) {
			const chNum = key.slice(3);
			const group = resolveGroup(chNum);
			const label = group ? `${group.flag} CH. ${chNum} - ${group.note}` : `🌍 CH. ${chNum}`;
			rows.push({ key, label, kind: 'channel', count });
		} else {
			rows.push({ key, label: key, kind: 'channel', count });
		}
	}

	if (dmTotal > 0) {
		rows.push({ key: 'dm', label: '✉️ DM', kind: 'dm', count: dmTotal });
	}

	return rows.sort((a, b) => b.count - a.count);
}

// Canonical sort order for distance buckets (numeric lo edge first, "100+" last).
function binLo(label: string): number {
	if (label.endsWith('+')) return Infinity;
	return parseInt(label.split('-')[0], 10);
}

// buildDistanceHistogram merges distance_km maps from all buckets and returns
// sorted bins.
export function buildDistanceHistogram(buckets: Bucket[]): DistanceBin[] {
	const merged = new Map<string, number>();
	for (const b of buckets) {
		if (!b.distance_km) continue;
		for (const [label, count] of Object.entries(b.distance_km)) {
			merged.set(label, (merged.get(label) ?? 0) + count);
		}
	}
	return [...merged.entries()]
		.map(([label, count]) => ({ label, count }))
		.sort((a, b) => binLo(a.label) - binLo(b.label));
}
