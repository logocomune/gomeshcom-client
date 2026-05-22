import { describe, expect, it } from 'vitest';
import {
	ackEntriesForRecord,
	buildAckIndex,
	buildAckIndexFromChatRecords,
	mergeAckIndexes
} from './acks';
import type { ChatRecord, StreamEvent } from './types';

describe('ackEntriesForRecord', () => {
	it('keeps ACKs scoped to the sent message destination when sequence IDs collide', () => {
		const ackIndex = buildAckIndex([
			packetEvent('2026-05-20T10:00:05Z', {
				type: 'msg',
				src: 'QQ2BOB-1',
				dst: 'QQ0ME-1',
				msg: 'QQ0ME-1 :ack571'
			}),
			packetEvent('2026-05-20T10:00:06Z', {
				type: 'msg',
				src: 'QQ3ANN-1',
				dst: 'QQ0ME-1',
				msg: 'QQ0ME-1 :ack571'
			})
		]);

		const bobRecord = sentRecord('QQ2BOB-1', 'hello bob {571');
		const annRecord = sentRecord('QQ3ANN-1', 'hello ann {571');

		expect(ackEntriesForRecord(ackIndex, '571', bobRecord, 'QQ0ME-1')).toMatchObject([
			{ source: 'QQ2BOB-1' }
		]);
		expect(ackEntriesForRecord(ackIndex, '571', annRecord, 'QQ0ME-1')).toMatchObject([
			{ source: 'QQ3ANN-1' }
		]);
	});

	it('ignores ACKs targeted at another station', () => {
		const ackIndex = buildAckIndex([
			packetEvent('2026-05-20T10:00:05Z', {
				type: 'msg',
				src: 'QQ2BOB-1',
				dst: 'QQ9OTHER-1',
				msg: 'QQ9OTHER-1 :ack571'
			})
		]);

		expect(
			ackEntriesForRecord(ackIndex, '571', sentRecord('QQ2BOB-1', 'hello bob {571'), 'QQ0ME-1')
		).toEqual([]);
	});
});

describe('buildAckIndex', () => {
	it('ignores replay packet.received ACK events', () => {
		const ackIndex = buildAckIndex([
			{
				id: 'live',
				type: 'packet.received',
				receivedAt: '2026-05-20T10:00:06Z',
				data: {
					packet: {
						type: 'msg',
						src: 'QQ2BOB-1',
						dst: 'QQ0ME-1',
						msg: 'QQ0ME-1 :ack571'
					}
				}
			},
			{
				id: 'replay',
				type: 'packet.received',
				receivedAt: '2026-05-20T10:00:05Z',
				data: {
					replay: true,
					packet: {
						type: 'msg',
						src: 'QQ3ANN-1',
						dst: 'QQ0ME-1',
						msg: 'QQ0ME-1 :ack571'
					}
				}
			}
		]);

		expect(ackEntriesForRecord(ackIndex, '571', sentRecord('QQ2BOB-1', 'hello {571'), 'QQ0ME-1')).toMatchObject([
			{ source: 'QQ2BOB-1' }
		]);
		expect(ackEntriesForRecord(ackIndex, '571', sentRecord('QQ3ANN-1', 'hello {571'), 'QQ0ME-1')).toEqual([]);
	});
});

function sentRecord(dst: string, msg: string): ChatRecord {
	return {
		received_at: '2026-05-20T10:00:00Z',
		src: 'QQ0ME-1',
		dst,
		msg
	};
}

function packetEvent(receivedAt: string, packet: Record<string, unknown>): StreamEvent {
	return {
		id: `${receivedAt}-${packet.src}`,
		type: 'packet.received',
		receivedAt,
		data: { packet }
	};
}

describe('buildAckIndexFromChatRecords', () => {
	it('indexes ACKs already loaded from chat history', () => {
		const ackIndex = buildAckIndexFromChatRecords([
			sentRecord('IZ5PFI-12', 'Prova invio{916'),
			{
				received_at: '2026-05-20T10:09:30.522559329Z',
				src: 'IZ5PFI-12,IK5FCK-10',
				src_type: 'lora',
				dst: 'IU5PMP-1',
				msg_id: '1AE10322',
				msg: 'IU5PMP-1 :ack916',
				rssi: -37,
				snr: 5
			}
		]);

		expect(
			ackEntriesForRecord(
				ackIndex,
				'916',
				{
					received_at: '2026-05-20T10:06:39.937257857Z',
					src: 'IU5PMP-1',
					src_type: 'node',
					dst: 'IZ5PFI-12',
					msg_id: 'E4B9D394',
					msg: 'Prova invio{916'
				},
				'IU5PMP-1'
			)
		).toMatchObject([{ source: 'IZ5PFI-12', via: ['IK5FCK-10'], rssi: -37, snr: 5 }]);
	});

	it('deduplicates duplicate ACKs inside chat history only', () => {
		const duplicateAck = {
			received_at: '2026-05-20T10:14:01.69911781Z',
			src: 'IK5FCK-10',
			dst: 'IU5PMP-1',
			msg_id: 'E4C491B8',
			msg: 'IU5PMP-1 :ack918',
			rssi: -36,
			snr: 5
		};
		const secondAck = {
			received_at: '2026-05-20T10:14:35.903623675Z',
			src: 'IK5FCK-10',
			dst: 'IU5PMP-1',
			msg_id: 'E4C491B9',
			msg: 'IU5PMP-1 :ack918',
			rssi: -37,
			snr: 6
		};
		const ackIndex = buildAckIndexFromChatRecords([
			duplicateAck,
			secondAck,
			duplicateAck,
			secondAck
		]);

		expect(
			ackEntriesForRecord(ackIndex, '918', sentRecord('IK5FCK-10', 'Prova invio 1{918'), 'IU5PMP-1')
		).toHaveLength(2);
	});

	it('deduplicates ACKs present in both live and chat-history indexes', () => {
		const liveIndex = buildAckIndex([
			packetEvent('2026-05-20T10:14:01.69911781Z', {
				type: 'msg',
				src: 'IK5FCK-10',
				dst: 'IU5PMP-1',
				msg_id: 'E4C491B8',
				msg: 'IU5PMP-1 :ack918',
				rssi: -36,
				snr: 5
			}),
			packetEvent('2026-05-20T10:14:35.903623675Z', {
				type: 'msg',
				src: 'IK5FCK-10',
				dst: 'IU5PMP-1',
				msg_id: 'E4C491B9',
				msg: 'IU5PMP-1 :ack918',
				rssi: -37,
				snr: 6
			})
		]);
		const historyIndex = buildAckIndexFromChatRecords([
			{
				received_at: '2026-05-20T10:14:01.69911781Z',
				src: 'IK5FCK-10',
				dst: 'IU5PMP-1',
				msg_id: 'E4C491B8',
				msg: 'IU5PMP-1 :ack918',
				rssi: -36,
				snr: 5
			},
			{
				received_at: '2026-05-20T10:14:35.903623675Z',
				src: 'IK5FCK-10',
				dst: 'IU5PMP-1',
				msg_id: 'E4C491B9',
				msg: 'IU5PMP-1 :ack918',
				rssi: -37,
				snr: 6
			}
		]);

		const merged = mergeAckIndexes(liveIndex, historyIndex);

		expect(
			ackEntriesForRecord(merged, '918', sentRecord('IK5FCK-10', 'Prova invio 1{918'), 'IU5PMP-1')
		).toHaveLength(2);
	});

	it('merges live and chat-history ACK indexes', () => {
		const liveIndex = buildAckIndex([
			packetEvent('2026-05-20T10:00:05Z', {
				type: 'msg',
				src: 'QQ2BOB-1',
				dst: 'QQ0ME-1',
				msg: 'QQ0ME-1 :ack571'
			})
		]);
		const historyIndex = buildAckIndexFromChatRecords([
			{
				received_at: '2026-05-20T10:00:06Z',
				src: 'QQ3ANN-1',
				dst: 'QQ0ME-1',
				msg: 'QQ0ME-1 :ack572'
			}
		]);

		const merged = mergeAckIndexes(liveIndex, historyIndex);

		expect(
			ackEntriesForRecord(merged, '571', sentRecord('QQ2BOB-1', 'hello bob {571'), 'QQ0ME-1')
		).toHaveLength(1);
		expect(
			ackEntriesForRecord(merged, '572', sentRecord('QQ3ANN-1', 'hello ann {572'), 'QQ0ME-1')
		).toHaveLength(1);
	});
});
