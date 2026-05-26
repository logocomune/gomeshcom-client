import {
	mdiAccessPoint,
	mdiInformationOutline,
	mdiMapOutline,
	mdiMessageTextOutline,
	mdiStarOutline,
	mdiViewDashboardOutline,
	mdiWaveform
} from '@mdi/js';

export type NavRoute = {
	href: string;
	icon: string;
	label: string;
};

export const primaryNavRoutes: NavRoute[] = [
	{ href: '/', icon: mdiViewDashboardOutline, label: 'Dashboard' },
	{ href: '/chat', icon: mdiMessageTextOutline, label: 'Chat' },
	{ href: '/map', icon: mdiMapOutline, label: 'Map' },
	{ href: '/nodes', icon: mdiAccessPoint, label: 'Nodes' },
	{ href: '/traffic', icon: mdiWaveform, label: 'Traffic' }
];

export const secondaryNavRoutes: NavRoute[] = [
	{ href: '/about', icon: mdiInformationOutline, label: 'About' },
	{ href: '/credits', icon: mdiStarOutline, label: 'Credits' }
];

export function routeHref(path: string, basePath: string): string {
	return `${basePath}${path}`;
}

export function isRouteActive(pathname: string, href: string, basePath = ''): boolean {
	const current = stripBase(pathname, basePath);
	return current === href || (href !== '/' && current.startsWith(`${href}/`));
}

function stripBase(pathname: string, basePath: string): string {
	if (basePath && pathname.startsWith(basePath)) {
		return pathname.slice(basePath.length) || '/';
	}
	return pathname || '/';
}
