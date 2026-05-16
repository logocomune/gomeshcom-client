<script lang="ts">
	import { base } from '$app/paths';
	const goLibs = [
		{
			name: 'ardanlabs/conf',
			version: 'v3.11.0',
			url: 'https://github.com/ardanlabs/conf',
			license: 'MIT',
			desc: 'Environment and CLI configuration'
		},
		{
			name: 'hashicorp/golang-lru',
			version: 'v2.0.7',
			url: 'https://github.com/hashicorp/golang-lru',
			license: 'MPL-2.0',
			desc: 'LRU cache with TTL expiry'
		}
	];

	const jsLibs = [
		{
			name: 'Svelte / SvelteKit',
			version: '5.x / 2.x',
			url: 'https://svelte.dev',
			license: 'MIT',
			desc: 'UI framework and application shell'
		},
		{
			name: 'OpenLayers',
			version: '10.x',
			url: 'https://openlayers.org',
			license: 'BSD-2-Clause',
			desc: 'Interactive map engine'
		},
		{
			name: 'ol-ext',
			version: '4.x',
			url: 'https://viglino.github.io/ol-ext',
			license: 'BSD-3-Clause',
			desc: 'OpenLayers extensions (Maidenhead grid, night overlay)'
		},
		{
			name: 'Material Design Icons (@mdi/js)',
			version: '7.x',
			url: 'https://pictogrammers.com/library/mdi',
			license: 'Apache-2.0',
			desc: 'Icon set'
		},
		{
			name: 'Tailwind CSS',
			version: '4.x',
			url: 'https://tailwindcss.com',
			license: 'MIT',
			desc: 'Utility-first CSS framework'
		},
		{
			name: 'Vite',
			version: '8.x',
			url: 'https://vite.dev',
			license: 'MIT',
			desc: 'Build tool and dev server'
		},
		{
			name: 'TypeScript',
			version: '6.x',
			url: 'https://www.typescriptlang.org',
			license: 'Apache-2.0',
			desc: 'Typed JavaScript'
		},
		{
			name: 'fast-check',
			version: '4.x',
			url: 'https://fast-check.dev',
			license: 'MIT',
			desc: 'Property-based testing'
		},
		{
			name: 'Vitest',
			version: '4.x',
			url: 'https://vitest.dev',
			license: 'MIT',
			desc: 'Unit test framework'
		},
		{
			name: 'Playwright',
			version: '1.x',
			url: 'https://playwright.dev',
			license: 'Apache-2.0',
			desc: 'End-to-end test framework'
		},
		{
			name: 'Prettier',
			version: '3.x',
			url: 'https://prettier.io',
			license: 'MIT',
			desc: 'Code formatter'
		},
		{
			name: 'ESLint',
			version: '10.x',
			url: 'https://eslint.org',
			license: 'MIT',
			desc: 'JavaScript / TypeScript linter'
		}
	];
</script>

<svelte:head>
	<title>Credits — gomeshcom</title>
</svelte:head>

<main class="min-h-screen bg-[#111827] px-4 py-10 text-gray-100">
	<div class="mx-auto max-w-2xl">
		<a
			href="{base}/"
			class="mb-8 inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-blue-400"
		>
			← Back
		</a>

		<h1 class="mb-1 font-mono text-2xl font-bold text-blue-300">gomeshcom</h1>
		<p class="mb-8 text-sm text-gray-400">
			A MeshCom UDP gateway client — open-source software built on open-source shoulders.
		</p>

		<!-- MeshCom project -->
		<section class="mb-10 rounded-lg border border-blue-500/30 bg-blue-500/5 p-5">
			<h2 class="mb-2 text-xs font-semibold uppercase tracking-wider text-blue-400">
				MeshCom Project
			</h2>
			<p class="mb-3 text-sm text-gray-300">
				This software interfaces with the <strong class="text-white">MeshCom</strong> LoRa mesh network
				developed by OE1KBC and the Austrian amateur radio community (ÖVSV / ICSSW).
			</p>
			<a
				href="https://icssw.org/en/meshcom/"
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1.5 rounded border border-blue-500/40 bg-blue-500/10 px-3 py-1.5 text-xs font-medium text-blue-300 hover:bg-blue-500/20"
			>
				icssw.org/en/meshcom ↗
			</a>
		</section>

		<!-- Go backend -->
		<section class="mb-8">
			<h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500">
				Go backend libraries
			</h2>
			<div class="flex flex-col gap-2">
				<!-- Go stdlib / runtime -->
				<div class="rounded border border-gray-700/60 bg-[#1c2230] px-4 py-3">
					<div class="flex items-baseline justify-between gap-2">
						<a
							href="https://go.dev"
							target="_blank"
							rel="noopener noreferrer"
							class="font-mono text-sm font-semibold text-gray-200 hover:text-blue-300"
						>
							Go
						</a>
						<span class="shrink-0 text-xs text-gray-600">1.26 · BSD-3-Clause</span>
					</div>
					<p class="mt-0.5 text-xs text-gray-500">Language runtime, standard library, toolchain</p>
				</div>
				{#each goLibs as lib (lib.name)}
					<div class="rounded border border-gray-700/60 bg-[#1c2230] px-4 py-3">
						<div class="flex items-baseline justify-between gap-2">
							<a
								href={lib.url}
								target="_blank"
								rel="noopener noreferrer"
								class="font-mono text-sm font-semibold text-gray-200 hover:text-blue-300"
							>
								{lib.name}
							</a>
							<span class="shrink-0 text-xs text-gray-600">{lib.version} · {lib.license}</span>
						</div>
						<p class="mt-0.5 text-xs text-gray-500">{lib.desc}</p>
					</div>
				{/each}
			</div>
		</section>

		<!-- JS frontend -->
		<section class="mb-8">
			<h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500">
				Frontend libraries
			</h2>
			<div class="flex flex-col gap-2">
				{#each jsLibs as lib (lib.name)}
					<div class="rounded border border-gray-700/60 bg-[#1c2230] px-4 py-3">
						<div class="flex items-baseline justify-between gap-2">
							<a
								href={lib.url}
								target="_blank"
								rel="noopener noreferrer"
								class="font-mono text-sm font-semibold text-gray-200 hover:text-blue-300"
							>
								{lib.name}
							</a>
							<span class="shrink-0 text-xs text-gray-600">{lib.version} · {lib.license}</span>
						</div>
						<p class="mt-0.5 text-xs text-gray-500">{lib.desc}</p>
					</div>
				{/each}
			</div>
		</section>

		<!-- Map data -->
		<section class="mb-8">
			<h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500">Map data</h2>
			<div class="rounded border border-gray-700/60 bg-[#1c2230] px-4 py-3">
				<div class="flex items-baseline justify-between gap-2">
					<a
						href="https://www.openstreetmap.org/copyright"
						target="_blank"
						rel="noopener noreferrer"
						class="font-mono text-sm font-semibold text-gray-200 hover:text-blue-300"
					>
						OpenStreetMap
					</a>
					<span class="shrink-0 text-xs text-gray-600">© OpenStreetMap contributors · ODbL</span>
				</div>
				<p class="mt-0.5 text-xs text-gray-500">Tile data and cartography used for the node map</p>
			</div>
		</section>

		<p class="text-center text-xs text-gray-600">
			gomeshcom is not affiliated with or endorsed by the MeshCom project or ÖVSV/ICSSW.
		</p>
	</div>
</main>
