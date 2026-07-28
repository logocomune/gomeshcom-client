import { describe, expect, it } from 'vitest';
import {
	hasNumericSsid,
	positionForCallsign,
	splitChatContent,
	splitChatMentions
} from './chat-mentions';

describe('splitChatMentions', () => {
	it('splits amateur-radio callsign mentions, including SSID', () => {
		expect(splitChatMentions('CQ @IU5PMP, reply @IU5PMP-12!')).toEqual([
			{ kind: 'text', value: 'CQ ' },
			{ kind: 'mention', callsign: 'IU5PMP', value: '@IU5PMP' },
			{ kind: 'text', value: ', reply ' },
			{ kind: 'mention', callsign: 'IU5PMP-12', value: '@IU5PMP-12' },
			{ kind: 'text', value: '!' }
		]);
	});

	it('does not match callsigns embedded in another identifier or email address', () => {
		expect(splitChatMentions('mail a@IU5PMP.it; bad @IU5PMP-123; ok @IU5PMP.')).toEqual([
			{ kind: 'text', value: 'mail a@IU5PMP.it; bad @IU5PMP-123; ok ' },
			{ kind: 'mention', callsign: 'IU5PMP', value: '@IU5PMP' },
			{ kind: 'text', value: '.' }
		]);
	});

	it('finds coordinates by callsign id or source', () => {
		const positions = [
			{ id: 'IU5PMP-12', source: 'node-a', lat: 43.7, lon: 11.2, updatedAt: '' },
			{ id: 'node-b', source: 'IU5PMP', lat: 44.1, lon: 10.9, updatedAt: '' }
		];
		expect(positionForCallsign(positions, 'iu5pmp-12')).toBe(positions[0]);
		expect(positionForCallsign(positions, 'IU5PMP')).toBe(positions[1]);
		expect(positionForCallsign(positions, 'IZ0ZZZ')).toBeUndefined();
	});

	it('only enables actions for callsigns with a numeric SSID', () => {
		expect(hasNumericSsid('IU5PMP')).toBe(false);
		expect(hasNumericSsid('IU5PMP-12')).toBe(true);
		expect(hasNumericSsid('IU5PMP-123')).toBe(false);
	});

	it('splits valid HTTP links and leaves trailing sentence punctuation as text', () => {
		expect(splitChatContent('See https://example.org/path?q=1. Then http://example.net/')).toEqual([
			{ kind: 'text', value: 'See ' },
			{ kind: 'link', value: 'https://example.org/path?q=1', href: 'https://example.org/path?q=1' },
			{ kind: 'text', value: '. Then ' },
			{ kind: 'link', value: 'http://example.net/', href: 'http://example.net/' }
		]);
	});

	it('rejects incomplete HTTP links', () => {
		expect(splitChatContent('Broken https:// only')).toEqual([
			{ kind: 'text', value: 'Broken https:// only' }
		]);
	});
});
