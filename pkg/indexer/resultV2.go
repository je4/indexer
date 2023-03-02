package indexer

type ResultV2 struct {
	Errors    map[string]string `json:"errors,omitempty"`
	Mimetype  string            `json:"mimetype"`
	Mimetypes []string          `json:"mimetypes"`
	Width     uint              `json:"width,omitempty"`
	Height    uint              `json:"height,omitempty"`
	Duration  uint              `json:"duration,omitempty"`
	Size      uint64            `json:"size"`
	Metadata  map[string]any    `json:"metadata"`
}
