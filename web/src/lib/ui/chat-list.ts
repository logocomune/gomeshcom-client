import type { ChatRecord, Conversation } from '$lib/api/types';
import { cleanMessage } from '$lib/api/events';

const PREVIEW_MAX = 40;

export function previewText(msg: string): string {
	const text = cleanMessage(msg) || msg;
	return text.length > PREVIEW_MAX ? text.slice(0, PREVIEW_MAX) + '…' : text;
}

export function conversationPreview(records: ChatRecord[]): string {
	const last = records.at(-1);
	if (!last) return '';
	return previewText(last.msg);
}

export function sortByRecency(conversations: Conversation[]): Conversation[] {
	return [...conversations].sort((a, b) => b.last_seen.localeCompare(a.last_seen));
}

export function formatRelativeTime(isoTs: string): string {
	const now = Date.now();
	const then = new Date(isoTs).getTime();
	if (isNaN(then)) return '';
	const diffMs = now - then;
	const diffMins = Math.floor(diffMs / 60_000);
	if (diffMins < 1) return 'now';
	if (diffMins < 60) return `${diffMins}m`;
	const diffHours = Math.floor(diffMins / 60);
	if (diffHours < 24) return `${diffHours}h`;
	const diffDays = Math.floor(diffHours / 24);
	if (diffDays === 1) return 'yesterday';
	if (diffDays < 7) return `${diffDays}d`;
	return new Date(isoTs).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}
