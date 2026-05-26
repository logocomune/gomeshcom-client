<script lang="ts">
	import { page } from '$app/state';
	import { base } from '$app/paths';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { chatState } from '$lib/stores/chat.svelte';
	import { isRouteActive, primaryNavRoutes, routeHref, secondaryNavRoutes } from '$lib/navigation';
</script>

<div class="flex items-center gap-1">
	{#each primaryNavRoutes as item}
		{@const active = isRouteActive(page.url.pathname, item.href, base)}
		{@const hasUnread = item.href === '/chat' && chatState.visibleUnreadIds.size > 0}
		<a
			href={routeHref(item.href, base)}
			class="relative flex h-8 items-center gap-1.5 rounded-md px-2 transition-colors
				{active
				? 'bg-blue-500/15 text-blue-300'
				: 'text-gray-400 hover:bg-gray-700/40 hover:text-gray-200'}"
			aria-current={active ? 'page' : undefined}
			title={item.label}
		>
			<MdiIcon path={item.icon} size={20} />
			<span class="hidden xl:inline text-xs font-medium">{item.label}</span>
			{#if hasUnread}
				<span class="absolute right-0.5 top-0.5 h-2 w-2 rounded-full bg-blue-400"></span>
			{/if}
		</a>
	{/each}

	<div class="mx-1 h-5 w-px bg-gray-600/60"></div>

	{#each secondaryNavRoutes as item}
		{@const active = isRouteActive(page.url.pathname, item.href, base)}
		<a
			href={routeHref(item.href, base)}
			class="flex h-8 items-center gap-1.5 rounded-md px-2 transition-colors
				{active
				? 'bg-blue-500/15 text-blue-300'
				: 'text-gray-500 hover:bg-gray-700/40 hover:text-gray-300'}"
			aria-current={active ? 'page' : undefined}
			title={item.label}
		>
			<MdiIcon path={item.icon} size={18} />
			<span class="hidden xl:inline text-xs">{item.label}</span>
		</a>
	{/each}
</div>
