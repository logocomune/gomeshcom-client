import { describe, expect, it } from 'vitest';
import { packetBadge } from '$lib/api/events';
import type { StreamEvent } from '$lib/api/types';
import {
	filterStreamEvents,
	iconTooltip,
	isMessageEvent,
	messageRoute,
	messageText,
	packetField,
	packetTone
} from './stream';

function streamEvent(type: string, data: unknown): StreamEvent {
	return {
		id: type,
		type,
		receivedAt: '2026-05-19T10:00:00Z',
		data
	};
}

function packetEvent(packet: Record<string, unknown>): StreamEvent {
	return streamEvent('packet.received', { packet });
}

describe('stream UI helpers', () => {
	it('filters events by summary, packet fields, and badge', () => {
		const message = packetEvent({ type: 'msg', src: 'IU5PMP-1', dst: '*', msg: 'hello' });
		const telemetry = packetEvent({ type: 'tele', src: 'NODE-1', batt: 90 });
		const error = streamEvent('packet.error', 'bad json');

		expect(filterStreamEvents([message, telemetry, error], 'iu5')).toEqual([message]);
		expect(filterStreamEvents([message, telemetry, error], 'tele')).toEqual([telemetry]);
		expect(filterStreamEvents([message, telemetry, error], 'bad')).toEqual([error]);
		expect(filterStreamEvents([message], '   ')).toEqual([message]);
	});

	it('formats message route and text', () => {
		const event = packetEvent({
			type: 'msg',
			src: 'IU5PMP-1,RELAY-2',
			dst: '*',
			msg: '{CET}hello'
		});

		expect(isMessageEvent(event)).toBe(true);
		expect(messageRoute(event)).toEqual({
			origin: 'IU5PMP-1',
			relays: ['RELAY-2'],
			destination: 'Broadcast'
		});
		expect(messageText(event)).toBe('hello');
	});

	it('formats packet fields and tones', () => {
		const position = packetEvent({ type: 'pos', src: 'NODE-1', lat: 43.1 });
		const raw = streamEvent('message.created', {});

		expect(packetBadge(position)).toBe('pos');
		expect(packetTone(position)).toContain('border-emerald');
		expect(packetTone(raw)).toContain('border-gray');
		expect(packetField(position, 'missing')).toBe('—');
		expect(packetField(position, 'lat')).toBe('43.1');
	});

	it('describes packet icons', () => {
		expect(iconTooltip(packetEvent({ type: 'msg', msg: 'ack123' }))).toBe(
			'ACK — message acknowledged'
		);
		expect(iconTooltip(packetEvent({ type: 'pos' }))).toBe('Position report');
		expect(iconTooltip(streamEvent('message.created', {}))).toBe('Parse error');
	});
});
