<script lang="ts">
	import UdpStreamPanel from '$lib/components/UdpStreamPanel.svelte';
	import { getAppContext } from '$lib/app/app-context';
	import { eventsState } from '$lib/stores/events.svelte';

	const app = getAppContext();
</script>

<svelte:head>
	<title>Traffic - goMeshCom</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col overflow-hidden">
	<UdpStreamPanel
		events={eventsState.events}
		filteredEvents={eventsState.filteredEvents}
		bind:streamFilter={eventsState.streamFilter}
		selectedEvent={eventsState.selectedEvent}
		isDesktop={app.isDesktop}
		streamHeightPx={eventsState.streamHeightPx}
		fillHeight
		onClearEvents={() => eventsState.clearAndSaveReplayCursor()}
		selectEvent={(event) =>
			app.isDesktop ? (eventsState.selectedEvent = event) : (eventsState.rawEvent = event)}
		showRawEvent={(event) => (eventsState.rawEvent = event)}
	/>
</div>
