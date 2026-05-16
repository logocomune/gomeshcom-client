export type HardwareInfo = {
	id: number;
	name: string;
	humanName: string;
};

// Source: docs/hardware-ids.md, derived from MeshCom-Firmware configuration_global.h.
export const HARDWARE_BY_ID: Record<number, HardwareInfo> = {
	1: { id: 1, name: 'TLORA_V2', humanName: 'T-Lora V2' },
	2: { id: 2, name: 'TLORA_V1', humanName: 'T-Lora V1' },
	3: { id: 3, name: 'TLORA_V2_1_1p6', humanName: 'T-Lora V2.1 1.6' },
	4: { id: 4, name: 'TBEAM', humanName: 'T-Beam' },
	5: { id: 5, name: 'TBEAM_1268', humanName: 'T-Beam 1268' },
	6: { id: 6, name: 'TBEAM_0p7', humanName: 'T-Beam 0.7' },
	7: { id: 7, name: 'T_ECHO', humanName: 'T-Echo' },
	8: { id: 8, name: 'T_DECK', humanName: 'T-Deck' },
	9: { id: 9, name: 'RAK4631', humanName: 'RAK4631' },
	10: { id: 10, name: 'HELTEC_V2_1', humanName: 'Heltec V2.1' },
	11: { id: 11, name: 'HELTEC_V1', humanName: 'Heltec V1' },
	12: { id: 12, name: 'TBEAM_AXP2101', humanName: 'T-Beam AXP2101' },
	39: { id: 39, name: 'EBYTE_E22', humanName: 'Ebyte E22' },
	40: { id: 40, name: 'T5_EPAPER', humanName: 'T5 E-Paper' },
	41: { id: 41, name: 'HELTEC_TRACKER', humanName: 'Heltec Tracker' },
	42: { id: 42, name: 'HELTEC_STICK_V3', humanName: 'Heltec Stick V3' },
	43: { id: 43, name: 'HELTEC_V3', humanName: 'Heltec V3' },
	44: { id: 44, name: 'HELTEC_E290', humanName: 'Heltec E290' },
	45: { id: 45, name: 'TBEAM_1262', humanName: 'T-Beam 1262' },
	46: { id: 46, name: 'T_DECK_PLUS', humanName: 'T-Deck Plus' },
	47: { id: 47, name: 'TBEAM_SUPREME', humanName: 'T-Beam Supreme' },
	48: { id: 48, name: 'ESP32_S3_EBYTE_E22', humanName: 'ESP32-S3 Ebyte E22' },
	49: { id: 49, name: 'TLORA_PAGER', humanName: 'T-Lora Pager' },
	50: { id: 50, name: 'T_DECK_PRO', humanName: 'T-Deck Pro' },
	51: { id: 51, name: 'TBEAM_1W', humanName: 'T-Beam 1W' },
	52: { id: 52, name: 'HELTEC_V4', humanName: 'Heltec V4' },
	53: { id: 53, name: 'T_ETH_ELITE_1262', humanName: 'T-Eth Elite 1262' }
};

export function hardwareInfo(hwId: string | number | undefined): HardwareInfo | null {
	if (hwId == null) return null;
	const id = typeof hwId === 'number' ? hwId : Number.parseInt(hwId, 10);
	if (!Number.isFinite(id)) return null;
	return HARDWARE_BY_ID[id] ?? { id, name: `HW_${id}`, humanName: `HW ${id}` };
}

export function hardwareHumanName(hwId: string | number | undefined): string {
	return hardwareInfo(hwId)?.humanName ?? '';
}
