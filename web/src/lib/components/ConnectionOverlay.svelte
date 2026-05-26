<script lang="ts">
	import type { ConnectionState } from '$lib/api/types';
	let { state }: { state: ConnectionState } = $props();
</script>

{#if state === 'connecting'}
	<div
		class="pointer-events-none absolute inset-0 z-[9999] flex flex-col items-center justify-center gap-4 bg-[#111827]/90 backdrop-blur-sm"
	>
		<svg
			class="h-10 w-10 animate-spin text-blue-400"
			xmlns="http://www.w3.org/2000/svg"
			fill="none"
			viewBox="0 0 24 24"
		>
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
			<path
				class="opacity-75"
				fill="currentColor"
				d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
			/>
		</svg>
		<div class="flex flex-col items-center gap-1">
			<span class="text-sm font-medium tracking-wide text-gray-200">Connecting to MeshCom</span>
			<span class="text-xs text-gray-500">Waiting for UDP stream…</span>
		</div>
	</div>
{:else if state === 'disconnected'}
	<div
		class="pointer-events-none absolute inset-0 z-[9999] flex flex-col items-center justify-center gap-4 bg-[#111827]/90 backdrop-blur-sm"
	>
		<svg
			class="h-10 w-10 text-red-400"
			xmlns="http://www.w3.org/2000/svg"
			fill="none"
			viewBox="0 0 24 24"
			stroke="currentColor"
			stroke-width="1.5"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
			/>
		</svg>
		<div class="flex flex-col items-center gap-1">
			<span class="text-sm font-semibold tracking-wide text-red-400">Disconnected</span>
			<span class="text-xs text-gray-500">Stream interrupted — reconnecting…</span>
		</div>
	</div>
{/if}
