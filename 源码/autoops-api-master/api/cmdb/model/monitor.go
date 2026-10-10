package model

type HostMetrics struct {
	CPUUsage     float64 `json:"cpuUsage"`     // CPU婵炶揪缍€濞夋洟寮妶澶嬪仢闁搞儮鏅滈銉╂煕閹烘垶顥為柣?
	MemoryUsage  float64 `json:"memoryUsage"`  // 闂佸憡鍔曢幊搴ㄦ偤閵婏附濯撮悹鎭掑妽閺嗗繘鏌ｅ鍐╁窛婵炲懌鍔戝畷姘跺幢濡吋啸
	DiskUsage    float64 `json:"diskUsage"`    // 缂佹儳褰為懗璺好鸿箛鏃€濯撮悹鎭掑妽閺嗗繘鏌ｅ鍐╁窛婵炲懌鍔戝畷姘跺幢濡吋啸
	Load5        float64 `json:"load5"`        // 5闂佸憡甯掑Λ婵嬪箰閹捐泛瀵查柤濮愬€楅崺鐘绘偣閹邦喖鏋欐繛?
	OnlineStatus int     `json:"onlineStatus"` // 闂侀潻璐熼崝搴ㄥ吹鎼淬劍鍋愰柤鍝ヮ暯閸?0:闂侀潻璐熼崝搴ㄥ吹?1:缂備礁鍊藉畷鐢稿吹?
}

type PrometheusQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"` // 闂佹椿娼块崝瀣姳閻炵trix缂備緡鍋夐褔鎮?
			Value  []interface{}     `json:"value"`  // 闂佹椿娼块崝瀣姳閻熷崘ctor缂備緡鍋夐褔鎮?
		} `json:"result"`
	} `json:"data"`
}

// HostMetricHistory 婵炴垶鎸剧划顖氣攦閳ь剟鏌熺粙鎸庡闁绘牭缍佸畷銏ゅ幢濡も偓閽戝鏌℃担鍝勵暭鐎?
type HostMetricHistory struct {
	HostID   uint              `json:"hostId"`   // 婵炴垶鎸剧划顖氣攦閳х悂D
	Metric   string            `json:"metric"`   // 闂佸湱顭堝ú锕傛偉閿濆洨灏甸悹鍥皺閳?cpu/memory/disk)
	TimeData []MetricDataPoint `json:"timeData"` // 闂佸搫鍟悥鐓幬涢幐搴㈠劅闊洦鎸搁悘娆撴煛娴ｅ搫顣肩€?
}

// MetricDataPoint 闂佸湱顭堝ú锕傛偉閿濆鏋侀柣妤€鐗嗙粊锕傛煟?
type MetricDataPoint struct {
	Timestamp int64   `json:"timestamp"` // 闂佸搫鍟悥鐓幬涢崸妤€绠?缂?
	Value     float64 `json:"value"`     // 闂佸湱顭堝ú锕傛偉閿濆纾?
}

// AllMetricsHistory 婵炴垶鎸剧划顖氣攦閳ь剟鏌熺喊妯轰壕闂佸搫鐗嗛ˇ鎵偓鍨哺瀵粙宕惰閸у﹪鏌涘▎蹇撶骇闁哄棛鍠栭獮?
type AllMetricsHistory struct {
	HostID         uint              `json:"hostId"`         // 婵炴垶鎸剧划顖氣攦閳х悂D
	CPU            []MetricDataPoint `json:"cpu"`            // CPU婵炶揪缍€濞夋洟寮妶澶嬪仢闁搞儜鍐ㄦ綉闂佸憡鐟ュ鍫曞汲閻旂厧绠?%)
	CPUUser        []MetricDataPoint `json:"cpuUser"`        // CPU闂佹椿娼块崝宥夊春濞戙垹绠戝ù锝呭槻閳诲繘鏌ｉ～顒€濡跨€圭銈稿畷銏ゅ幢濡も偓閽戝鏌℃担鍝勵暭鐎?%)
	CPUSystem      []MetricDataPoint `json:"cpuSystem"`      // CPU闂佸憡鍔曢幊蹇涙偋閹惰棄绠戝ù锝呭槻閳诲繘鏌ｉ～顒€濡跨€圭銈稿畷銏ゅ幢濡も偓閽戝鏌℃担鍝勵暭鐎?%)
	Memory         []MetricDataPoint `json:"memory"`         // 闂佸憡鍔曢幊搴ㄦ偤閵婏附濯撮悹鎭掑妽閺嗗繘鏌ｅ鍐╂悙鐎圭顭峰畷锝夊矗婢跺顔嶉梺?%)
	MemoryUsed     []MetricDataPoint `json:"memoryUsed"`     // 閻庤鐡曠亸娆撳极閵堝绀冮柛娑卞幘閹界娀鏌涘Ο鐑橆棞鐟滄澘鐖煎顐︽偋閸繄銈?GB)
	MemoryFree     []MetricDataPoint `json:"memoryFree"`     // 闂佸憡鐟崹鎶藉极閵堝绀冮柛娑卞幘閹界娀鏌涘Ο鐑橆棞鐟滄澘鐖煎顐︽偋閸繄銈?GB)
	Disk           []MetricDataPoint `json:"disk"`           // 缂佹儳褰為懗璺好鸿箛鏃€濯撮悹鎭掑妽閺嗗繘鏌ｅ鍐╂悙鐎圭顭峰畷锝夊矗婢跺顔嶉梺?%)
	DiskUsed       []MetricDataPoint `json:"diskUsed"`       // 缂佹儳褰為懗璺好鸿箛鎿冨晠闁肩⒈鍓氶弳蹇曠磼鐏炵偓銇濇俊顐㈡健瀹曘垽宕卞Δ鈧拺澶愭煛娴ｅ搫顣肩€?GB)
	DiskTotal      []MetricDataPoint `json:"diskTotal"`      // 缂佹儳褰為懗璺好鸿箛娑樼畱闁煎鍊愰弫鍕⒒閸屾稒缍戠€圭顭峰畷锝夊矗婢跺顔嶉梺?GB)
	DiskReadKB     []MetricDataPoint `json:"diskReadKB"`     // 缂佹儳褰為懗璺好鸿箛鏇熷珰閻犲洦褰冪徊鍧楁⒑椤愩倕鏋旂€?KB/s)
	DiskWriteKB    []MetricDataPoint `json:"diskWriteKB"`    // 缂佹儳褰為懗璺好鸿箛娑樼婵炲棗绻愬鎶芥⒑椤愩倕鏋旂€?KB/s)
	NetworkReceive []MetricDataPoint `json:"networkReceive"` // 缂傚倸鍟崹鍦垝閸洖绠抽柕澶涢檮閺嗩亪姊洪銈呮灁鐎?KB/s)
	NetworkSend    []MetricDataPoint `json:"networkSend"`    // 缂傚倸鍟崹鍦垝閸洖鐭楅柟杈捐吂閸嬫挻绗熼埀顒勫焵椤掑倸鏋旂€?KB/s)
	Load1min       []MetricDataPoint `json:"load1min"`       // 1闂佸憡甯掑Λ婵嬪箰閹捐泛瀵查柤濮愬€楅崺鐘绘偣閹邦喖鏋欐繛?
	Load5min       []MetricDataPoint `json:"load5min"`       // 5闂佸憡甯掑Λ婵嬪箰閹捐泛瀵查柤濮愬€楅崺鐘绘偣閹邦喖鏋欐繛?
	Load15min      []MetricDataPoint `json:"load15min"`      // 15闂佸憡甯掑Λ婵嬪箰閹捐泛瀵查柤濮愬€楅崺鐘绘偣閹邦喖鏋欐繛?
	TotalProcesses []MetricDataPoint `json:"totalProcesses"` // 缂備緡鍨靛畷鐢靛垝閻戞ɑ浜ゆ繛鎴炲焹閺屻倝鏌熼鍌滃笡闁?
	TCPConnections []MetricDataPoint `json:"tcpConnections"` // TCP闁哄鏅濋崑鐐垫暜閹绢喖鏋侀柡澶婄仢閸у﹪鏌涘▎蹇撶骇闁哄棛鍠栭獮?
	UDPConnections []MetricDataPoint `json:"udpConnections"` // UDP闁哄鏅濋崑鐐垫暜閹绢喖鏋侀柡澶婄仢閸у﹪鏌涘▎蹇撶骇闁哄棛鍠栭獮?
	OpenFiles      []MetricDataPoint `json:"openFiles"`      // 缂備緡鍨靛畷鐢靛垝濞差亜绠ラ柟鎯х－绾惧鏌￠崒姘煑婵炲棎鍨藉顐ｅ閺夋垶顏￠梺鍛娒Λ妤勩亹閸洖鏋侀柣妤€鐗嗙粊?
}

type HostServiceMetricsHistory struct {
	HostID      uint              `json:"hostId"`
	ServiceName string            `json:"serviceName"`
	CPU         []MetricDataPoint `json:"cpu"`
	Memory      []MetricDataPoint `json:"memory"`
}

// ProcessMetrics 闁哄鏅滅粙鎾诲煝閼测晝妫憸宥夊春閸℃稒鍎庨柟瀵稿仜娴犳﹢鏌熺粙鎸庡闁?
type ProcessMetrics struct {
	PID        uint              `json:"pid"`        // 闁哄鏅滅粙鎾诲煝缁粚
	Name       string            `json:"name"`       // 闁哄鏅滅粙鎾诲煝婵傜瑙︾€广儱娉?
	CPUPercent []MetricDataPoint `json:"cpuPercent"` // CPU婵炶揪缍€濞夋洟寮妶澶嬪剬闁诡垎鍐偓缁樻叏閿濆棙鐓ョ€圭顭峰畷锝夊矗婢跺顔嶉梺?
	MemPercent []MetricDataPoint `json:"memPercent"` // 闂佸憡鍔曢幊搴ㄦ偤閵婏附濯撮悹鎭掑妽閺嗗繘鏌ｈ閸庢娊宕规惔鈱掓帡寮崹顔兼綉闂佸憡鐟ュ鍫曞汲閻旂厧绠?
	Host       string            `json:"host"`       // 婵炴垶鎸剧划顖氣攦閳ь剟鏌?
}

// ProcessInfo 闁哄鏅滅粙鎾诲煝婵傜鏄ラ柣鎴炆戦幏鍗炃庨崶锝呭⒉濞?
type ProcessInfo struct {
	PID        uint    `json:"pid"`        // 闁哄鏅滅粙鎾诲煝缁粚
	Name       string  `json:"name"`       // 闁哄鏅滅粙鎾诲煝婵傜瑙︾€广儱娉?
	CPUPercent float64 `json:"cpuPercent"` // CPU婵炶揪缍€濞夋洟寮妶澶嬪剬闁诡垎鍐偓缁樻叏?
	MemPercent float64 `json:"memPercent"` // 闂佸憡鍔曢幊搴ㄦ偤閵婏附濯撮悹鎭掑妽閺嗗繘鏌ｈ閸庢娊宕规惔鈱?
	Host       string  `json:"host"`       // 婵炴垶鎸剧划顖氣攦閳ь剟鏌?
}

// TopProcessesResult 闁哄鏅滅粙鎾诲煝缁摃P闂佸湱鍎ょ敮鐔笺€侀幋鐘电＜闁规儳顕禍?
type TopProcessesResult struct {
	HostID     uint          `json:"hostId"`     // 婵炴垶鎸剧划顖氣攦閳х悂D
	HostName   string        `json:"hostName"`   // 婵炴垶鎸剧划顖氣攦閳ь剟鏌涘顒傂ょ悮?
	TopCPU     []ProcessInfo `json:"topCPU"`     // CPU婵炶揪缍€濞夋洟寮妶澶嬪仢闁搞儜鍐炬瀫5闂佹眹鍔岀€氼垳鎹㈠Ο鑲╃煓?
	TopMemory  []ProcessInfo `json:"topMemory"`  // 闂佸憡鍔曢幊搴ㄦ偤閵婏附濯撮悹鎭掑妽閺嗗繘鏌ｅ鍐╂悙濠?闂佹眹鍔岀€氼垳鎹㈠Ο鑲╃煓?
	UpdateTime int64         `json:"updateTime"` // 闂佸搫娲ら悺銊╁蓟婵犲洤绫嶉柛顐ｆ礃閿涚喖鏌?
}

// PortInfo TCP缂備焦妫忛崹鎷屻亹濞戞瑧鈹嶉柍鈺佸暕缁?
type PortInfo struct {
	Port     string  `json:"port"`     // 缂備焦妫忛崹鎷屻亹濞戙垹鐭?
	PID      string  `json:"pid"`      // 闁哄鏅滅粙鎾诲煝缁粚
	Service  string  `json:"service"`  // 闂佸搫鐗嗙粔瀛樻叏閻旂厧瑙︾€广儱娉?
	Status   int     `json:"status"`   // 闂佺儵鏅滈崹鐢稿箚婢舵劖鍋愰柤鍝ヮ暯閸?1:闂佺儵鏅滈崹鐢稿箚婢跺鈻?0:闂佸搫鐗滄禍鐐裁洪崘顔艰Е?
	CPUUsage float64 `json:"cpuUsage"` // 闁哄鏅滅粙鎾诲煝缁儘U婵炶揪缍€濞夋洟寮妶澶嬪仢闁搞儮鏅滈銉╂煕閹烘垶顥為柣?
	MemUsage float64 `json:"memUsage"` // 闁哄鏅滅粙鎾诲煝婵傜绀冮柛娑卞幘閹界姴霉閿濆牊纭堕柡浣靛€濋幃鎶藉炊閳哄偆浠撮梺鍛婂笒濡瑩鎯?
}

// HostPortsResult 婵炴垶鎸剧划顖氣攦閳ь剛绱掗弮鎴濈仩鐟滅増绋掔粚閬嶅焺閸愌呯缂傚倷鐒﹂幐濠氭倶?
type HostPortsResult struct {
	HostID     uint       `json:"hostId"`     // 婵炴垶鎸剧划顖氣攦閳х悂D
	HostName   string     `json:"hostName"`   // 婵炴垶鎸剧划顖氣攦閳ь剟鏌涘顒傂ょ悮?
	Ports      []PortInfo `json:"ports"`      // 缂備焦妫忛崹鎷屻亹濞戙垹绀嗘俊銈呭閳?
	Total      int        `json:"total"`      // 缂備焦妫忛崹鎷屻亹濞戙垹绠戠紓浣股戝▓?
	UpdateTime int64      `json:"updateTime"` // 闂佸搫娲ら悺銊╁蓟婵犲洤绫嶉柛顐ｆ礃閿涚喖鏌?
}

// HostUsageRank 婵炴垶鎸剧划顖氣攦閳ь剙霉閿濆牊纭堕柡浣靛€濋幃鎶藉炊閵娿儳鐟╅梺?
type HostUsageRank struct {
	HostID       uint    `json:"hostId"`       // 婵炴垶鎸剧划顖氣攦閳х悂D
	HostName     string  `json:"hostName"`     // 婵炴垶鎸剧划顖氣攦閳ь剟鏌涘顒傂ょ悮?
	IP           string  `json:"ip"`           // IP闂侀潻闄勫妯侯焽?
	AvgUsage     float64 `json:"avgUsage"`     // 濡ょ姷鍋涢崯鍨焽鎼淬垺濯撮悹鎭掑妽閺嗗繘鏌?
	MaxUsage     float64 `json:"maxUsage"`     // 闂佸搫鐗冮崑鎾愁熆閸棗鍊搁埢蹇涙煟椤剙濡跨€?
	CurrentUsage float64 `json:"currentUsage"` // 閻熸粎澧楅幐鍛婃櫠閻樿櫕濯撮悹鎭掑妽閺嗗繘鏌?
}

// TopUsageHostsResult 婵炶揪缍€濞夋洟寮妶澶嬪仢闁搞埄妾縋婵炴垶鎸剧划顖氣攦閳ь剛绱撴担瑙勫鞍闁?
type TopUsageHostsResult struct {
	TopCPU    []HostUsageRank `json:"topCpu"`    // CPU婵炶揪缍€濞夋洟寮妶澶嬪仢闁搞儜鍐炬瀫N闂佹眹鍔岀€氼亞鈧潧鐭傚?
	TopMemory []HostUsageRank `json:"topMemory"` // 闂佸憡鍔曢幊搴ㄦ偤閵婏附濯撮悹鎭掑妽閺嗗繘鏌ｅ鍐╂悙濠⒀呭殬闂佹眹鍔岀€氼亞鈧潧鐭傚?
	TopDisk   []HostUsageRank `json:"topDisk"`   // 缂佹儳褰為懗璺好鸿箛鏃€濯撮悹鎭掑妽閺嗗繘鏌ｅ鍐╂悙濠⒀呭殬闂佹眹鍔岀€氼亞鈧潧鐭傚?
	Period    string          `json:"period"`    // 缂傚倷鑳堕崰鏇㈩敇閹间礁宸濋柕濠忛檮閸╁倿鏌ㄥ☉妯煎妞?15婵?闂?
	StartTime string          `json:"startTime"` // 閻庢鍠掗崑鎾斥攽椤旂⒈鍎愭俊鐐插€垮?
	EndTime   string          `json:"endTime"`   // 缂傚倷鐒﹂幐璇差焽椤愶箑绫嶉柛顐ｆ礃閿?
}

// HostCurrentUsageRank current usage rank item
type HostCurrentUsageRank struct {
	HostID   uint    `json:"hostId"`
	HostName string  `json:"hostName"`
	Instance string  `json:"instance"`
	IP       string  `json:"ip"`
	Value    float64 `json:"value"`
}

// CurrentUsageRankingsResult current rankings for cpu/memory/disk(root)
type CurrentUsageRankingsResult struct {
	Limit      int                    `json:"limit"`
	MaxCPU     []HostCurrentUsageRank `json:"maxCpu"`
	MinCPU     []HostCurrentUsageRank `json:"minCpu"`
	MaxMemory  []HostCurrentUsageRank `json:"maxMemory"`
	MinMemory  []HostCurrentUsageRank `json:"minMemory"`
	MaxDisk    []HostCurrentUsageRank `json:"maxDisk"`
	MinDisk    []HostCurrentUsageRank `json:"minDisk"`
	UpdateTime int64                  `json:"updateTime"`
}

// CompleteHostMetrics 闁诲海鎳撻張顒勫汲閿濆鍎嶉柛鏇ㄤ簻閻︽粓鏌￠崼锝嗩仩婵炴彃鍟撮獮鎺曨槼闁哄棛鍠栭獮?
type CompleteHostMetrics struct {
	HostID   uint   `json:"hostId"`   // 婵炴垶鎸剧划顖氣攦閳х悂D
	HostName string `json:"hostName"` // 婵炴垶鎸剧划顖氣攦閳ь剟鏌涘顒傂ょ悮?
	IP       string `json:"ip"`       // IP闂侀潻闄勫妯侯焽?

	// CPU闂佺儵鏅濋…鍫ュ矗?
	CPU CPUMetrics `json:"cpu"` // CPU闂佸湱顭堝ú锕傛偉?

	// 闂佸憡鍔曢幊搴ㄦ偤閵娾晜鍎庣紒瀣仢瑜?
	Memory MemoryMetrics `json:"memory"` // 闂佸憡鍔曢幊搴ㄦ偤閵娾晛绠伴柛銉ｅ妿閸?

	// 缂佹儳褰為懗璺好鸿箛娑欏剮缂佸鐏濊ぐ?
	Disk DiskMetrics `json:"disk"` // 缂佹儳褰為懗璺好鸿箛娑樼闁搞儯鍔庨崹?

	// 闁荤姵鍔楅崰鏇灻?
	Load LoadMetrics `json:"load"` // 闁荤姵鍔楅崰鏇灻归崶顒€绠伴柛銉ｅ妿閸?

	// 闁哄鏅濋崑鐐垫暜閹绢喖鏋?
	Connections ConnMetrics `json:"connections"` // 闁哄鏅濋崑鐐垫暜閹绢喖鏋侀柣妤€鐗嗛惁褰掓煛?

	// 缂傚倸鍟崹鍦垝?
	Network NetworkMetrics `json:"network"` // 缂傚倸鍟崹鍦垝閸洖绠伴柛銉ｅ妿閸?

	// 闂佸搫鐗嗙粔瀛樻叏?
	Services []ServiceInfo `json:"services"` // 闂佸搫鐗嗙粔瀛樻叏閻旂厧绀嗘俊銈呭閳?

	// 缂備緡鍨靛畷鐢靛垝閻戞ɑ浜ら柟閭﹀灱閺€浠嬫煛閸愩劎鍩ｆ俊?
	Uptime    float64 `json:"uptime"`    // 缂備緡鍨靛畷鐢靛垝閻戞ɑ浜ら柟閭﹀灱閺€浠嬫煛閸愩劎鍩ｆ俊?婵?
	UptimeStr string  `json:"uptimeStr"` // 缂備緡鍨靛畷鐢靛垝閻戞ɑ浜ら柟閭﹀灱閺€浠嬫煛閸愩劎鍩ｆ俊?闂佸搫绉堕崢褏妲愰敓鐘茬闁哄诞鍛喕缂備焦顨愮紓姘辨啺?

	// 缂備緡鍨靛畷鐢靛垝閻戞ɑ浜ゆ繛鎴炲焹閺屻倝鏌熼鍌滃笡闁?
	ProcessCount int `json:"processCount"` // 缂備緡鍨靛畷鐢靛垝閻戞ɑ浜ゆ繛鎴炲焹閺屻倝鏌熼鍌滃笡闁?

	// 闂佸搫鍊稿ú锝呪枎閵忋倕绠甸煫鍥ㄨ壘閻楁氨绱掑Δ瀣闁哄棛鍠栭弻?
	OpenFiles int `json:"openFiles"` // 闂佺懓鐏氶幐鍝ユ閹达附鍎嶉柛鏇ㄥ墯閻庮喖霉閻樺搫鐓愮€殿喗瀵у濠氬箥椤曞懎鍓ㄩ梺杞版閸楁娊宕?

	UpdateTime int64 `json:"updateTime"` // 闂佸搫娲ら悺銊╁蓟婵犲洤绫嶉柛顐ｆ礃閿涚喖鏌?
}

// CPUMetrics CPU闂佸湱顭堝ú锕傛偉?
type CPUMetrics struct {
	Total  float64 `json:"total"`  // CPU闂佽鍓濋濠冪箾閸ヮ剚鍋ㄩ柕濞у啳濮?%)
	User   float64 `json:"user"`   // 闂佹椿娼块崝宥夊春濞戙垹绠戝☉鎾虫憠U婵炶揪缍€濞夋洟寮妶澶嬪仢?%)
	System float64 `json:"system"` // 闂佸憡鍔曢幊蹇涙偋閹惰棄绠戝☉鎾虫憠U婵炶揪缍€濞夋洟寮妶澶嬪仢?%)
}

// MemoryMetrics 闂佸憡鍔曢幊搴ㄦ偤閵娾晛绠伴柛銉ｅ妿閸?
type MemoryMetrics struct {
	Used        uint64  `json:"used"`        // 閻庤鐡曞鎾寸箾閸ヮ剚鍋ㄩ柕濞垮劚閺佸爼鎮?bytes)
	UsedStr     string  `json:"usedStr"`     // 閻庤鐡曞鎾寸箾閸ヮ剚鍋ㄩ柕濞垮劚閺佸爼鎮?闂佸搫绉堕崢褏妲愰敓鐘茬闁哄诞鍛喕缂備焦顨愮紓姘辨啺?
	Total       uint64  `json:"total"`       // 闂佽鍓濋褔宕€电硶鍋?bytes)
	TotalStr    string  `json:"totalStr"`    // 闂佽鍓濋褔宕€电硶鍋?闂佸搫绉堕崢褏妲愰敓鐘茬闁哄诞鍛喕缂備焦顨愮紓姘辨啺?
	Free        uint64  `json:"free"`        // 闂佸憡鐟崹鎶藉极閵堝绀冮柛娑卞幘閹?bytes)
	FreeStr     string  `json:"freeStr"`     // 闂佸憡鐟崹鎶藉极閵堝绀冮柛娑卞幘閹?闂佸搫绉堕崢褏妲愰敓鐘茬闁哄诞鍛喕缂備焦顨愮紓姘辨啺?
	UsageRate   float64 `json:"usageRate"`   // 闂佸憡鍔曢幊搴ㄦ偤閵婏附濯撮悹鎭掑妽閺嗗繘鏌?%)
	SwapUsed    uint64  `json:"swapUsed"`    // 婵炲瓨鍤庨崐鏍х暦閺屻儱绀冮柛娑卞幘閹界姴霉閿濆牊纭堕柡浣靛€濋弻?bytes)
	SwapUsedStr string  `json:"swapUsedStr"` // 婵炲瓨鍤庨崐鏍х暦閺屻儱绀冮柛娑卞幘閹界姴霉閿濆牊纭堕柡浣靛€濋弻?闂佸搫绉堕崢褏妲愰敓鐘茬闁哄诞鍛喕缂備焦顨愮紓姘辨啺?
}

// LoadMetrics 闁荤姵鍔楅崰鏇灻归崶顒€绠伴柛銉ｅ妿閸?
type LoadMetrics struct {
	Load1       float64 `json:"load1"`       // 1闂佸憡甯掑Λ婵嬪箰閹捐埖瀚婚柣鏃傜帛缁?
	Load5       float64 `json:"load5"`       // 5闂佸憡甯掑Λ婵嬪箰閹捐埖瀚婚柣鏃傜帛缁?
	Load15      float64 `json:"load15"`      // 15闂佸憡甯掑Λ婵嬪箰閹捐埖瀚婚柣鏃傜帛缁?
	LoadRatio1  float64 `json:"loadRatio1"`  // 1闂佸憡甯掑Λ婵嬪箰閹捐埖瀚婚柣鏃傜帛缁佺増鎱?闁荤姵鍔楅崰鏇灻?CPU闂佸搫绉撮幉锟犲汲?
	LoadRatio5  float64 `json:"loadRatio5"`  // 5闂佸憡甯掑Λ婵嬪箰閹捐埖瀚婚柣鏃傜帛缁佺増鎱?
	LoadRatio15 float64 `json:"loadRatio15"` // 15闂佸憡甯掑Λ婵嬪箰閹捐埖瀚婚柣鏃傜帛缁佺増鎱?
	CPUCores    int     `json:"cpuCores"`    // CPU闂佸搫绉堕…鍫㈢紦妤ｅ啫鏋?
}

// ConnMetrics 闁哄鏅濋崑鐐垫暜閹绢喖鏋侀柣妤€鐗嗛惁褰掓煛?
type ConnMetrics struct {
	TCP   int `json:"tcp"`   // TCP闁哄鏅濋崑鐐垫暜閹绢喖鏋?
	UDP   int `json:"udp"`   // UDP闁哄鏅濋崑鐐垫暜閹绢喖鏋?
	Total int `json:"total"` // 闂佽鍓氬Σ鎺旀崲濞戙垹绠抽柕澶涢檮濞?
}

// DiskMetrics 缂佹儳褰為懗璺好鸿箛娑樼闁搞儯鍔庨崹?
type DiskMetrics struct {
	UsageRate      float64               `json:"usageRate"`      // 缂佹儳褰為懗璺好鸿箛鏃€濯撮悹鎭掑妽閺嗗繘鏌?%)
	Used           uint64                `json:"used"`           // 缂佹儳褰為懗璺好鸿箛鏃€濯撮悹鎭掑妽閺嗗繘姊?bytes)
	UsedStr        string                `json:"usedStr"`        // 缂佹儳褰為懗璺好鸿箛鏃€濯撮悹鎭掑妽閺嗗繘姊?闂佸搫绉堕崢褏妲愰敓鐘茬闁哄诞鍛喕缂備焦顨愮紓姘辨啺?
	Free           uint64                `json:"free"`           // 缂佹儳褰為懗璺好鸿箛娑樼闁宠　鍋撶紓宥嗗灴閺?bytes)
	FreeStr        string                `json:"freeStr"`        // 缂佹儳褰為懗璺好鸿箛娑樼闁宠　鍋撶紓宥嗗灴閺?闂佸搫绉堕崢褏妲愰敓鐘茬闁哄诞鍛喕缂備焦顨愮紓姘辨啺?
	Total          uint64                `json:"total"`          // 缂佹儳褰為懗璺好鸿箛娑樼畱濠电姵鑹惧▍?bytes)
	TotalStr       string                `json:"totalStr"`       // 缂佹儳褰為懗璺好鸿箛娑樼畱濠电姵鑹惧▍?闂佸搫绉堕崢褏妲愰敓鐘茬闁哄诞鍛喕缂備焦顨愮紓姘辨啺?
	DiskThroughput DiskThroughputMetrics `json:"diskThroughput"` // 缂佹儳褰為懗璺好鸿箛娑樿Е闁绘劕寮堕崐鐑芥⒑?
	DiskIOPS       DiskIOPSMetrics       `json:"diskIops"`       // 缂佹儳褰為懗璺好哄鎿甈S
	InodeUsage     float64               `json:"inodeUsage"`     // Inode婵炶揪缍€濞夋洟寮妶澶嬪仢?%)
	InodeTotal     uint64                `json:"inodeTotal"`     // Inode闂佽鍓涚划顖炲汲?
	InodeUsed      uint64                `json:"inodeUsed"`      // Inode閻庤鐡曠亸娆撳极閵堝鏋?
	InodeFree      uint64                `json:"inodeFree"`      // Inode闂佸憡鐟崹鎶藉极閵堝鏋?
}

// DiskThroughputMetrics 缂佹儳褰為懗璺好鸿箛娑樿Е闁绘劕寮堕崐鐑芥⒑閹绘帞绠查悗鍨哺瀵?
type DiskThroughputMetrics struct {
	ReadKBPerSec  float64 `json:"readKbPerSec"`  // 闁荤姴娲╅褑銇愰崶顒佺劵闁绘柨鎼懞?KB/s)
	WriteKBPerSec float64 `json:"writeKbPerSec"` // 闂佸憡鍔栭悷銉╁矗閸℃稒鐒婚柣鏂挎惈閼?KB/s)
}

// DiskIOPSMetrics 缂佹儳褰為懗璺好哄鎿甈S闂佸湱顭堝ú锕傛偉?
type DiskIOPSMetrics struct {
	ReadPerSec  float64 `json:"readPerSec"`  // 闁荤姴娲ㄩ惈渚絇S(濠?s)
	WritePerSec float64 `json:"writePerSec"` // 闂佸憡鍔栭埞鍍孭S(濠?s)
}

// NetworkMetrics 缂傚倸鍟崹鍦垝閸洖绠伴柛銉ｅ妿閸?
type NetworkMetrics struct {
	ReceiveKBPerSec float64 `json:"receiveKbPerSec"` // 闂佽浜介崕鏌ュ极瑜版帗鐒婚柣鏂挎惈閼?KB/s)
	SendKBPerSec    float64 `json:"sendKbPerSec"`    // 闂佸憡鐟﹂崹鍧楀焵椤戣法绐旈柍褜鍓涢崰鎾愁啅?KB/s)
}

// ServiceInfo 闂佸搫鐗嗙粔瀛樻叏閻斿摜鈹嶉柍鈺佸暕缁?
type ServiceInfo struct {
	Name   string  `json:"name"`   // 闂佸搫鐗嗙粔瀛樻叏閻旂厧瑙︾€广儱娉?
	CPU    float64 `json:"cpu"`    // CPU婵炶揪缍€濞夋洟寮妶澶嬪仢?%)
	Memory float64 `json:"memory"` // 闂佸憡鍔曢幊搴ㄦ偤閵婏附濯撮悹鎭掑妽閺嗗繘鏌?%)
}
