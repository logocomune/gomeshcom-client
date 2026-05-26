<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import ChatPage from '../../routes/chat/+page.svelte';
	import { chatState } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { eventsState } from '$lib/stores/events.svelte';
	import { createAppController } from '$lib/app/app-controller.svelte';
	import { setAppContext } from '$lib/app/app-context';

	resetGlobalState();
	const app = createAppController();

	setAppContext(app.context);

	$effect(() => app.loadCurrentConversationHistory());

	onMount(() => {
		void app.mount();
	});

	onDestroy(() => {
		app.destroy();
	});
	function resetGlobalState() {
		chatState.chatHistory = {};
		chatState.conversations = [];
		chatState.chatTarget = { kind: 'channel', value: 'Broadcast' };
		chatState.chatStatus = {};
		chatState.chatFilter = '';
		chatState.fetchedConvIds = new Set();
		chatState.draftMessage = '';
		chatState.sending = false;
		chatState.sendError = null;
		chatState.conversationsLoaded = false;
		eventsState.clear();
		eventsState.storedPositions = [];
		connectionState.stationCallsign = '';
		connectionState.txDisabled = false;
	}
</script>

<ChatPage />
