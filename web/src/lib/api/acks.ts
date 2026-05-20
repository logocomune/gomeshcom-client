import type { ChatRecord, StreamEvent } from './types';
import type { AckSource } from './events';
import {
	ackSeqId,
	ackSource,
	isReplayEvent,
	rejSeqId,
	ackSource as getAckSource,
	ackTargetCallsign,
	messageKind,
	splitSourcePath,
	packetFromEvent
} from './events';

export type AckEntry = {
	receivedAt: string;
	msgId?: string;
	rssi?: number;
	snr?: number;
	via: string[];
	source: string;
	ackSource: AckSource;
	target: string | null;
};

export type AckIndex = {
	acked: Map<string, AckEntry[]>;
	rejected: Map<string, AckEntry[]>;
};

export function ackEntriesForRecord(
	ackIndex: AckIndex,
	sequenceId: string | null,
	record: ChatRecord,
	stationCallsign: string
): AckEntry[] {
	return entriesForRecord(ackIndex.acked, sequenceId, record, stationCallsign);
}

export function rejectEntriesForRecord(
	ackIndex: AckIndex,
	sequenceId: string | null,
	record: ChatRecord,
	stationCallsign: string
): AckEntry[] {
	return entriesForRecord(ackIndex.rejected, sequenceId, record, stationCallsign);
}

export function mergeAckIndexes(...indexes: AckIndex[]): AckIndex {
	const acked = new Map<string, AckEntry[]>();
	const rejected = new Map<string, AckEntry[]>();

	for (const index of indexes) {
		appendEntries(acked, index.acked);
		appendEntries(rejected, index.rejected);
	}

	return { acked, rejected };
}

export function buildAckIndex(events: StreamEvent[]): AckIndex {
	const acked = new Map<string, AckEntry[]>();
	const rejected = new Map<string, AckEntry[]>();

	for (const event of events) {
		if (isReplayEvent(event)) continue;
		const packet = packetFromEvent(event);
		if (!packet) continue;

		const { relays, origin } = splitSourcePath(packet.src);
		const entry: AckEntry = {
			receivedAt: event.receivedAt,
			msgId: packet.msg_id,
			rssi: packet.rssi,
			snr: packet.snr,
			via: relays,
			source: origin,
			ackSource: getAckSource(packet),
			target: ackTargetCallsign(packet)
		};

		const aSeq = ackSeqId(packet);
		if (aSeq !== null) {
			appendEntry(acked, aSeq, entry);
		}

		const rSeq = rejSeqId(packet);
		if (rSeq !== null) {
			appendEntry(rejected, rSeq, entry);
		}
	}

	return { acked, rejected };
}

export function buildAckIndexFromChatRecords(records: ChatRecord[]): AckIndex {
	const acked = new Map<string, AckEntry[]>();
	const rejected = new Map<string, AckEntry[]>();

	for (const record of records) {
		const kind = messageKind(record.msg).kind;
		if (kind !== 'ack' && kind !== 'reject') continue;

		const { relays, origin } = splitSourcePath(record.src);
		const entry: AckEntry = {
			receivedAt: record.received_at,
			msgId: record.msg_id,
			rssi: record.rssi,
			snr: record.snr,
			via: relays,
			source: origin,
			ackSource: ackSource(record),
			target: ackTargetCallsign(record)
		};

		const sequenceId = kind === 'ack' ? ackSeqId(record) : rejSeqId(record);
		if (sequenceId === null) continue;

		const index = kind === 'ack' ? acked : rejected;
		appendEntry(index, sequenceId, entry);
	}

	return { acked, rejected };
}

function entriesForRecord(
	entriesBySequence: Map<string, AckEntry[]>,
	sequenceId: string | null,
	record: ChatRecord,
	stationCallsign: string
): AckEntry[] {
	if (!sequenceId) return [];
	const entries = entriesBySequence.get(sequenceId) ?? [];
	return entries.filter((entry) => entryMatchesRecord(entry, record, stationCallsign));
}

function entryMatchesRecord(entry: AckEntry, record: ChatRecord, stationCallsign: string): boolean {
	const destination = normalizeCallsign(record.dst);
	if (destination && normalizeCallsign(entry.source) !== destination) return false;

	const target = normalizeCallsign(entry.target);
	const local = normalizeCallsign(stationCallsign);
	if (target && local && target !== local) return false;

	return true;
}

function normalizeCallsign(value: string | null | undefined): string {
	return (value ?? String()).trim().toUpperCase();
}

function appendEntries(target: Map<string, AckEntry[]>, source: Map<string, AckEntry[]>): void {
	for (const [sequenceId, entries] of source) {
		for (const entry of entries) {
			appendEntry(target, sequenceId, entry);
		}
	}
}

function appendEntry(target: Map<string, AckEntry[]>, sequenceId: string, entry: AckEntry): void {
	const existing = target.get(sequenceId) ?? [];
	const key = ackEntryKey(entry);
	if (existing.some((item) => ackEntryKey(item) === key)) return;
	target.set(sequenceId, [...existing, entry]);
}

function ackEntryKey(entry: AckEntry): string {
	if (entry.msgId && entry.msgId !== '') {
		return ['msg', entry.source, entry.msgId].join('|');
	}

	return [
		entry.receivedAt,
		entry.source,
		entry.ackSource,
		entry.target ?? String(),
		entry.via.join(','),
		entry.rssi ?? String(),
		entry.snr ?? String()
	].join('|');
}
