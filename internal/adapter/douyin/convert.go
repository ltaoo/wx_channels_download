package douyinadapter

import (
	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
)

func (h *handler) ToContent(data any) (*model.Content, error) {
	model_data, err := douyin_model_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return douyin_content_from_data(model_data), nil
}

func (h *handler) ToAccount(data any) (*model.Account, error) {
	model_data, err := douyin_model_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return douyin_account_from_data(model_data), nil
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	model_data, err := douyin_model_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content_id := BuildContentID(model_data.video_id)
	return douyin_content_details(
		model_data.content_type,
		content_id,
		douyin_content_detail_from_data(model_data),
	), nil
}

func douyin_content_details(content_type string, content_id string, detail any) []adapter.ContentDetail {
	if detail == nil {
		return nil
	}
	return []adapter.ContentDetail{{Type: content_type, Key: content_id, Data: detail}}
}
