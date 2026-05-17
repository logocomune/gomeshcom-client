import { describe, expect, it } from 'vitest';
import { getSendComposerState } from './send';

describe('getSendComposerState', () => {
	it('allows send when draft is non-empty and TX enabled', () => {
		expect(
			getSendComposerState({
				draftMessage: '  hello  ',
				sending: false,
				txDisabled: false
			})
		).toEqual({
			canSend: true,
			label: 'Send',
			hint: null
		});
	});

	it.each([
		{
			name: 'empty draft',
			input: { draftMessage: '   ', sending: false, txDisabled: false },
			state: { canSend: false, label: 'Send', hint: null }
		},
		{
			name: 'sending in progress',
			input: { draftMessage: 'hello', sending: true, txDisabled: false },
			state: { canSend: false, label: '…', hint: null }
		},
		{
			name: 'TX disabled',
			input: { draftMessage: 'hello', sending: false, txDisabled: true },
			state: { canSend: false, label: 'TX off', hint: 'Dry-run mode' }
		}
	])('returns disabled composer for $name', ({ input, state }) => {
		expect(getSendComposerState(input)).toEqual(state);
	});
});
