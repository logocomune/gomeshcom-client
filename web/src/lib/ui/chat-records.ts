import { cleanMessage, messageKind } from '$lib/api/events';
import type { ChatRecord } from '$lib/api/types';
import {
	mdiAlertCircleOutline,
	mdiCheckCircleOutline,
	mdiClockOutline,
	mdiCogOutline,
	mdiEmailOutline
} from '@mdi/js';

export function chatRecordSeqId(record: ChatRecord): string | null {
	const match = (record.msg ?? '').match(/\{(\d+)\s*$/);
	return match?.[1] ?? null;
}

export function chatMdiIcon(record: ChatRecord): string {
	const kind = messageKind(record.msg).kind;
	if (kind === 'time') return mdiClockOutline;
	if (kind === 'ack') return mdiCheckCircleOutline;
	if (kind === 'reject') return mdiAlertCircleOutline;
	if (kind === 'config') return mdiCogOutline;
	return mdiEmailOutline;
}

export function chatIconTooltip(record: ChatRecord): string {
	if (record.delivery_status === 'pending') return 'Pending';
	if (record.delivery_status === 'failed') return 'Failed';
	const kind = messageKind(record.msg).kind;
	if (kind === 'ack') return 'ACK — message acknowledged';
	if (kind === 'reject') return 'REJ — message rejected';
	if (kind === 'time') return 'Network time sync';
	if (kind === 'config') return 'Config update';
	return 'Text message';
}

export function chatRecordMatchesFilter(record: ChatRecord, filter: string): boolean {
	const term = filter.trim().toLowerCase();
	if (term === '') return true;
	const kind = messageKind(record.msg);
	const haystack: string[] = [
		record.src ?? '',
		record.dst ?? '',
		record.msg ?? '',
		cleanMessage(record.msg),
		kind.kind,
		kind.label
	];
	return haystack.some((value) => value.toLowerCase().includes(term));
}

export function stripNodeSequence(message: string): string {
	return message.replace(/\{\d+\}?$/, '');
}
