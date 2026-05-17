<script lang="ts">
	import favicon from '$lib/assets/gomeshcom-logo.png';
	import '../app.css';
	import TxDisabledBanner from '$lib/components/TxDisabledBanner.svelte';
	import { stationStore } from '$lib/stores/station';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{#if $stationStore?.txDisabled}
	<TxDisabledBanner />
{/if}

{@render children()}
