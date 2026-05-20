import { describe, expect, it } from 'vitest';
import type { ChatRecord } from '$lib/api/types';
import {
	chatIconTooltip,
	chatRecordMatchesFilter,
	chatRecordSeqId,
	stripNodeSequence
} from './chat-records';

function record(overrides: Partial<ChatRecord> = {}): ChatRecord {
	return {
		received_at: '2026-05-19T10:00:00Z',
		src: 'QQ5PMP-1',
		dst: '*',
		msg: 'hello {42',
		...overrides
	};
}

describe('chat record UI helpers', () => {
	it('extracts sequence id and strips node sequence suffix', () => {
		expect(chatRecordSeqId(record())).toBe('42');
		expect(chatRecordSeqId(record({ msg: 'hello {43}' }))).toBe('43');
		expect(chatRecordSeqId(record({ msg: 'hello' }))).toBeNull();
		expect(stripNodeSequence('hello{42}')).toBe('hello');
		expect(stripNodeSequence('hello {42}')).toBe('hello ');
		expect(stripNodeSequence('hello {42')).toBe('hello ');
		expect(stripNodeSequence('hello {43}')).toBe('hello ');
	});

	it('matches filter against source, destination, text, and message kind', () => {
		const item = record({ src: 'QQ5PMP-1,RELAY', dst: '9', msg: '{CET}2026-05-19 10:00:00' });

		expect(chatRecordMatchesFilter(item, 'qq5')).toBe(true);
		expect(chatRecordMatchesFilter(item, '9')).toBe(true);
		expect(chatRecordMatchesFilter(item, 'network')).toBe(true);
		expect(chatRecordMatchesFilter(item, 'missing')).toBe(false);
		expect(chatRecordMatchesFilter(item, '   ')).toBe(true);
	});

	it('describes delivery and message kind', () => {
		expect(chatIconTooltip(record({ delivery_status: 'pending' }))).toBe('Pending');
		expect(chatIconTooltip(record({ delivery_status: 'failed' }))).toBe('Failed');
		expect(chatIconTooltip(record({ msg: 'ack123' }))).toBe('ACK — message acknowledged');
	});
});
