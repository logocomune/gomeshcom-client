import { describe, it, expect, vi, afterEach } from 'vitest';
import { formatTime, formatRtt, formatChatTimestamp } from './format';

describe('formatTime', () => {
	it('formats ISO string to HH:MM:SS', () => {
		const result = formatTime('2025-01-15T14:30:45Z');
		expect(result).toMatch(/\d{2}:\d{2}:\d{2}/);
	});
});

describe('formatChatTimestamp', () => {
	afterEach(() => vi.useRealTimers());

	it('today: returns only HH:MM:SS', () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2025-06-15T10:00:00Z'));
		const result = formatChatTimestamp('2025-06-15T09:45:30Z');
		expect(result).toMatch(/^\d{2}:\d{2}:\d{2}$/);
	});

	it('past day: returns date + HH:MM:SS', () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2025-06-15T10:00:00Z'));
		const result = formatChatTimestamp('2025-06-14T09:45:30Z');
		expect(result).toMatch(/^\d{2}\/\d{2} \d{2}:\d{2}:\d{2}$/);
	});

	it('yesterday: returns date + HH:MM:SS', () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2025-06-15T12:00:00Z'));
		const result = formatChatTimestamp('2025-06-13T09:00:00Z');
		expect(result).toMatch(/^\d{2}\/\d{2} \d{2}:\d{2}:\d{2}$/);
	});
});

describe('formatRtt', () => {
	const cases: [number, string][] = [
		[-1, ''],
		[0, '0ms'],
		[500, '500ms'],
		[999, '999ms'],
		[1000, '1s'],
		[1500, '2s'],
		[59000, '59s'],
		[60000, '1m 0s'],
		[90000, '1m 30s']
	];
	it.each(cases)('formatRtt(%i) → %s', (ms, expected) => {
		expect(formatRtt(ms)).toBe(expected);
	});
});
