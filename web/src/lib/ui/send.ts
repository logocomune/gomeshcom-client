type SendComposerStateInput = {
	draftMessage: string;
	sending: boolean;
	txDisabled: boolean;
};

type SendComposerState = {
	canSend: boolean;
	label: string;
	hint: string | null;
};

export function getSendComposerState(input: SendComposerStateInput): SendComposerState {
	if (input.txDisabled) {
		return {
			canSend: false,
			label: 'TX off',
			hint: 'Dry-run mode'
		};
	}

	if (input.sending) {
		return {
			canSend: false,
			label: '…',
			hint: null
		};
	}

	const canSend = input.draftMessage.trim().length > 0;
	return {
		canSend,
		label: 'Send',
		hint: null
	};
}
