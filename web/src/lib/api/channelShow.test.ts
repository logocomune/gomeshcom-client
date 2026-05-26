import { describe, it, expect } from 'vitest';
import { isValidChannelShowChannel, isConvHidden } from './channelShow';
import type { ChannelShowConfig } from './channelShow';

describe('isValidChannelShowChannel', () => {
	it.each(['*', '2', '222', '22201', '9', '260'])('accepts valid channel %s', (v) => {
		expect(isValidChannelShowChannel(v)).toBe(true);
	});

	it.each(['', 'abc', '12a', '-1', ' ', '0abc'])('rejects invalid channel %s', (v) => {
		expect(isValidChannelShowChannel(v)).toBe(false);
	});
});

describe('isConvHidden', () => {
	const all: ChannelShowConfig = { mode: 'all', channels: [] };
	const allowlist: ChannelShowConfig = { mode: 'allowlist', channels: ['*', '222'] };
	const emptyAllowlist: ChannelShowConfig = { mode: 'allowlist', channels: [] };

	it('never hides anything when mode=all', () => {
		expect(isConvHidden('P_broadcast', all)).toBe(false);
		expect(isConvHidden('P_222', all)).toBe(false);
		expect(isConvHidden('P_260', all)).toBe(false);
		expect(isConvHidden('DM_QQ1ABC-1', all)).toBe(false);
	});

	it('never hides DMs regardless of mode', () => {
		expect(isConvHidden('DM_QQ1ABC-1', allowlist)).toBe(false);
		expect(isConvHidden('DM_QQ1ABC-1', emptyAllowlist)).toBe(false);
	});

	it('shows channels in allowlist', () => {
		expect(isConvHidden('P_broadcast', allowlist)).toBe(false);
		expect(isConvHidden('P_222', allowlist)).toBe(false);
	});

	it('hides channels not in allowlist', () => {
		expect(isConvHidden('P_260', allowlist)).toBe(true);
		expect(isConvHidden('P_262', allowlist)).toBe(true);
	});

	it('hides everything except DMs when allowlist is empty', () => {
		expect(isConvHidden('P_broadcast', emptyAllowlist)).toBe(true);
		expect(isConvHidden('P_222', emptyAllowlist)).toBe(true);
	});
});
