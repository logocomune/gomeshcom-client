const STORAGE_CHAT_CHANNELS_COLLAPSED = 'meshcom:chatChannelsCollapsed';
const STORAGE_SHOW_TIME_BEACONS = 'meshcom:chatShowTimeBeacons';

type LayoutStorage = Pick<Storage, 'getItem' | 'setItem'>;

export function loadChatChannelsCollapsed(storage: LayoutStorage): boolean {
	return storage.getItem(STORAGE_CHAT_CHANNELS_COLLAPSED) === '1';
}

export function saveChatChannelsCollapsed(storage: LayoutStorage, collapsed: boolean): void {
	storage.setItem(STORAGE_CHAT_CHANNELS_COLLAPSED, collapsed ? '1' : '0');
}

export function loadShowTimeBeacons(storage: LayoutStorage): boolean {
	return storage.getItem(STORAGE_SHOW_TIME_BEACONS) === '1';
}

export function saveShowTimeBeacons(storage: LayoutStorage, showTimeBeacons: boolean): void {
	storage.setItem(STORAGE_SHOW_TIME_BEACONS, showTimeBeacons ? '1' : '0');
}

export function chatSidebarGridColumns(collapsed: boolean): string {
	return collapsed ? '3rem minmax(0, 1fr)' : '10rem minmax(0, 1fr)';
}

export function chatSidebarNewDmLabel(collapsed: boolean): string {
	return collapsed ? 'DM' : 'New DM';
}

export function chatSidebarGridStyle(collapsed: boolean, desktop: boolean): string {
	if (!desktop && !collapsed) {
		return '';
	}

	return `grid-template-columns: ${chatSidebarGridColumns(collapsed)}`;
}
