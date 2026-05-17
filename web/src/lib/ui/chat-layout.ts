const STORAGE_CHAT_CHANNELS_COLLAPSED = 'meshcom:chatChannelsCollapsed';

type LayoutStorage = Pick<Storage, 'getItem' | 'setItem'>;

export function loadChatChannelsCollapsed(storage: LayoutStorage): boolean {
	return storage.getItem(STORAGE_CHAT_CHANNELS_COLLAPSED) === '1';
}

export function saveChatChannelsCollapsed(storage: LayoutStorage, collapsed: boolean): void {
	storage.setItem(STORAGE_CHAT_CHANNELS_COLLAPSED, collapsed ? '1' : '0');
}

export function chatSidebarGridColumns(collapsed: boolean): string {
	return collapsed ? '3rem minmax(0, 1fr)' : '10rem minmax(0, 1fr)';
}

export function chatSidebarNewDmLabel(collapsed: boolean): string {
	return collapsed ? 'DM' : 'New DM';
}
