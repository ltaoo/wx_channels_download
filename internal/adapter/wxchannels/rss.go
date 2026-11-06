package wxchannelsadapter

import (
	"encoding/json"
	"fmt"
	"time"

	"wx_channel/pkg/scraper/wxchannels"
)

func build_channels_atom_feed(raw_response json.RawMessage, username, self_url string) (atom_feed, error) {
	// Decode only the fields used by the feed; unrelated upstream fields can vary.
	var response struct {
		ErrCode int    `json:"errCode"`
		ErrMsg  string `json:"errMsg"`
		Data    *struct {
			BaseResponse wxchannels.BaseResponse     `json:"BaseResponse"`
			Contact      wxchannels.ChannelsContact  `json:"contact"`
			Object       []wxchannels.ChannelsObject `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw_response, &response); err != nil {
		return atom_feed{}, fmt.Errorf("parse channels feed response: %w", err)
	}
	if response.ErrCode != 0 {
		return atom_feed{}, fmt.Errorf("fetch channels feed failed (%d): %s", response.ErrCode, response.ErrMsg)
	}
	if response.Data == nil {
		return atom_feed{}, fmt.Errorf("channels feed response is missing data")
	}
	if response.Data.BaseResponse.Ret != 0 {
		return atom_feed{}, fmt.Errorf("fetch channels feed failed (%d): %s", response.Data.BaseResponse.Ret, response.Data.BaseResponse.ErrMsg.String)
	}

	contact := response.Data.Contact
	if contact.Nickname == "" && len(response.Data.Object) > 0 {
		contact = response.Data.Object[0].Contact
	}
	feed := atom_feed{
		Title:     contact.Nickname,
		ID:        "urn:wxchannels:account:" + username,
		Updated:   time.Now().UTC().Format(time.RFC3339),
		Generator: "FindRSS",
		Link: []atom_link{
			{Rel: "self", Href: self_url},
			{Rel: "alternate", Href: "https://channels.weixin.qq.com"},
		},
		Author: atom_author{Name: contact.Nickname},
	}
	if feed.Title == "" {
		feed.Title = "WeChat Channels"
	}
	if feed.Author.Name == "" {
		feed.Author.Name = username
	}
	var latest_time int
	for _, object := range response.Data.Object {
		entry_url := BuildJumpURLFromParts(object.ID, object.ObjectNonceId, object.SourceURL, object.Contact.Username)
		published := time.Unix(int64(object.CreateTime), 0).UTC().Format(time.RFC3339)
		author := object.Contact.Nickname
		if author == "" {
			author = feed.Author.Name
		}
		feed.Entry = append(feed.Entry, atom_entry{
			Title:     ObjectTitle(&object),
			ID:        "urn:wxchannels:feed:" + object.ID,
			Updated:   published,
			Published: published,
			Link:      []atom_link{{Rel: "alternate", Href: entry_url}},
			Content:   atom_content{Type: "text", Body: object.ObjectDesc.Description},
			Author:    atom_author{Name: author},
		})
		if object.CreateTime > latest_time {
			latest_time = object.CreateTime
		}
	}
	if latest_time > 0 {
		feed.Updated = time.Unix(int64(latest_time), 0).UTC().Format(time.RFC3339)
	}
	return feed, nil
}
