package shuba69

// Novel contains a novel profile and its chapter directory.
type Novel struct {
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	CoverURL string    `json:"cover_url,omitempty"`
	Author   string    `json:"author,omitempty"`
	Category string    `json:"category,omitempty"`
	Status   string    `json:"status,omitempty"`
	BookID   string    `json:"book_id,omitempty"`
	Chapters []Chapter `json:"chapters,omitempty"`
}

// Chapter contains one chapter directory entry.
type Chapter struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	URL   string `json:"url"`
}
