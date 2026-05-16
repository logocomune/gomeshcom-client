import type { StreamEvent } from './types';
import type { AckSource } from './events';
import {
	ackSeqId,
	rejSeqId,
	ackSource as getAckSource,
	ackTargetCallsign,
	splitSourcePath,
	packetFromEvent
} from './events';

export type AckEntry = {
	receivedAt: string;
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

export function buildAckIndex(events: StreamEvent[]): AckIndex {
	const acked = new Map<string, AckEntry[]>();
	const rejected = new Map<string, AckEntry[]>();

	for (const event of events) {
		const packet = packetFromEvent(event);
		if (!packet) continue;

		const { relays, origin } = splitSourcePath(packet.src);
		const entry: AckEntry = {
			receivedAt: event.receivedAt,
			rssi: packet.rssi,
			snr: packet.snr,
			via: relays,
			source: origin,
			ackSource: getAckSource(packet),
			target: ackTargetCallsign(packet)
		};

		const aSeq = ackSeqId(packet);
		if (aSeq !== null) {
			const list = acked.get(aSeq) ?? [];
			list.push(entry);
			acked.set(aSeq, list);
		}

		const rSeq = rejSeqId(packet);
		if (rSeq !== null) {
			const list = rejected.get(rSeq) ?? [];
			list.push(entry);
			rejected.set(rSeq, list);
		}
	}

	return { acked, rejected };
}
