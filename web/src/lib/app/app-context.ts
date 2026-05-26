import { getContext, setContext } from 'svelte';
import type { AckIndex } from '$lib/api/acks';
import type { SseStore } from '$lib/stores/sse.svelte';

const APP_CONTEXT_KEY = Symbol('meshcom-app-context');

export type AppContext = {
	readonly ackIndex: AckIndex;
	readonly isDesktop: boolean;
	readonly sse: SseStore;
	handleSend: () => void | Promise<void>;
	openDeleteConfirm: () => void;
	openNewDm: () => void;
	openNewChannel: () => void;
};

export function setAppContext(context: AppContext) {
	setContext(APP_CONTEXT_KEY, context);
}

export function getAppContext(): AppContext {
	return getContext<AppContext>(APP_CONTEXT_KEY);
}
