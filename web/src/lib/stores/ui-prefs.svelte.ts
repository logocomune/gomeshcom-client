const STORAGE_KEY = 'gomeshcom.ui-prefs';

type UiPrefs = {
	soundEnabled: boolean;
	dmSoundEnabled: boolean;
	dmToastEnabled: boolean;
	mentionToastEnabled: boolean;
	compactMode: boolean;
	timestampFormat: 'relative' | 'absolute';
	showPacketCounter: boolean;
};

const defaults: UiPrefs = {
	soundEnabled: true,
	dmSoundEnabled: true,
	dmToastEnabled: true,
	mentionToastEnabled: true,
	compactMode: false,
	timestampFormat: 'absolute',
	showPacketCounter: true
};

function loadPrefs(): UiPrefs {
	if (typeof localStorage === 'undefined') return { ...defaults };
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return { ...defaults };
		return { ...defaults, ...JSON.parse(raw) };
	} catch {
		return { ...defaults };
	}
}

class UiPrefsStore {
	soundEnabled = $state(defaults.soundEnabled);
	dmSoundEnabled = $state(defaults.dmSoundEnabled);
	dmToastEnabled = $state(defaults.dmToastEnabled);
	mentionToastEnabled = $state(defaults.mentionToastEnabled);
	compactMode = $state(defaults.compactMode);
	timestampFormat = $state<UiPrefs['timestampFormat']>(defaults.timestampFormat);
	showPacketCounter = $state(defaults.showPacketCounter);

	constructor() {
		const saved = loadPrefs();
		this.soundEnabled = saved.soundEnabled;
		this.dmSoundEnabled = saved.dmSoundEnabled;
		this.dmToastEnabled = saved.dmToastEnabled;
		this.mentionToastEnabled = saved.mentionToastEnabled;
		this.compactMode = saved.compactMode;
		this.timestampFormat = saved.timestampFormat;
		this.showPacketCounter = saved.showPacketCounter;
	}

	save() {
		if (typeof localStorage === 'undefined') return;
		const prefs: UiPrefs = {
			soundEnabled: this.soundEnabled,
			dmSoundEnabled: this.dmSoundEnabled,
			dmToastEnabled: this.dmToastEnabled,
			mentionToastEnabled: this.mentionToastEnabled,
			compactMode: this.compactMode,
			timestampFormat: this.timestampFormat,
			showPacketCounter: this.showPacketCounter
		};
		localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
	}

	reset() {
		this.soundEnabled = defaults.soundEnabled;
		this.dmSoundEnabled = defaults.dmSoundEnabled;
		this.dmToastEnabled = defaults.dmToastEnabled;
		this.mentionToastEnabled = defaults.mentionToastEnabled;
		this.compactMode = defaults.compactMode;
		this.timestampFormat = defaults.timestampFormat;
		this.showPacketCounter = defaults.showPacketCounter;
		this.save();
	}
}

export const uiPrefs = new UiPrefsStore();
