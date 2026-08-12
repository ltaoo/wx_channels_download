package shuba69adapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	shuba69 "wx_channel/pkg/scraper/69shuba"
)

func novel_from_fetch(data any) (*shuba69.Novel, error) {
	switch value := data.(type) {
	case *shuba69.Novel:
		return validate_novel(value)
	case shuba69.Novel:
		return validate_novel(&value)
	case json.RawMessage:
		return novel_from_json(value)
	case []byte:
		return novel_from_json(value)
	case string:
		return novel_from_json([]byte(value))
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode 69shuba fetch data: %w", err)
	}
	return novel_from_json(encoded)
}

func novel_from_json(content_json []byte) (*shuba69.Novel, error) {
	if len(strings.TrimSpace(string(content_json))) == 0 {
		return nil, fmt.Errorf("69shuba content is empty")
	}
	var novel shuba69.Novel
	if err := json.Unmarshal(content_json, &novel); err != nil {
		return nil, fmt.Errorf("decode 69shuba content: %w", err)
	}
	return validate_novel(&novel)
}

func validate_novel(novel *shuba69.Novel) (*shuba69.Novel, error) {
	if novel == nil {
		return nil, fmt.Errorf("69shuba novel is nil")
	}
	if strings.TrimSpace(novel.URL) == "" {
		return nil, fmt.Errorf("69shuba profile url is empty")
	}
	if strings.TrimSpace(novel.Title) == "" {
		return nil, fmt.Errorf("69shuba profile title is empty")
	}
	if _, err := novel_external_id(novel); err != nil {
		return nil, err
	}
	return novel, nil
}

func (a *Shuba69Adapter) ToContent(data any) (*model.Content, error) {
	novel, err := novel_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToContent(novel)
}

func (a *Shuba69Adapter) ToAccount(data any) (*model.Account, error) {
	novel, err := novel_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToAccount(novel)
}

func (a *Shuba69Adapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	novel, err := novel_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToContentDetails(novel)
}
