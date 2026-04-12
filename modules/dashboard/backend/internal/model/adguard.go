package model

type AdguardStats struct {
	Queries24h      int          `json:"queries_24h"`
	Blocked24h      int          `json:"blocked_24h"`
	BlockRate       float64      `json:"block_rate"`
	TopBlocked      []TopDomain  `json:"top_blocked"`
	TopClients      []TopClient  `json:"top_clients"`
	QueryDensity24h []DensityBin `json:"query_density_24h"`
}

type TopDomain struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

type TopClient struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

type DensityBin struct {
	StartHour int `json:"start_hour"`
	Queries   int `json:"queries"`
	Blocked   int `json:"blocked"`
}

type QueryLogEntry struct {
	Time         string `json:"time"`
	QuestionName string `json:"question"`
	QuestionType string `json:"question_type"`
	Client       string `json:"client"`
	Upstream     string `json:"upstream"`
	Reason       string `json:"reason"`
	ElapsedMS    int    `json:"elapsed_ms"`
}
