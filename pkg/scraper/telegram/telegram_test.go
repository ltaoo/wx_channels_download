package telegram

import (
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	cases := []struct {
		raw       string
		username  string
		messageID int
		ok        bool
	}{
		{"https://t.me/telegram", "telegram", 0, true},
		{"https://t.me/s/telegram/445", "telegram", 445, true},
		{"https://telegram.me/telegram/445", "telegram", 445, true},
		{"tg://resolve?domain=telegram&post=445", "telegram", 445, true},
		{"@telegram", "telegram", 0, true},
		{"telegram", "telegram", 0, true},
		{"https://t.me/c/12345/6", "", 0, false},
		{"https://example.com/telegram", "", 0, false},
	}

	for _, tc := range cases {
		got, ok := ParseURL(tc.raw)
		if ok != tc.ok {
			t.Fatalf("ParseURL(%q) ok = %v, want %v", tc.raw, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if got.Username != tc.username || got.MessageID != tc.messageID {
			t.Fatalf("ParseURL(%q) = %#v", tc.raw, got)
		}
		if !strings.Contains(got.WebURL, "/s/"+tc.username) {
			t.Fatalf("web url = %q", got.WebURL)
		}
	}
}

func TestParsePageHTML(t *testing.T) {
	page, err := ParsePageHTML("https://t.me/s/testchannel", sampleTelegramHTML)
	if err != nil {
		t.Fatalf("ParsePageHTML: %v", err)
	}
	if page.Channel.Username != "testchannel" || page.Channel.Title != "Test Channel" {
		t.Fatalf("channel = %#v", page.Channel)
	}
	if page.Channel.Counters["subscribers"] != 12500 || page.Channel.Counters["photos"] != 7 {
		t.Fatalf("counters = %#v", page.Channel.Counters)
	}
	if len(page.Messages) != 2 {
		t.Fatalf("messages len = %d", len(page.Messages))
	}

	first := page.Messages[0]
	if first.ID != 11 || first.MediaType != "Photo" {
		t.Fatalf("first message = %#v", first)
	}
	if !strings.Contains(first.ContentText, "Hello Telegram") || !strings.Contains(first.ContentText, "Second line") {
		t.Fatalf("content text = %q", first.ContentText)
	}
	if len(first.Media) != 1 || first.Media[0].Type != "photo" || first.Media[0].URL != "https://cdn.example.com/photo.jpg" {
		t.Fatalf("photo media = %#v", first.Media)
	}
	if first.ViewCount != 1200 || first.PublishedAt != "2026-06-01T10:00:00+00:00" {
		t.Fatalf("message stats = %#v", first)
	}
	if !hasLink(first.Links, "https://example.com/a") || !hasLink(first.Links, "https://t.me/testchannel/10") {
		t.Fatalf("links = %#v", first.Links)
	}

	second := page.Messages[1]
	if second.ID != 12 || second.MediaType != "Video" {
		t.Fatalf("second message = %#v", second)
	}
	if len(second.Media) != 1 || second.Media[0].URL != "https://cdn.example.com/video.mp4?token=1" || second.Media[0].ThumbnailURL != "https://cdn.example.com/thumb.jpg" {
		t.Fatalf("video media = %#v", second.Media)
	}
	if second.LinkPreview == nil || second.LinkPreview.ImageURL != "https://cdn.example.com/preview.jpg" {
		t.Fatalf("link preview = %#v", second.LinkPreview)
	}

	rendered := BuildHTML(page)
	if !strings.Contains(rendered, "<video controls") || !strings.Contains(rendered, "Test Channel") {
		t.Fatalf("rendered html missing expected content: %s", rendered)
	}
}

func TestParsePageHTMLSingleMessage(t *testing.T) {
	page, err := ParsePageHTML("https://t.me/testchannel/12", sampleTelegramHTML)
	if err != nil {
		t.Fatalf("ParsePageHTML: %v", err)
	}
	if page.ContentType() != ContentTypeMessage || page.ContentID() != "testchannel_12" {
		t.Fatalf("page content = %s %s", page.ContentType(), page.ContentID())
	}
	if len(page.Messages) != 1 || page.Messages[0].ID != 12 {
		t.Fatalf("messages = %#v", page.Messages)
	}
}

const sampleTelegramHTML = `<!doctype html>
<html>
<head>
  <title>Test Channel - Telegram</title>
  <meta property="og:title" content="Test Channel">
  <meta property="og:description" content="Channel description from meta">
  <meta property="og:image" content="https://cdn.example.com/avatar-meta.jpg">
</head>
<body>
<header>
  <div class="tgme_channel_info_header">
    <i class="tgme_page_photo_image"><img src="//cdn.example.com/avatar.jpg"></i>
    <div class="tgme_channel_info_header_title"><span>Test Channel</span></div>
    <div class="tgme_channel_info_header_labels"><i class="verified-icon">✔</i></div>
    <div class="tgme_channel_info_header_username"><a href="https://t.me/testchannel">@testchannel</a></div>
  </div>
  <div class="tgme_channel_info_counters">
    <div class="tgme_channel_info_counter"><span class="counter_value">12.5K</span> <span class="counter_type">subscribers</span></div>
    <div class="tgme_channel_info_counter"><span class="counter_value">7</span> <span class="counter_type">photos</span></div>
  </div>
  <div class="tgme_channel_info_description">A public Telegram channel</div>
</header>
<main class="tgme_main" data-url="/testchannel">
<section class="tgme_channel_history js-message_history">
  <div class="tgme_widget_message_wrap js-widget_message_wrap">
    <div class="tgme_widget_message js-widget_message" data-post="testchannel/11">
      <div class="tgme_widget_message_user"><a href="https://t.me/testchannel"><i class="tgme_widget_message_user_photo"><img src="https://cdn.example.com/author.jpg"></i></a></div>
      <div class="tgme_widget_message_bubble">
        <div class="tgme_widget_message_author"><a class="tgme_widget_message_owner_name" href="https://t.me/testchannel"><span>Test Channel</span></a></div>
        <a class="tgme_widget_message_photo_wrap" style="background-image:url('https://cdn.example.com/photo.jpg');width:800px;height:600px" href="https://t.me/testchannel/11"></a>
        <div class="tgme_widget_message_text js-message_text" dir="auto"><b>Hello Telegram</b><br/>Second line <a href="/testchannel/10" onclick="return false;">previous</a> <a href="https://example.com/a">external</a></div>
        <div class="tgme_widget_message_footer compact js-message_footer">
          <div class="tgme_widget_message_info short js-message_info">
            <span class="tgme_widget_message_views">1.2K</span><span class="copyonly"> views</span>
            <span class="tgme_widget_message_meta"><a class="tgme_widget_message_date" href="https://t.me/testchannel/11"><time datetime="2026-06-01T10:00:00+00:00" class="time">10:00</time></a></span>
          </div>
        </div>
      </div>
    </div>
  </div>
  <div class="tgme_widget_message_wrap js-widget_message_wrap">
    <div class="tgme_widget_message js-widget_message" data-post="testchannel/12">
      <div class="tgme_widget_message_bubble">
        <div class="tgme_widget_message_author"><a class="tgme_widget_message_owner_name" href="https://t.me/testchannel"><span>Test Channel</span></a></div>
        <a class="tgme_widget_message_video_player js-message_video_player" href="https://t.me/testchannel/12">
          <i class="tgme_widget_message_video_thumb" style="background-image:url(&quot;https://cdn.example.com/thumb.jpg&quot;)"></i>
          <div class="tgme_widget_message_video_wrap" style="width:1920px;padding-top:100%">
            <video src="https://cdn.example.com/video.mp4?token=1" class="tgme_widget_message_video js-message_video"></video>
          </div>
          <time class="message_video_duration js-message_video_duration">0:09</time>
        </a>
        <div class="tgme_widget_message_text js-message_text" dir="auto">Video post</div>
        <a class="tgme_widget_message_link_preview" href="https://example.com/post">
          <div class="link_preview_site_name">Example</div>
          <i class="link_preview_image" style="background-image:url('https://cdn.example.com/preview.jpg')"></i>
          <div class="link_preview_title">Preview title</div>
          <div class="link_preview_description">Preview description</div>
        </a>
        <div class="tgme_widget_message_footer compact js-message_footer">
          <div class="tgme_widget_message_info short js-message_info">
            <span class="tgme_widget_message_views">2M</span>
            <span class="tgme_widget_message_meta"><a class="tgme_widget_message_date" href="https://t.me/testchannel/12"><time datetime="2026-06-02T10:00:00+00:00" class="time">10:00</time></a></span>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>
</main>
</body>
</html>`

func hasLink(links []Link, target string) bool {
	for _, link := range links {
		if link.URL == target {
			return true
		}
	}
	return false
}
