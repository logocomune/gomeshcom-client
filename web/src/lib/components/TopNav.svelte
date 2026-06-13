<script lang="ts">
	import { page } from '$app/state';
	import { base } from '$app/paths';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { chatState } from '$lib/stores/chat.svelte';
	import { isRouteActive, primaryNavRoutes, routeHref, secondaryNavRoutes } from '$lib/navigation';
	import { mdiChevronDown } from '@mdi/js';

	let openDropdown = $state<string | null>(null);

	function toggleDropdown(href: string) {
		openDropdown = openDropdown === href ? null : href;
	}

	function closeDropdown() {
		openDropdown = null;
	}
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
		{#if item.children}
			{@const active = isRouteActive(page.url.pathname, item.href, base)}
			{@const isOpen = openDropdown === item.href}
			<div class="relative">
				<button
					type="button"
					onclick={() => toggleDropdown(item.href)}
					class="flex h-8 items-center gap-1.5 rounded-md px-2 transition-colors
						{active
						? 'bg-blue-500/15 text-blue-300'
						: 'text-gray-500 hover:bg-gray-700/40 hover:text-gray-300'}"
					aria-haspopup="menu"
					aria-expanded={isOpen}
					title={item.label}
				>
					<MdiIcon path={item.icon} size={18} />
					<span class="hidden xl:inline text-xs">{item.label}</span>
					<span class="transition-transform {isOpen ? 'rotate-180' : ''}">
						<MdiIcon path={mdiChevronDown} size={12} />
					</span>
				</button>

				{#if isOpen}
					<button
						type="button"
						class="fixed inset-0 z-40"
						aria-label="Close menu"
						onclick={closeDropdown}
					></button>
					<div
						class="absolute right-0 top-full z-50 mt-1 min-w-[10rem] overflow-hidden rounded-lg border border-ink-dim/20 bg-surface shadow-xl"
						role="menu"
					>
						{#each item.children as child}
							{@const childActive =
								page.url.pathname === routeHref(child.href, base) ||
								(base
									? page.url.pathname === `${base}${child.href}`
									: page.url.pathname === child.href)}
							<a
								href={routeHref(child.href, base)}
								role="menuitem"
								onclick={closeDropdown}
								class="flex items-center gap-2.5 px-3 py-2 text-xs transition-colors
									{childActive
									? 'bg-blue-500/15 text-blue-300'
									: 'text-gray-300 hover:bg-gray-700/40 hover:text-gray-100'}"
							>
								<MdiIcon path={child.icon} size={16} />
								{child.label}
							</a>
						{/each}
					</div>
				{/if}
			</div>
		{:else}
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
		{/if}
	{/each}
</div>
