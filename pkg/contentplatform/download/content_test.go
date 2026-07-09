package download

import "testing"

func TestNewContentDefaultsMissingAuthorToPlatform(t *testing.T) {
	content := NewContent(ContentSummary{
		Platform: "youku",
		Type:     "tv",
		ID:       "show_1",
		Title:    "show",
	}, map[string]any{"id": "show_1"}, nil, nil)

	summary := ContentSummaryOf(content)
	if summary.Author != "youku" || summary.AuthorNickname != "youku" {
		t.Fatalf("summary author = %#v", summary)
	}
	if homepage := ContentAuthorHomepageURL(content); homepage != "https://www.youku.com/" {
		t.Fatalf("author homepage = %q", homepage)
	}
	if metadata := ContentMetadataOf(content); metadata["author_homepage_url"] != "https://www.youku.com/" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if output := ContentOutputOf(content); output["author_homepage_url"] != "https://www.youku.com/" {
		t.Fatalf("output = %#v", output)
	}
}

func TestNewContentPreservesFetchedAuthor(t *testing.T) {
	content := NewContent(ContentSummary{
		Platform:       "youtube",
		Type:           "video",
		ID:             "video_1",
		Title:          "video",
		AuthorNickname: "channel",
	}, map[string]any{"id": "video_1"}, nil, nil)

	summary := ContentSummaryOf(content)
	if summary.Author != "channel" || summary.AuthorNickname != "channel" {
		t.Fatalf("summary author = %#v", summary)
	}
	if homepage := ContentAuthorHomepageURL(content); homepage != "" {
		t.Fatalf("author homepage = %q", homepage)
	}
	if metadata := ContentMetadataOf(content); metadata != nil {
		t.Fatalf("metadata should stay nil, got %#v", metadata)
	}
}

func TestContentAuthorHomepageURLPrefersFetchedHomepage(t *testing.T) {
	content := NewContent(ContentSummary{
		Platform: "youtube",
		Type:     "video",
		ID:       "video_1",
		Title:    "video",
	}, map[string]any{"id": "video_1"}, map[string]any{
		"channel_url": "https://www.youtube.com/@channel",
	}, nil)

	if homepage := ContentAuthorHomepageURL(content); homepage != "https://www.youtube.com/@channel" {
		t.Fatalf("author homepage = %q", homepage)
	}
	if output := ContentOutputOf(content); output != nil {
		t.Fatalf("output should stay nil, got %#v", output)
	}
}
