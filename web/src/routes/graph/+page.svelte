<script lang="ts">
	import GraphView from '$lib/components/views/GraphView.svelte';
</script>

<svelte:head>
	<title>Graph - goMeshCom</title>
</svelte:head>

<div
	class="m-2 flex min-h-0 flex-1 flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#151a24] shadow-sm"
>
	<GraphView />
</div>
