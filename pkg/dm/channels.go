package dm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// SearchContact searches the connected channels account contacts.
func (c *Client) SearchContact(ctx context.Context, keyword string, next_marker string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/contact/search", url.Values{
		"keyword":     []string{keyword},
		"next_marker": []string{next_marker},
	})
}

// ContactFeedList lists one contact's feeds.
func (c *Client) ContactFeedList(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/contact/feed/list", url.Values{
		"username":    []string{username},
		"next_marker": []string{next_marker},
	})
}

// LiveReplayList lists one contact's live replays.
func (c *Client) LiveReplayList(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/live/replay/list", url.Values{
		"username":    []string{username},
		"next_marker": []string{next_marker},
	})
}

// FeedProfile reads one feed's profile, addressed by url or oid/nid/eid.
func (c *Client) FeedProfile(ctx context.Context, oid string, nid string, request_url string, eid string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/feed/profile", url.Values{
		"url": []string{request_url},
		"oid": []string{oid},
		"nid": []string{nid},
		"eid": []string{eid},
	})
}

// FeedCommentList lists one feed's comments.
func (c *Client) FeedCommentList(ctx context.Context, oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/feed/comment/list", url.Values{
		"oid":         []string{oid},
		"nid":         []string{nid},
		"comment_id":  []string{comment_id},
		"next_marker": []string{next_marker},
	})
}

// FeedShareURL resolves one feed's share link.
func (c *Client) FeedShareURL(ctx context.Context, oid string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/feed/share_url", url.Values{"oid": []string{oid}})
}

// ChannelStatus returns the video-channel runtime status.
func (c *Client) ChannelStatus(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/status", nil)
}

// LiveProfile reads one live room's profile.
func (c *Client) LiveProfile(ctx context.Context, username string, oid string, nid string, live_id string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/live/profile", url.Values{
		"username": []string{username},
		"oid":      []string{oid},
		"nid":      []string{nid},
		"id":       []string{live_id},
	})
}

// InteractedFeedList lists feeds the account liked or favorited.
func (c *Client) InteractedFeedList(ctx context.Context, flag int, next_marker string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/interactioned/list", url.Values{
		"flag":        []string{strconv.Itoa(flag)},
		"next_marker": []string{next_marker},
	})
}

// FollowedAccounts lists the accounts the connected user follows.
func (c *Client) FollowedAccounts(ctx context.Context, next_marker string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/follow/list", url.Values{"next_marker": []string{next_marker}})
}

// PlayHistory lists the connected user's playback history.
func (c *Client) PlayHistory(ctx context.Context, next_marker string) (json.RawMessage, error) {
	return c.get(ctx, "/api/channels/play/history", url.Values{"next_marker": []string{next_marker}})
}

// DecryptVideo decrypts one downloaded file in place.
func (c *Client) DecryptVideo(ctx context.Context, file_path string, key uint64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/channels/decrypt", url.Values{
		"filepath": []string{file_path},
		"key":      []string{strconv.FormatUint(key, 10)},
	}, nil)
}
