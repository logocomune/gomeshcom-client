<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import favicon from '$lib/assets/gomeshcom-logo.png';
	import '../app.css';
	import AppShell from '$lib/app/AppShell.svelte';
	import { createAppController } from '$lib/app/app-controller.svelte';
	import { setAppContext } from '$lib/app/app-context';
	import { watchDesktop } from '$lib/responsive';

	let { children } = $props();
	const app = createAppController();

	setAppContext(app.context);

	$effect(() => watchDesktop((value) => (app.isDesktop = value)));
	$effect(() => app.loadCurrentConversationHistory());

	onMount(() => {
		void app.mount();
	});

	onDestroy(() => {
		app.destroy();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<AppShell {app}>
	{@render children()}
</AppShell>
