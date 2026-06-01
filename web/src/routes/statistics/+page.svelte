<script lang="ts">
	import StatisticsView from '$lib/components/views/StatisticsView.svelte';
</script>

<svelte:head>
	<title>Statistics - goMeshCom</title>
</svelte:head>

<div
	class="m-2 flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-ink-dim/20 bg-base shadow-sm"
>
	<StatisticsView />
</div>
