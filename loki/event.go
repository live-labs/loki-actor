package loki

type Stream struct {
	Details map[string]string `json:"stream"`
	Values  [][]string        `json:"values"` // nanosecond unix epoch, log line
}

type Event struct {
	Streams []Stream `json:"streams"`
}
