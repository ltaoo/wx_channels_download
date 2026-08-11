package wxchannelsadapter

import (
	"encoding/json"
	"fmt"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxchannels"
)

func channels_object_from_fetch(data any) (*wxchannels.ChannelsObject, error) {
	switch value := data.(type) {
	case *wxchannels.ChannelsObject:
		if value == nil {
			return nil, fmt.Errorf("channels object is nil")
		}
		return value, nil
	case wxchannels.ChannelsObject:
		return &value, nil
	case *wxchannels.ChannelsFeedProfileData:
		if value == nil {
			return nil, fmt.Errorf("channels feed profile data is nil")
		}
		return &value.Object, nil
	case wxchannels.ChannelsFeedProfileData:
		return &value.Object, nil
	case *wxchannels.FeedPage:
		if value == nil {
			return nil, fmt.Errorf("channels feed page is nil")
		}
		return &value.Object, nil
	case wxchannels.FeedPage:
		return &value.Object, nil
	}

	var content_json json.RawMessage
	switch value := data.(type) {
	case json.RawMessage:
		content_json = value
	case []byte:
		content_json = value
	case string:
		content_json = json.RawMessage(value)
	default:
		encoded, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("encode channels fetch data: %w", err)
		}
		content_json = encoded
	}

	object, err := parse_channels_object_for_download(content_json)
	if err != nil {
		return nil, err
	}
	return &object, nil
}

func (a *ChannelsAdapter) ToContent(data any) (*model.Content, error) {
	object, err := channels_object_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content, _, err := ToContent(object)
	return content, err
}

func (a *ChannelsAdapter) ToAccount(data any) (*model.Account, error) {
	object, err := channels_object_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToAccount(object)
}

func (a *ChannelsAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	object, err := channels_object_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content, detail, err := ToContent(object)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, nil
	}
	return []adapter.ContentDetail{{Type: content.Type, Key: content.Id, Data: detail}}, nil
}
