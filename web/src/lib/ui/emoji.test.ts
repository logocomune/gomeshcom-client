import { describe, expect, it } from 'vitest';
import { insertAtSelection, limitRunes, runeCount } from './emoji';

describe('emoji input helpers', () => {
	it.each([
		{
			message: 'CQ test',
			selectionStart: 2,
			selectionEnd: 2,
			value: '🙂',
			expected: { message: 'CQ🙂 test', cursor: 4 }
		},
		{
			message: 'hello world',
			selectionStart: 6,
			selectionEnd: 11,
			value: '👋',
			expected: { message: 'hello 👋', cursor: 8 }
		}
	])('inserts emoji at selection', ({ message, selectionStart, selectionEnd, value, expected }) => {
		expect(insertAtSelection(message, selectionStart, selectionEnd, value)).toEqual(expected);
	});

	it('counts Unicode runes instead of UTF-16 code units', () => {
		expect(runeCount('A🙂👨‍👩‍👧‍👦')).toBe(9);
	});

	it('limits Unicode input by runes', () => {
		expect(limitRunes('A🙂B', 2)).toBe('A🙂');
	});
});
