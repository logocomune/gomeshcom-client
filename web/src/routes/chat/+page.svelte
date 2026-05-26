<script lang="ts">
	import ChatView from '$lib/components/views/ChatView.svelte';
	import { getAppContext } from '$lib/app/app-context';

	const app = getAppContext();
</script>

<svelte:head>
	<title>Chat - goMeshCom</title>
</svelte:head>

<div class="flex min-h-0 flex-1 overflow-hidden">
	<ChatView ackIndex={app.ackIndex} handleSend={app.handleSend} isDesktop={app.isDesktop} />
</div>
