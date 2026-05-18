import { beforeEach, describe, expect, it } from 'vitest';
import {
	chatSidebarGridColumns,
	chatSidebarGridStyle,
	chatSidebarNewDmLabel,
	loadChatChannelsCollapsed,
	saveChatChannelsCollapsed
} from './chat-layout';

describe('chat layout helpers', () => {
	let store: Record<string, string> = {};

	beforeEach(() => {
		store = {};
	});

	function storage(): Pick<Storage, 'getItem' | 'setItem'> {
		return {
			getItem(key: string) {
				return store[key] ?? null;
			},
			setItem(key: string, value: string) {
				store[key] = value;
			}
		};
	}

	it('loads collapsed state from storage', () => {
		store['meshcom:chatChannelsCollapsed'] = '1';
		expect(loadChatChannelsCollapsed(storage())).toBe(true);
	});

	it('defaults to expanded when storage missing', () => {
		expect(loadChatChannelsCollapsed(storage())).toBe(false);
	});

	it('saves collapsed state to storage', () => {
		saveChatChannelsCollapsed(storage(), true);
		expect(store['meshcom:chatChannelsCollapsed']).toBe('1');
		saveChatChannelsCollapsed(storage(), false);
		expect(store['meshcom:chatChannelsCollapsed']).toBe('0');
	});

	it('returns narrower columns when collapsed', () => {
		expect(chatSidebarGridColumns(true)).toBe('3rem minmax(0, 1fr)');
		expect(chatSidebarGridColumns(false)).toBe('10rem minmax(0, 1fr)');
	});

	it('keeps collapsed sidebar columns on mobile', () => {
		expect(chatSidebarGridStyle(true, false)).toBe('grid-template-columns: 3rem minmax(0, 1fr)');
	});

	it('keeps expanded mobile layout stacked', () => {
		expect(chatSidebarGridStyle(false, false)).toBe('');
	});

	it('shortens new dm label when collapsed', () => {
		expect(chatSidebarNewDmLabel(true)).toBe('DM');
		expect(chatSidebarNewDmLabel(false)).toBe('New DM');
	});
});
