package meshcom

import "testing"

func FuzzParsePacket(f *testing.F) {
	seeds := []string{
		`{"src_type":"node","type":"msg","src":"QQ1ABC-1","dst":"*","msg":"hello","msg_id":"ABC","firmware":"4.35","fw_sub":"p","rssi":-90,"snr":8}`,
		`{"src_type":"lora","type":"msg","src":"QQ1XAR-32,QQ5PFI-12","dst":"*","msg":"{CET}2026-05-14 19:14:19","msg_id":"6A01DD09","firmware":0,"fw_sub":"#","rssi":-108,"snr":0}`,
		`{"src_type":"node","type":"pos","src":"QQ1ABC-1","msg":"","lat":48,"lat_dir":"N","long":16,"long_dir":"E","aprs_symbol":"#","aprs_symbol_group":"/","hw_id":"MAC","msg_id":"ABC","alt":123,"batt":85,"firmware":"4.35","fw_sub":"p","rssi":-90,"snr":8}`,
		`{"src_type":"lora","type":"pos","src":"QQ5EKX-11,QQ5AKT-10,QQ5PFI-1","msg":"","lat":43.5076,"lat_dir":"N","long":10.3476,"long_dir":"E","aprs_symbol":"&","aprs_symbol_group":"/","hw_id":4,"msg_id":"AB39600F","alt":367,"batt":0,"firmware":35,"fw_sub":"p","rssi":-108,"snr":1}`,
		`{"src_type":"node","type":"tele","src":"QQ1ABC-1","temp1":9.99,"temp2":8.88,"hum":50,"qfe":999.9,"qnh":1001.1,"gas":9.9,"co2":400}`,
		`{"src_type":"lora","type":"tele","src":"QQ5EKX-11,QQ5AKT-10,QQ5PFI-1","batt":0,"temp1":0,"temp2":0,"hum":0,"qfe":0,"qnh":0,"gas":0,"co2":0}`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParsePacket(data)
	})
}
