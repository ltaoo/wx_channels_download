package wxmpadapter

import (
	"encoding/json"
	"fmt"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxmp"
)

func article_data_from_fetch(data any) (*wxmp.ArticleCgiDataNew, error) {
	switch value := data.(type) {
	case *wxmp.ArticleCgiDataNew:
		if value == nil {
			return nil, fmt.Errorf("wxmp article data is nil")
		}
		return value, nil
	case wxmp.ArticleCgiDataNew:
		return &value, nil
	case *wxmp.CgiDataNew:
		return article_data_from_legacy(value)
	case wxmp.CgiDataNew:
		return article_data_from_legacy(&value)
	case *wxmp.WechatOfficialArticle:
		if value == nil || value.PageJSON == nil {
			return nil, fmt.Errorf("wxmp article page data is empty")
		}
		return article_data_from_legacy(value.PageJSON)
	case wxmp.WechatOfficialArticle:
		if value.PageJSON == nil {
			return nil, fmt.Errorf("wxmp article page data is empty")
		}
		return article_data_from_legacy(value.PageJSON)
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode wxmp fetch data: %w", err)
	}
	var article_data wxmp.ArticleCgiDataNew
	if err := json.Unmarshal(encoded, &article_data); err != nil {
		return nil, fmt.Errorf("decode wxmp fetch data: %w", err)
	}
	return &article_data, nil
}

func article_data_from_legacy(data *wxmp.CgiDataNew) (*wxmp.ArticleCgiDataNew, error) {
	if data == nil {
		return nil, fmt.Errorf("wxmp legacy article data is nil")
	}

	article_data := &wxmp.ArticleCgiDataNew{
		UserName:            data.UserName,
		NickName:            data.NickName,
		RoundHeadImg:        data.RoundHeadImg,
		Title:               data.Title,
		Desc:                data.Desc,
		ContentNoencode:     data.ContentNoEncode,
		CreateTime:          data.CreateTime,
		CdnURL:              data.CdnUrl,
		Link:                data.Link,
		SourceURL:           data.SourceUrl,
		CanShare:            data.CanShare,
		Alias:               data.Alias,
		Type:                data.Type,
		Author:              data.Author,
		OriCreateTime:       int(data.OriCreateTime),
		Signature:           data.Signature,
		HdHeadImg:           data.HdHeadImg,
		Bizuin:              data.BizUin,
		Mid:                 int(data.Mid),
		Idx:                 int(data.Idx),
		OriHeadImgURL:       data.OriHeadImgUrl,
		PageType:            data.PageType,
		ItemShowType:        data.ItemShowType,
		ImgFormat:           data.ImgFormat,
		PicturePageInfoList: data.PicturePageInfoList,
	}
	article_data.CopyrightInfo.CopyrightStat = data.CopyrightInfo.CopyrightStat
	return article_data, nil
}

func (a *OfficialAccountAdapter) ToContent(data any) (*model.Content, error) {
	article_data, err := article_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content, _, err := ToContent(article_data)
	return content, err
}

func (a *OfficialAccountAdapter) ToAccount(data any) (*model.Account, error) {
	article_data, err := article_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToAccount(article_data)
}
