import { hardwareHumanName } from '$lib/api/hardware';
import { nodeFreshness } from './node-state';
import type { MapPosition } from './types';

export function buildOwnMarkerTooltipHtml(position: MapPosition): string {
	const hardware = hardwareHumanName(position.hwId);
	if (!hardware) return escHtml(position.source);
	return `<strong>${escHtml(position.source)}</strong><br>${escHtml(hardware)}`;
}

export function buildTooltipHtml(position: MapPosition, now = Date.now()): string {
	const freshness = nodeFreshness(position, now);
	const lines: string[] = [
		`<strong>${escHtml(position.source)}</strong>`,
		`${escHtml(freshness)} · ${escHtml(timeAgo(position.lastSeen ?? position.updatedAt, now))}`
	];
	appendSeenLine(lines, 'last seen', position.lastSeen);
	appendSeenLine(lines, 'first seen', position.firstSeen);
	const hardware = hardwareHumanName(position.hwId);
	if (hardware) lines.push(escHtml(hardware));
	if (position.via && position.via.length > 0) {
		lines.push(`via ${escHtml(position.via.join(', '))}`);
	}
	if (freshness === 'direct') {
		const signalParts: string[] = [];
		if (position.rssi != null) signalParts.push(`📶 ${position.rssi} dBm`);
		if (position.snr != null) signalParts.push(`SNR ${position.snr}`);
		if (signalParts.length > 0) lines.push(signalParts.join(' · '));
	}
	if (position.altitude != null) lines.push(`↑ ${position.altitude} m`);
	if (position.battery != null) lines.push(`🔋 ${position.battery}%`);
	return lines.join('<br>');
}

function appendSeenLine(lines: string[], label: string, dateString: string | undefined) {
	if (!dateString) return;
	lines.push(`${label} ${escHtml(formatSeenDate(dateString))}`);
}

function formatSeenDate(dateString: string): string {
	const date = new Date(dateString);
	if (Number.isNaN(date.getTime())) return dateString;
	return date.toLocaleString('it-IT');
}

function timeAgo(dateString: string | undefined, now: number): string {
	if (!dateString) return 'sconosciuto';
	const diffSec = Math.max(0, Math.floor((now - new Date(dateString).getTime()) / 1000));
	if (diffSec < 60) return `${diffSec}s fa`;
	const diffMin = Math.floor(diffSec / 60);
	if (diffMin < 60) {
		const s = diffSec % 60;
		return s > 0 ? `${diffMin}m ${s}s fa` : `${diffMin}m fa`;
	}
	const diffHour = Math.floor(diffMin / 60);
	if (diffHour < 24) {
		const m = diffMin % 60;
		return m > 0 ? `${diffHour}h ${m}m fa` : `${diffHour}h fa`;
	}
	const diffDay = Math.floor(diffHour / 24);
	const h = diffHour % 24;
	return h > 0 ? `${diffDay}d ${h}h fa` : `${diffDay}d fa`;
}

export function escHtml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;');
}
