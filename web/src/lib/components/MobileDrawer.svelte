<script lang="ts">
	import { page } from '$app/state';
	import { base } from '$app/paths';
	import { mdiClose } from '@mdi/js';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { viewState } from '$lib/stores/view.svelte';
	import { chatState } from '$lib/stores/chat.svelte';
	import { isRouteActive, primaryNavRoutes, routeHref, secondaryNavRoutes } from '$lib/navigation';
</script>

{#if viewState.mobileDrawerOpen}
	<button
		type="button"
		class="fixed inset-0 z-[200] bg-black/60 backdrop-blur-sm"
		aria-label="Close menu"
		onclick={() => viewState.closeDrawer()}
	></button>

	<nav
		class="fixed inset-y-0 left-0 z-[201] flex w-64 flex-col border-r border-gray-700/60 bg-[#1c2230] shadow-2xl"
		aria-label="Mobile navigation"
	>
		<div class="flex h-12 shrink-0 items-center justify-between border-b border-gray-700/60 px-4">
			<span class="text-sm font-semibold text-gray-200">Menu</span>
			<button
				type="button"
				class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-gray-500 hover:text-gray-200"
				aria-label="Close menu"
				onclick={() => viewState.closeDrawer()}
			>
				<MdiIcon path={mdiClose} size={18} />
			</button>
		</div>

		<ul class="flex flex-1 flex-col gap-0.5 overflow-y-auto p-2 pt-3">
			{#each primaryNavRoutes as item}
				{@const active = isRouteActive(page.url.pathname, item.href, base)}
				{@const hasUnread = item.href === '/chat' && chatState.visibleUnreadIds.size > 0}
				<li>
					<a
						href={routeHref(item.href, base)}
						class="flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-left text-sm transition-colors
							{active ? 'bg-blue-500/20 text-blue-300' : 'text-gray-300 hover:bg-gray-700/40 hover:text-white'}"
						onclick={() => viewState.closeDrawer()}
						aria-current={active ? 'page' : undefined}
					>
						<span class="relative shrink-0">
							<MdiIcon path={item.icon} size={20} />
							{#if hasUnread}
								<span class="absolute -right-1 -top-1 h-2 w-2 rounded-full bg-blue-400"></span>
							{/if}
						</span>
						<span class="font-medium">{item.label}</span>
					</a>
				</li>
			{/each}
		</ul>

		<div class="mx-3 border-t border-gray-700/40"></div>

		<ul class="flex flex-col gap-0.5 p-2">
			{#each secondaryNavRoutes as item}
				{@const active = isRouteActive(page.url.pathname, item.href, base)}
				<li>
					<a
						href={routeHref(item.href, base)}
						class="flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-left text-sm transition-colors
							{active
							? 'bg-blue-500/20 text-blue-300'
							: 'text-gray-400 hover:bg-gray-700/40 hover:text-gray-200'}"
						onclick={() => viewState.closeDrawer()}
						aria-current={active ? 'page' : undefined}
					>
						<MdiIcon path={item.icon} size={18} />
						<span>{item.label}</span>
					</a>
				</li>
			{/each}
		</ul>
	</nav>
{/if}
