import { beforeEach, describe, expect, it } from 'vitest';

import { AppController } from './app-controller.svelte';
import { chatState } from '$lib/stores/chat.svelte';

describe('AppController confirmNewDm', () => {
	beforeEach(() => {
		chatState.newDmOpen = true;
		chatState.newDmCallsign = '';
		chatState.newDmError = '';
		chatState.chatTarget = { kind: 'channel', value: 'Broadcast' };
	});

	it('normalizes non-IU prefixes to QQ before selecting contact', () => {
		const controller = new AppController();
		chatState.newDmCallsign = 'ik5mnn-1';

		controller.confirmNewDm();

		expect(chatState.newDmError).toBe('');
		expect(chatState.newDmOpen).toBe(false);
		expect(chatState.chatTarget).toEqual({ kind: 'contact', value: 'QQ5MNN-1' });
	});

	it('keeps IU5PMP unchanged', () => {
		const controller = new AppController();
		chatState.newDmCallsign = 'iu5pmp-1';

		controller.confirmNewDm();

		expect(chatState.chatTarget).toEqual({ kind: 'contact', value: 'IU5PMP-1' });
	});

	it('rejects invalid callsigns', () => {
		const controller = new AppController();
		chatState.newDmCallsign = 'bad';

		controller.confirmNewDm();

		expect(chatState.newDmError).toContain('Invalid callsign');
		expect(chatState.chatTarget).toEqual({ kind: 'channel', value: 'Broadcast' });
	});
});
