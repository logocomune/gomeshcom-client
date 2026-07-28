import { describe, expect, it } from 'vitest';
import { emojiCategories, emojiTabs } from './emoji-catalog';

describe('emoji catalog', () => {
	it('contains each requested category', () => {
		expect(emojiCategories.map((category) => category.label)).toEqual([
			'Smiling & Affectionate',
			'Tongues, Hands & Accessories',
			'Neutral & Skeptical',
			'Sleepy & Unwell',
			'Concerned & Negative',
			'Costume, Creature & Animal',
			'Places & Travel'
		]);
	});

	it('contains configured emoji catalog', () => {
		const emojis = emojiCategories.flatMap((category) => category.emojis);
		expect(emojis).toHaveLength(270);
		expect(new Set(emojis)).toHaveLength(270);
	});

	it('groups all face emoji under Smile tab', () => {
		expect(emojiTabs.map((tab) => tab.label)).toEqual([
			'Smile',
			'Animal',
			'Places & Travel',
			'Flags',
			'Objects'
		]);
		expect(emojiTabs[0].emojis).toHaveLength(131);
		expect(emojiTabs[1].emojis).toHaveLength(133);
		expect(emojiTabs[2].emojis).toContain('🚀');
		expect(emojiTabs[3].emojis).toContain('🇮🇹');
		expect(emojiTabs[4].emojis).toContain('📱');
	});
});
