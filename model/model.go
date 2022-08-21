package model

type URLEntry struct {
	Id          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortedURL  string `json:"short_url"`
	Clicks      int32  `json:"clicks"`
}
