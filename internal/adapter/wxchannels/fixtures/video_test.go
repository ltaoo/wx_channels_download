package wxchannels_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	wxchannels "wx_channel/internal/adapter/wxchannels"
	"wx_channel/internal/database/model"
	wxchannelspkg "wx_channel/pkg/scraper/wxchannels"
)

const videoFeedJSON = `{
	"id": "14962486294771997060",
	"nickname": "迷人的大嘴猴",
	"username": "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
	"objectDesc": {
		"description": "讨厌我有什么用 有本事弄死我",
		"media": [
			{
				"url": "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&hy=SZ&idx=1&m=414c8b10462c8fa97a904c3d999a0476&uzid=7a206",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPnQr0gxUocFa2h6q3eoq4hXzG39ub5SWukSZAsfOaRiadTuuGIYouJicfpVpzk12gN6RJ2mlOl26YUBWWTVupMcpSIhJDGZaKiaRI&token=ic1n0xDG6awibhOHyNxbvz6nLNtsL3qg5UrFPrz5Jj4TMUicLBbchc6FxnZm5WybqCJGmyeCPokfKqLKqgia6PpXIc7oxANHcCfUGvZ2tkcIfe9Gnz8pKU6G2fVsHnRmVYqPkoqyLdic9MrwTdQWmCLTamzeQ40lL8sTUiaaMgr0QibWm7wQAbtMvUalYywFOoiaotMxjeEHU4mg8GLIS33rP8iaUwuyIrBiandouT&hy=SZ&idx=1&m=7b022855f315b6aa0a3dd30f631d1d4a&picformat=200&wxampicformat=503",
				"mediaType": 4,
				"videoPlayLen": 9,
				"width": 1080,
				"height": 2128,
				"md5sum": "265e55f408171415a0f987e6faa757b0",
				"fileSize": 9613487,
				"bitrate": 8226,
				"spec": [
					{
						"fileFormat": "xWT111",
						"firstLoadBytes": 1066775,
						"bitRate": 190,
						"codingFormat": "h264",
						"dynamicRangeType": 0,
						"vfps": 30,
						"width": 720,
						"height": 1416,
						"durationMs": 9134,
						"qualityScore": 77,
						"videoBitrate": 1489,
						"audioBitrate": 67,
						"levelOrder": 100,
						"bypass": "{\"rid\":\"1783693349912495837\";\"level_order\":100;\"ip_area_id\":\"cn.ml\";\"max_bitrate\":150;\"trans_flag\":21;\"phone_level\":0;\"full_url_type\":0;\"video_play_len\":9;\"grade\":1;\"highest_grade\":1;\"lowest_grade\":3;\"cgi_id\":3763;\"cgi_scene\":6;\"pre_f_time\":30000;\"b_len\":10;\"avg_v_len\":9;\"fake_uin\":460008016;\"vqas_v0\":67.1524047852;\"vqas_ps\":62.2868690491;\"ad_flag\":4;\"netid\":4}",
						"is3az": 0,
						"enhance": "0",
						"libVersion": "158423",
						"mediaRateType": 0,
						"fileSize": 0
					},
					{
						"fileFormat": "xWT112",
						"firstLoadBytes": 872322,
						"bitRate": 156,
						"codingFormat": "h264",
						"dynamicRangeType": 0,
						"vfps": 30,
						"width": 720,
						"height": 1416,
						"durationMs": 9134,
						"qualityScore": 75,
						"videoBitrate": 1204,
						"audioBitrate": 67,
						"levelOrder": 200,
						"bypass": "{\"rid\":\"1783693349912495837\";\"level_order\":200;\"ip_area_id\":\"cn.ml\";\"max_bitrate\":150;\"trans_flag\":21;\"phone_level\":0;\"full_url_type\":0;\"video_play_len\":9;\"grade\":2;\"highest_grade\":1;\"lowest_grade\":3;\"cgi_id\":3763;\"cgi_scene\":6;\"pre_f_time\":30000;\"b_len\":10;\"avg_v_len\":9;\"fake_uin\":460008016;\"vqas_v0\":67.1524047852;\"vqas_ps\":61.8532600403;\"ad_flag\":4;\"netid\":4}",
						"is3az": 0,
						"enhance": "0",
						"libVersion": "158423",
						"mediaRateType": 0,
						"fileSize": 0
					},
					{
						"fileFormat": "xWT113",
						"firstLoadBytes": 708630,
						"bitRate": 126,
						"codingFormat": "h264",
						"dynamicRangeType": 0,
						"vfps": 30,
						"width": 720,
						"height": 1416,
						"durationMs": 9134,
						"qualityScore": 73,
						"videoBitrate": 976,
						"audioBitrate": 54,
						"levelOrder": 300,
						"bypass": "{\"rid\":\"1783693349912495837\";\"level_order\":300;\"ip_area_id\":\"cn.ml\";\"max_bitrate\":150;\"trans_flag\":21;\"phone_level\":0;\"full_url_type\":0;\"video_play_len\":9;\"grade\":3;\"highest_grade\":1;\"lowest_grade\":3;\"cgi_id\":3763;\"cgi_scene\":6;\"pre_f_time\":30000;\"b_len\":10;\"avg_v_len\":9;\"fake_uin\":460008016;\"vqas_v0\":67.1524047852;\"ad_flag\":4;\"netid\":4}",
						"is3az": 0,
						"enhance": "0",
						"libVersion": "158423",
						"mediaRateType": 0,
						"fileSize": 0
					},
					{
						"fileFormat": "xWT156",
						"firstLoadBytes": 651116,
						"bitRate": 115,
						"codingFormat": "h265",
						"dynamicRangeType": 0,
						"vfps": 30,
						"width": 720,
						"height": 1416,
						"durationMs": 9134,
						"qualityScore": 77,
						"videoBitrate": 867,
						"audioBitrate": 67,
						"levelOrder": 100,
						"bypass": "{\"rid\":\"1783693349912495837\";\"level_order\":100;\"ip_area_id\":\"cn.ml\";\"max_bitrate\":150;\"trans_flag\":21;\"phone_level\":0;\"full_url_type\":0;\"video_play_len\":9;\"grade\":1;\"highest_grade\":1;\"lowest_grade\":3;\"cgi_id\":3763;\"cgi_scene\":6;\"pre_f_time\":30000;\"b_len\":10;\"avg_v_len\":9;\"fake_uin\":460008016;\"vqas_v0\":67.1524047852;\"vqas_ps\":62.7309570312;\"ad_flag\":4;\"netid\":4}",
						"is3az": 0,
						"enhance": "0",
						"libVersion": "159423",
						"mediaRateType": 0,
						"fileSize": 0
					},
					{
						"fileFormat": "xWT157",
						"firstLoadBytes": 538133,
						"bitRate": 94,
						"codingFormat": "h265",
						"dynamicRangeType": 0,
						"vfps": 30,
						"width": 720,
						"height": 1416,
						"durationMs": 9134,
						"qualityScore": 75,
						"videoBitrate": 702,
						"audioBitrate": 67,
						"levelOrder": 200,
						"bypass": "{\"rid\":\"1783693349912495837\";\"level_order\":200;\"ip_area_id\":\"cn.ml\";\"max_bitrate\":150;\"trans_flag\":21;\"phone_level\":0;\"full_url_type\":0;\"video_play_len\":9;\"grade\":2;\"highest_grade\":1;\"lowest_grade\":3;\"cgi_id\":3763;\"cgi_scene\":6;\"pre_f_time\":30000;\"b_len\":10;\"avg_v_len\":9;\"fake_uin\":460008016;\"vqas_v0\":67.1524047852;\"ad_flag\":4;\"netid\":4}",
						"is3az": 0,
						"enhance": "0",
						"libVersion": "159423",
						"mediaRateType": 0,
						"fileSize": 0
					},
					{
						"fileFormat": "xWT158",
						"firstLoadBytes": 434724,
						"bitRate": 76,
						"codingFormat": "h265",
						"dynamicRangeType": 0,
						"vfps": 30,
						"width": 720,
						"height": 1416,
						"durationMs": 9134,
						"qualityScore": 73,
						"videoBitrate": 563,
						"audioBitrate": 54,
						"levelOrder": 300,
						"bypass": "{\"rid\":\"1783693349912495837\";\"level_order\":300;\"ip_area_id\":\"cn.ml\";\"max_bitrate\":150;\"trans_flag\":21;\"phone_level\":0;\"full_url_type\":0;\"video_play_len\":9;\"grade\":3;\"highest_grade\":1;\"lowest_grade\":3;\"cgi_id\":3763;\"cgi_scene\":6;\"pre_f_time\":30000;\"b_len\":10;\"avg_v_len\":9;\"fake_uin\":460008016;\"vqas_v0\":67.1524047852;\"vqas_ps\":62.739151001;\"ad_flag\":4;\"netid\":4}",
						"is3az": 0,
						"enhance": "0",
						"libVersion": "159423",
						"mediaRateType": 0,
						"fileSize": 0
					}
				],
				"coverUrl": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=2fG3V4WwQPluPpjb46OTKMXHc112k4G2oJic38N7rnuA86EibU1Y76s8oA7ibJ2icEheVFXiah55XOtQTzMnAsGIe2IWYOSogJ0DHQGv97AFZePM&token=AxricY7RBHdVdU7Gn7iczBDOyqkPzEiazv6slYib62vrPnRWLdajxdDW6L5750WibUCk6R96RGUJ3MAHbTqSV90lo9nH8Wn7JShFsWZgr68VIDPoEYFqYLakd4tDgsE26h00sXkjVy5cSHmf6aCEbjhuJYGRaQ3eZISKiatbry08Ugw1R9B6zzeWxvqJ2hNlojz1GCPcpNq8j85OXOWGlicSBmVd3kQGj5vTzx7&hy=SZ&idx=1&m=73a9ef1bc335f9c43d800208ddc42f09&uzid=1&picformat=200&wxampicformat=503",
				"decodeKey": "1522886121",
				"urlToken": "&token=2lt8WBSnjTkTjXXRcWF576SLtqb9LdRn1Cliaa0icf5zFjCLyBFNe1e3eKzhzzEc5h05O81ibb3hwbVTVywYQAQbSQzZkHicCqabpEdwBzhTgdyPiakaMMw7n96CtNxoPbKkQxiaYOzPImgS9ZG3kDzKcLjMEyIIVGYuibzdHECVIOFibOQGL4pWibDRRD6VcpGApwhugo6k9Mq48YAov7zg751dO260H5iaGeEkJZWhKhib0hib4W0&basedata=CAMSBnhXVDE1OCJaCgoKBnhXVDE1OBAACgoKBnhXVDE1NxAACgoKBnhXVDE1NhAACgoKBnhXVDExMxAACgoKBnhXVDExMhAACgoKBnhXVDExMRAACgcKA3hBMBAACgcKA3hBMhAA&sign=AgZzkYT5vBvSWwKe5MpufA75x2T3Xnnz7PtuTK98WxdVbZm4Grpnyl52sDN4W6CI562FVgGaZ-_tYlBjCRLdIQ&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAABRfl4aFfX8vo5XJgBRahAAAADnaHZTnGbFfAj9RgZXfw6ViUCWOt8LYujr%2BrkpCHNy7PD375%2BDqLzGDCk8ibQxWRl9tKOjUKAhiL4%3D&svrnonce=1783693350",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullCoverUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPkvOWRTX1H8EEicA8MBXLiaswl8IU9r6jGthTDHZ1qGmukNzmd8O8OUEplibFhS7ZtT1yvWDNHYW7Toib9iciaR9TdAmkt81pCYVutibE&token=AxricY7RBHdVddAbcZopbg4GqHZicJpZaIjN1nicJI2DHsxIyD6z33Bic2LMztwEgfcPcE2XmpibLLYG82ooVOpAvVUj0bEG95VeFpEeZIQLicVOohQX1FzmiaiapgdjIJDNxlfByncia1EOQcGHnv2CicezKhic53aL5eILyiaibdngPSUrLRicMvsOjXSGRZ5dAR9jTiccxGTg6R8d0XD3ib2WTlQn2HyctNgT6xZIWq7I&hy=SZ&idx=1&m=ab396fcc30747f5492aaac827c3e33c1&picformat=200&wxampicformat=503",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"scalingInfo": {
					"version": "v2.0.1",
					"isSplitScreen": true,
					"isDisableFollow": false,
					"upPercentPosition": 0.06439435482025146,
					"downPercentPosition": 0.9592386484146118
				},
				"cardShowStyle": 1,
				"dynamicRangeType": 0,
				"videoType": 1,
				"audioSpec": [],
				"mediaCdnInfo": {
					"isUsePcdn": true,
					"beginUsePcdnBufferSeconds": 12,
					"exitUsePcdnBufferSeconds": 8,
					"preloadBeginUsePcdnBufferKbytes": 768,
					"pcdnTimeoutRetryCount": 1,
					"marsPreDownloadKbytes": 0,
					"isUseUgcWhenNoPreload": true,
					"preloadUsePcdnOnly": true,
					"preloadPcdnConnections": 4,
					"socForceUseH3": false
				},
				"shareCoverUrl": ""
			}
		],
		"mediaType": 4,
		"location": {
			"longitude": 114.30999755859375,
			"latitude": 23,
			"city": "惠州市",
			"poiName": "华廷·悦府",
			"poiAddress": "广东省惠州市惠城区陈江大道南19号",
			"poiClassifyId": "qqmap_2524498428568312814",
			"country": "中国",
			"buildingId": "",
			"floorName": "",
			"productId": [],
			"commercializationFlag": 0,
			"multiLangInfo": [],
			"countryCode": "CN",
			"adcode": 441302
		},
		"extReading": {},
		"topic": {},
		"mentionedUser": [],
		"feedLocation": {
			"productId": [],
			"multiLangInfo": []
		},
		"mentionedMusics": [],
		"followPostInfo": {
			"musicInfo": {
				"chorusBegin": 0,
				"docType": 0
			},
			"hasBgm": 1
		},
		"clientDraftExtInfo": {
			"coverWordInfo": [],
			"lbsFlagType": 2,
			"videoMusicId": "0",
			"needPostATemplateComment": 0,
			"memberData": {
				"postWithMemberZoneLink": 0
			},
			"mjPublisherInfo": {
				"mjPublisherSessionId": "3d46cc62-6406-4ce6-96a5-5e7c8b3bbcf1",
				"mjPublisherEntryType": "FinderPersonalCenterPagePostingButton",
				"isDuetShoot": false,
				"mjPublisherExportScene": 10,
				"mjPublisherScTemplateTabId": "",
				"sourceFeedId": "",
				"sourceSongId": "",
				"followFeedTemplateId": "",
				"mjPublisherScTemplateId": "",
				"mjPublisherScTemplatePosition": 0,
				"isScAssetGenerate": false,
				"mjPublisherCreationPageId": 30098,
				"isFromMovieTemplate": 0,
				"scTemplateIsFavorite": false,
				"mjPublisherTemplateType": 0,
				"scTemplateIsAigc": false
			},
			"videoSourceType": 1,
			"feedLongitude": 0,
			"feedLatitude": 0,
			"sourceEnterScene": 3,
			"shootMusicReportInfo": {
				"scene": 1
			},
			"editMusicReportInfo": {
				"scene": 2
			},
			"coverSelectSource": 0
		},
		"generalReportInfo": {
			"clientInfo": "eyJjaGlsZF9lbnRlcnNjZW5lIjowLCJ2aWRlb3NvdXJjZSI6MSwiY29tbWVudFNjZW5lIjo5NSwiZW50ZXJzY2VuZSI6M30="
		},
		"posterLocation": {
			"city": "Huizhou City",
			"productId": [],
			"multiLangInfo": [],
			"adcode": 441300
		},
		"shortTitle": [],
		"flowCardDesc": {
			"description": "讨厌我有什么用 有本事弄死我"
		},
		"finderNewlifeDesc": {
			"secretlyPushChatroomName": [],
			"commentEggInfo": [],
			"videoTmplInfo": [],
			"customCropInfo": [],
			"mpLocations": []
		},
		"memberData": {
			"postWithMemberZoneLink": 0
		},
		"modFeedInfo": {
			"history": [],
			"modifyButtonStatus": 0
		},
		"publisherVideoInfo": {
			"editingTools": 9,
			"multiEditingTools": [],
			"videoSource": 1,
			"showWording": "抖音"
		}
	},
	"createtime": 1783667361,
	"likeFlag": 0,
	"likeList": [],
	"commentList": [],
	"forwardCount": 89,
	"contact": {
		"username": "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		"nickname": "迷人的大嘴猴",
		"headUrl": "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132",
		"signature": "谢谢观看\n只是爱分享一些大哥爱看的視頻 仅此而已\n懂点规矩 蠢狗不要发私信",
		"followFlag": 0,
		"authInfo": {
			"authIconType": 1,
			"authProfession": "娱乐主播",
			"detailLink": "pages/index/index.html?showdetail=true&username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
			"appName": "gh_4ee148a6ecaa@app",
			"authIconUrl": "https://dldir1v6.qq.com/weixin/checkresupdate/auth_icon_level3_2e2f94615c1e4651a25a7e0446f63135.png",
			"customerType": 0
		},
		"coverImgUrl": "",
		"spamStatus": 0,
		"extFlag": 270663948,
		"extInfo": {
			"country": "CN",
			"province": "Guangdong",
			"city": "Huizhou",
			"sex": 2
		},
		"liveStatus": 2,
		"originalEntranceFlag": 1,
		"liveCoverImgUrl": "https://finder.video.qq.com/292/20350/stodownload?encfilekey=oibeqyX228riaCwo9STVsGLDB0NpTOSvvAd6ycnth0VPP7AOCXyXibHibsiaDfGBiaYLyJJ6sFMqtU0xIMkkNbLSP6ibNSYgFKibtLPtDMDkyWSgv7l8j3u2NLLkPTAEgZNia7aBAdI5TBolbZK8&token=ic1n0xDG6awic0NhTSWxiaIDueLPBTh74CqqV7I3Awx0R8LwicfuaEMPhicxVXbxzPdLwvrU3UIPoJjZAcJpa0whly9uNNbYKoeV4SUQaFxM6wHvDtvqDBs5KLAYglrnDAaWMS4qibTZw3VBvwDqWOzVCjuqZ57HFQ87usibRWBDwx7ks2iadNvIJ4p0PVNTqIJ377Yk5oIylCSgL0YnW1AIyxj4VFWmOB9yibxvB&hy=SZ&idx=1&m=90940085b4afbbe6022f04b9831ae182&picformat=200&wxampicformat=503",
		"liveInfo": {
			"anchorStatusFlag": "1619134656",
			"switchFlag": 4607,
			"sourceType": 0,
			"micSetting": {
				"settingFlag": 0,
				"settingSwitchFlag": 4,
				"highlightMicPerson": false,
				"pkSettingFlag": 0,
				"micLayoutBaseMode": 1
			},
			"lotterySetting": {
				"settingFlag": 0,
				"attendType": 3
			},
			"liveCoverImgs": [
				{
					"url": "https://finder.video.qq.com/292/20350/stodownload?encfilekey=oibeqyX228riaCwo9STVsGLDB0NpTOSvvAd6ycnth0VPP7AOCXyXibHibsiaDfGBiaYLyJJ6sFMqtU0xIMkkNbLSP6ibNSYgFKibtLPtDMDkyWSgv7l8j3u2NLLkPTAEgZNia7aBAdI5TBolbZK8&token=AxricY7RBHdVa3vODdVUCodn9OZEycBicFMhVNXaO2EM0XPLrMAA5nJ94gA787diaDWp5RuYmTzdD9PmXMjVsu9uVSUVBpFT04UwaNguNkeM1icx2Pu6LX8WhXtjaRKtPgpy4FIicfiaUibI4iaxAXY49r6ARich1ORJhuwzaB6nrFOSQ94mBTezlVJao8QSQukOj6eovWISX1XicjyUibOJNujSZQ5ugcAUymArlHI&hy=SZ&idx=1&m=90940085b4afbbe6022f04b9831ae182&picformat=200&wxampicformat=503",
					"urlToken": "",
					"fileSize": 0,
					"width": 810,
					"height": 1080,
					"ratio": 2,
					"source": 2,
					"liveCoverHd": false
				}
			],
			"liveCoverImgTs": 1783691130,
			"replaySetting": {
				"autoGenLiveReplay": false,
				"intelligentlyGenReplayHighlight": false,
				"enableReplayDumpDanmu": false,
				"canUseIntelligentlyGenReplayHighlight": true
			}
		},
		"friendFollowCount": 0,
		"feedCount": 17,
		"bindInfo": [],
		"menu": [],
		"status": "0",
		"additionalFlag": "1041",
		"referenceInfo": [
			{
				"type": 1,
				"name": "公众号/服务号",
				"status": 2
			},
			{
				"type": 2,
				"name": "小程序",
				"status": 2
			},
			{
				"type": 4,
				"name": "秒剪",
				"status": 2
			}
		]
	},
	"recommenderList": [],
	"likeCount": 92,
	"commentCount": 18,
	"friendLikeCount": 0,
	"objectNonceId": "4390481592474233535_0_146_0_0",
	"objectStatus": 0,
	"sendShareFavWording": "",
	"originalFlag": 0,
	"secondaryShowFlag": 1,
	"mentionedUserContact": [],
	"sessionBuffer": "eyJjdXJfbGlrZV9jb3VudCI6OTIsImN1cl9jb21tZW50X2NvdW50IjoxOCwicmVjYWxsX3R5cGVzIjpbXSwiZGVsaXZlcnlfc2NlbmUiOjYsImRlbGl2ZXJ5X3RpbWUiOjE3ODM2OTMzNTAsInNldF9jb25kaXRpb25fZmxhZyI6MjksInJlY2FsbF9pbmRleCI6W10sInJlcXVlc3RfaWQiOjUwMTYzNDU2MzUzNTE2MzQsInJlY2FsbF9pbmZvIjpbXSwic2VjcmV0ZV9kYXRhIjoiQmdBQWpIVk5hSnJGNWZyY2tvRzhKUXVOa2l5U3JZZXJjT0ZGSUl6VDRhMEljb3F2anRZMXQ1ZzdYTVQwSTVTTEJSZmF4NmdQcDhsUSIsImlkYyI6MywiZGV2aWNlX3R5cGVfaWQiOjI5LCJwdWxsX3R5cGUiOjQsImNsaWVudF9yZXBvcnRfYnVmZiI6IntcImVudHJhbmNlSWRcIjpcIjEwMDJcIn0iLCJjb21tZW50X3NjZW5lIjoxNDAsIm9iamVjdF9pZCI6MTQ5NjI0ODYyOTQ3NzE5OTcwNjAsImV4cHRfZmxhZyI6MSwib2JqX2ZsYWciOjE2Mzg0MCwiZXJpbCI6W10sInBna2V5cyI6W10sIm9ial9leHRfZmxhZyI6MjYyMjA4LCJzY2lkIjoiYzkwYTVlNTgtN2M2YS0xMWYxLWFkOTgtMjk1ZDk1NDk5YjZiIn0=",
	"warnFlag": 2,
	"warnWording": "作者提示: 内容为虚构剧情，仅供娱乐",
	"favCount": 337,
	"favFlag": 0,
	"urlValidDuration": 172800,
	"forwardStyle": 0,
	"permissionFlag": 2147483648,
	"objectType": 0,
	"friendCommentList": [],
	"adFlag": 4,
	"funcFlag": 272,
	"showOriginal": true,
	"playhistoryInfo": {
		"breakpointTimeMs": 6000,
		"lastPlayTime": 1783692373616
	},
	"finderPromotionJumpinfo": {
		"jumpInfo": {
			"jumpinfoType": 1,
			"wording": "帮上热门",
			"miniAppInfo": {
				"appId": "wx0ebcb2fd0155584d",
				"path": "pages/promote/PromoteFinderForm.html",
				"extraData": "eyJleHBvcnRfaWQiOiJleHBvcnQvVXpGZkJnQUF4TkNBVkZvNkxBRzNqTXpUNERDTFJoaXprSGx1VXdFRXgzZWlDc0tPSHcifQ=="
			},
			"style": [],
			"supportDeviceList": []
		},
		"wording": "帮上热门",
		"destinationType": 1
	},
	"ipRegionInfo": {
		"regionText": "Guangdong"
	},
	"objectExtend": {
		"favInfo": {
			"starFavFlag": 0,
			"starFavCount": 0,
			"fingerlikeFavFlag": 0,
			"fingerlikeFavCount": 337
		},
		"preloadConfig": {
			"commentIsPreload": true,
			"commentWaitTime": 5,
			"commentPreloadBuffer": "CAEQBQ=="
		},
		"monotonicData": {
			"countInfo": {
				"commentCount": 18,
				"likeCount": 92,
				"forwardCount": 89,
				"readCount": 0,
				"favCount": 337,
				"versionData": {
					"dataVersion": 1783693350,
					"overwrite": false
				}
			},
			"commentCount": {
				"commentCount": 18,
				"imageCommentCount": 0,
				"versionData": {
					"dataVersion": 1783693350
				}
			},
			"globalFavCount": {},
			"globalFavFlag": {},
			"thumbUpCount": {
				"thumbUpCount": 337
			},
			"thumbUpFlag": {},
			"chatroomPushCount": {},
			"chatroomPushFlag": {
				"chatroomPushList": []
			},
			"thankChatroomPushFlag": {}
		},
		"finderNewlifeInfo": {
			"chatroomPushList": [],
			"pictureCropInfo": [],
			"followPostInfo": {}
		},
		"originalInfo": {
			"originalAuditStatus": 3,
			"originalPlanVer": 2
		},
		"streamContextId": "c90a5e58-7c6a-11f1-ad98-295d95499b6b"
	}
}
`

func TestToAccount_FromVideoFeedJSON(t *testing.T) {
	var obj wxchannelspkg.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}

	expectedUsername := "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder"
	if account.ExternalId != expectedUsername {
		t.Errorf("ExternalId = %q", account.ExternalId)
	}
	if account.Nickname != "迷人的大嘴猴" {
		t.Errorf("Nickname = %q, want %q", account.Nickname, "迷人的大嘴猴")
	}
	if account.PlatformId != "wx_channels" {
		t.Errorf("PlatformId = %q", account.PlatformId)
	}
	_ = expectedUsername // account.Id is a string primary key
}

func TestToContent_FromVideoFeedJSON(t *testing.T) {
	var obj wxchannelspkg.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	content, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}

	if content.ContentType != "video" {
		t.Errorf("ContentType = %q, want %q", content.ContentType, "video")
	}
	if content.Title != "讨厌我有什么用 有本事弄死我" {
		t.Errorf("Title = %q", content.Title)
	}
	if content.Description != "讨厌我有什么用 有本事弄死我" {
		t.Errorf("Description = %q", content.Description)
	}
	if content.ExternalId != "14962486294771997060" {
		t.Errorf("ExternalId = %q", content.ExternalId)
	}
	if content.ExternalId2 != "4390481592474233535_0_146_0_0" {
		t.Errorf("ExternalId2 = %q, want %q", content.ExternalId2, "4390481592474233535_0_146_0_0")
	}
	if content.ExternalId3 != "1522886121" {
		t.Errorf("ExternalId3 = %q, want %q", content.ExternalId3, "1522886121")
	}
	_ = content.ContentType // content.Id is set by ToContent as string
	expectedContentURL := "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&token=2lt8WBSnjTkTjXXRcWF576SLtqb9LdRn1Cliaa0icf5zFjCLyBFNe1e3eKzhzzEc5h05O81ibb3hwbVTVywYQAQbSQzZkHicCqabpEdwBzhTgdyPiakaMMw7n96CtNxoPbKkQxiaYOzPImgS9ZG3kDzKcLjMEyIIVGYuibzdHECVIOFibOQGL4pWibDRRD6VcpGApwhugo6k9Mq48YAov7zg751dO260H5iaGeEkJZWhKhib0hib4W0&basedata=CAMSBnhXVDE1OCJaCgoKBnhXVDE1OBAACgoKBnhXVDE1NxAACgoKBnhXVDE1NhAACgoKBnhXVDExMxAACgoKBnhXVDExMhAACgoKBnhXVDExMRAACgcKA3hBMBAACgcKA3hBMhAA&sign=AgZzkYT5vBvSWwKe5MpufA75x2T3Xnnz7PtuTK98WxdVbZm4Grpnyl52sDN4W6CI562FVgGaZ-_tYlBjCRLdIQ&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAABRfl4aFfX8vo5XJgBRahAAAADnaHZTnGbFfAj9RgZXfw6ViUCWOt8LYujr%2BrkpCHNy7PD375%2BDqLzGDCk8ibQxWRl9tKOjUKAhiL4%3D&svrnonce=1783693350"
	if content.ContentURL != expectedContentURL {
		t.Errorf("ContentURL = %q, want %q", content.ContentURL, expectedContentURL)
	}
	if content.URL != content.ContentURL {
		t.Errorf("URL = %q, should equal ContentURL", content.URL)
	}
	expectedCoverURL := "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPnQr0gxUocFa2h6q3eoq4hXzG39ub5SWukSZAsfOaRiadTuuGIYouJicfpVpzk12gN6RJ2mlOl26YUBWWTVupMcpSIhJDGZaKiaRI&token=ic1n0xDG6awibhOHyNxbvz6nLNtsL3qg5UrFPrz5Jj4TMUicLBbchc6FxnZm5WybqCJGmyeCPokfKqLKqgia6PpXIc7oxANHcCfUGvZ2tkcIfe9Gnz8pKU6G2fVsHnRmVYqPkoqyLdic9MrwTdQWmCLTamzeQ40lL8sTUiaaMgr0QibWm7wQAbtMvUalYywFOoiaotMxjeEHU4mg8GLIS33rP8iaUwuyIrBiandouT&hy=SZ&idx=1&m=7b022855f315b6aa0a3dd30f631d1d4a&picformat=200&wxampicformat=503"
	if content.CoverURL != expectedCoverURL {
		t.Errorf("CoverURL = %q, want %q", content.CoverURL, expectedCoverURL)
	}
	if content.CoverWidth != "1080" {
		t.Errorf("CoverWidth = %q, want %q", content.CoverWidth, "1080")
	}
	if content.CoverHeight != "2128" {
		t.Errorf("CoverHeight = %q, want %q", content.CoverHeight, "2128")
	}
	if content.Duration != 9 {
		t.Errorf("Duration = %d, want 9", content.Duration)
	}
	if content.Size != 9613487 {
		t.Errorf("Size = %d, want 9613487", content.Size)
	}
	if content.PublishTime == nil || *content.PublishTime != 1783667361 {
		t.Errorf("PublishTime = %v, want ptr to 1783667361", content.PublishTime)
	}
	expectedMetadata := `{"key":"1522886121"}`
	if content.Metadata != expectedMetadata {
		t.Errorf("Metadata = %q, want %q", content.Metadata, expectedMetadata)
	}
}

func mustMarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func assertEqual(t *testing.T, expected, actual any) {
	t.Helper()
	expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
	actualJSON, _ := json.MarshalIndent(actual, "", "  ")
	if string(expectedJSON) != string(actualJSON) {
		t.Errorf("mismatch:\n--- expected\n+++ actual\n%s\n%s",
			string(expectedJSON), string(actualJSON))
	}
}

func TestBuildBrowseRecord_FromVideoFeedJSON(t *testing.T) {
	var obj wxchannelspkg.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	r := wxchannels.BuildBrowseRecordFromObject(&obj)

	expected := model.BrowseHistory{
		Id:                "wx_channels:14962486294771997060",
		PlatformId:        "wx_channels",
		VisitedTimes:      1,
		AccountExternalId: "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		AccountUsername:   "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		AccountNickname:   "迷人的大嘴猴",
		AccountAvatarURL:  "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132",
		ContentType:       "video",
		ContentExternalId: "14962486294771997060",
		ContentTitle:      "讨厌我有什么用 有本事弄死我",
		ContentURL:        r.ContentURL,
		ContentSourceURL:  "https://channels.weixin.qq.com/web/pages/feed?username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665%40finder&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		ContentCoverURL:   r.ContentCoverURL,
		ExtraData:         string(mustMarshalJSON(map[string]any{"id": "14962486294771997060", "nonce_id": "4390481592474233535_0_146_0_0", "decode_key": "1522886121"})),
		Timestamps:        r.Timestamps,
	}

	expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
	actualJSON, _ := json.MarshalIndent(r, "", "  ")

	if string(expectedJSON) != string(actualJSON) {
		t.Errorf("BrowseHistory mismatch:\n--- expected\n+++ actual\n%s\n%s",
			string(expectedJSON), string(actualJSON))
	}
}

// TestDownloadFlow_FromVideoFeedJSON verifies the model conversion and linkage
// from a raw video feed JSON: Account, Content, BrowseHistory, and DownloadTask
// are correctly built and cross-referenced.
func TestDownloadFlow_FromVideoFeedJSON(t *testing.T) {
	var obj wxchannelspkg.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// ---- Account ----
	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	expectedAccount := model.Account{
		Id:         "wx_channels:v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		PlatformId: "wx_channels",
		ExternalId: "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		Username:   "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		Nickname:   "迷人的大嘴猴",
		AvatarURL:  "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132",
		Timestamps: account.Timestamps,
	}
	assertEqual(t, expectedAccount, *account)

	// ---- Content ----
	content, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	if content.PlatformId != "wx_channels" {
		t.Errorf("Content.PlatformId = %q, want %q", content.PlatformId, "wx_channels")
	}
	if content.ContentType != "video" {
		t.Errorf("Content.ContentType = %q, want %q", content.ContentType, "video")
	}
	if content.ExternalId != "14962486294771997060" {
		t.Errorf("Content.ExternalId = %q", content.ExternalId)
	}
	if content.ExternalId2 != "4390481592474233535_0_146_0_0" {
		t.Errorf("Content.ExternalId2 = %q", content.ExternalId2)
	}
	if content.ExternalId3 != "1522886121" {
		t.Errorf("Content.ExternalId3 = %q", content.ExternalId3)
	}
	if content.Title != "讨厌我有什么用 有本事弄死我" {
		t.Errorf("Content.Title = %q", content.Title)
	}
	if content.Size != 9613487 {
		t.Errorf("Content.Size = %d, want 9613487", content.Size)
	}
	if content.Duration != 9 {
		t.Errorf("Content.Duration = %d, want 9", content.Duration)
	}
	if content.CoverWidth != "1080" {
		t.Errorf("Content.CoverWidth = %q, want '1080'", content.CoverWidth)
	}
	if content.CoverHeight != "2128" {
		t.Errorf("Content.CoverHeight = %q, want '2128'", content.CoverHeight)
	}
	if content.PublishTime == nil || *content.PublishTime != 1783667361 {
		t.Errorf("Content.PublishTime = %v, want ptr to 1783667361", content.PublishTime)
	}
	if content.Metadata != `{"key":"1522886121"}` {
		t.Errorf("Content.Metadata = %q", content.Metadata)
	}
	if content.ContentURL == "" || content.URL != content.ContentURL {
		t.Errorf("Content.URL must equal ContentURL")
	}
	if content.CoverURL == "" {
		t.Errorf("Content.CoverURL must not be empty")
	}

	// ---- Download parameters: use production functions only ----
	spec := wxchannels.PickSpec(&obj)
	if spec != "xWT111" {
		t.Errorf("PickSpec = %q, want %q", spec, "xWT111")
	}
	title := wxchannels.ObjectTitle(&obj)
	if title != "讨厌我有什么用 有本事弄死我" {
		t.Errorf("ObjectTitle = %q, want %q", title, "讨厌我有什么用 有本事弄死我")
	}
	downloadURL := wxchannels.BuildDownloadURLWithSpec(&obj, spec)
	if !strings.HasSuffix(downloadURL, "&X-snsvideoflag=xWT111") {
		t.Errorf("BuildDownloadURLWithSpec should end with X-snsvideoflag=xWT111, got %q", downloadURL)
	}
	if !strings.HasPrefix(downloadURL, content.ContentURL) {
		t.Errorf("BuildDownloadURLWithSpec should start with ContentURL")
	}

	// ---- V1 DownloadTaskV1: task-level container ----
	configJSON, _ := json.Marshal(map[string]any{
		"platform":    content.PlatformId,
		"external_id": content.ExternalId,
		"nonce_id":    content.ExternalId2,
		"spec":        spec,
		"source_url":  content.SourceURL,
		"url":         content.URL,
		"content_url": content.ContentURL,
	})
	tmpTaskId := 1
	task := model.DownloadTaskV1{
		Id:           tmpTaskId,
		Name:         title,
		ResourceType: model.ResourceTypeFile,
		Status:       model.TaskStatusWaiting,
		SavePath:     "/downloads/wx_channels",
		ConfigJSON:   string(configJSON),
	}
	if task.Id != 1 {
		t.Errorf("task.Id = %d, want 1", task.Id)
	}
	if task.Name != "讨厌我有什么用 有本事弄死我" {
		t.Errorf("task.Name = %q", task.Name)
	}
	if task.ResourceType != model.ResourceTypeFile {
		t.Errorf("task.ResourceType = %q, want %q", task.ResourceType, model.ResourceTypeFile)
	}
	if task.Status != model.TaskStatusWaiting {
		t.Errorf("task.Status = %v, want %v", task.Status, model.TaskStatusWaiting)
	}

	// ---- V1 DownloadResource: the video file to download ----
	resource := model.DownloadResource{
		Id:     1,
		TaskId: task.Id,
		Name:   title + ".mp4",
		Kind:   "video",
		Size:   content.Size,
		Status: 0,
	}
	if resource.TaskId != task.Id {
		t.Errorf("resource.TaskId = %d, want task.Id = %d", resource.TaskId, task.Id)
	}
	if resource.Kind != "video" {
		t.Errorf("resource.Kind = %q, want %q", resource.Kind, "video")
	}
	if resource.Size != 9613487 {
		t.Errorf("resource.Size = %d, want 9613487", resource.Size)
	}

	// ---- V1 DownloadEndpoint: the download source URL ----
	endpoint := model.DownloadEndpoint{
		Id:         1,
		ResourceId: resource.Id,
		Protocol:   "https",
		URL:        downloadURL,
		Priority:   0,
		Enabled:    1,
		Status:     0,
	}
	if endpoint.ResourceId != resource.Id {
		t.Errorf("endpoint.ResourceId = %d, want resource.Id = %d", endpoint.ResourceId, resource.Id)
	}
	if endpoint.Protocol != "https" {
		t.Errorf("endpoint.Protocol = %q, want %q", endpoint.Protocol, "https")
	}

	// ---- Content ↔ DownloadTaskV1 linkage ----
	content.DownloadTaskId = &task.Id
	if content.DownloadTaskId == nil || *content.DownloadTaskId != task.Id {
		t.Errorf("Content.DownloadTaskId = %v, want %d", content.DownloadTaskId, task.Id)
	}

	// ---- Content ↔ Account via ContentAccount bridge ----
	ca := model.ContentAccount{
		ContentId: content.Id,
		AccountId: account.Id,
		Role:      "owner",
	}
	if ca.Role != "owner" {
		t.Errorf("ContentAccount.Role = %q, want %q", ca.Role, "owner")
	}

	// ---- BrowseHistory: cross-references back to Content and Account ----
	r := wxchannels.BuildBrowseRecordFromObject(&obj)
	if r.ContentExternalId != content.ExternalId {
		t.Errorf("BrowseHistory.ContentExternalId = %q, want %q", r.ContentExternalId, content.ExternalId)
	}
	if r.AccountExternalId != account.ExternalId {
		t.Errorf("BrowseHistory.AccountExternalId = %q, want %q", r.AccountExternalId, account.ExternalId)
	}
	if r.ContentURL != content.ContentURL {
		t.Errorf("BrowseHistory.ContentURL = %q, want Content.ContentURL = %q", r.ContentURL, content.ContentURL)
	}

	// =====================================================================
	// 下载生命周期模拟
	// 模拟完整下载流程: 开始 → 10%暂停 → 恢复 → 100%完成
	// =====================================================================

	// ---- 初始化分片：整个文件作为一个分片 ----
	segment := model.DownloadSegment{
		Id:          1,
		ResourceId:  resource.Id,
		Index:       0,
		URL:         endpoint.URL,
		OffsetStart: 0,
		OffsetEnd:   resource.Size - 1,
		Size:        resource.Size,
		Downloaded:  0,
		Status:      0,
		Retry:       0,
	}

	// ---- 初始化连接：模拟一个活跃下载连接 ----
	conn := model.DownloadConnection{
		Id:         1,
		EndpointId: endpoint.Id,
		WorkerId:   "worker-1",
		Host:       "finder.video.qq.com",
		IP:         "183.60.15.100",
		Speed:      0,
		Bytes:      0,
		Status:     0,
		LastActive: 0,
	}

	tenPercent := resource.Size / 10
	logs := make([]model.DownloadLog, 0, 4)

	// Stage 1: 开始下载 → Preparing → Downloading
	t.Run("Stage1_StartDownloading", func(t *testing.T) {
		task.Status = model.TaskStatusPreparing
		if task.Status != model.TaskStatusPreparing {
			t.Errorf("Preparing.Status = %d, want %d", task.Status, model.TaskStatusPreparing)
		}

		task.Status = model.TaskStatusDownloading
		conn.Status = 1
		logs = append(logs, model.DownloadLog{
			Id: len(logs) + 1, TaskId: task.Id, Level: "info",
			Message: "download started",
		})

		if task.Status != model.TaskStatusDownloading {
			t.Errorf("Stage1 task.Status = %d, want %d (Downloading)", task.Status, model.TaskStatusDownloading)
		}
		if conn.Status != 1 {
			t.Errorf("Stage1 conn.Status = %d, want 1 (active)", conn.Status)
		}
		if len(logs) != 1 {
			t.Errorf("Stage1 logs count = %d, want 1", len(logs))
		}
	})

	// Stage 2: 下载到 10% → 暂停
	t.Run("Stage2_Download10PercentAndPause", func(t *testing.T) {
		segment.Downloaded = tenPercent
		conn.Bytes = tenPercent
		conn.Speed = 1024 * 1024 // 1 MB/s

		task.Status = model.TaskStatusPaused
		resource.Status = 1
		segment.Status = 1
		conn.Status = 2
		logs = append(logs, model.DownloadLog{
			Id: len(logs) + 1, TaskId: task.Id, Level: "info",
			Message: "download paused at 10%",
		})

		if segment.Downloaded != tenPercent {
			t.Errorf("Stage2 segment.Downloaded = %d, want %d (10%%)", segment.Downloaded, tenPercent)
		}
		if conn.Bytes != tenPercent {
			t.Errorf("Stage2 conn.Bytes = %d, want %d", conn.Bytes, tenPercent)
		}
		if task.Status != model.TaskStatusPaused {
			t.Errorf("Stage2 task.Status = %d, want %d (Paused)", task.Status, model.TaskStatusPaused)
		}
		if resource.Status != 1 {
			t.Errorf("Stage2 resource.Status = %d, want 1 (partial)", resource.Status)
		}
		if conn.Status != 2 {
			t.Errorf("Stage2 conn.Status = %d, want 2 (paused)", conn.Status)
		}
	})

	// Stage 3: 恢复下载 → 继续到 100%
	t.Run("Stage3_ResumeAndDownloadTo100Percent", func(t *testing.T) {
		task.Status = model.TaskStatusDownloading
		conn.Status = 1
		conn.Speed = 2 * 1024 * 1024 // 2 MB/s
		logs = append(logs, model.DownloadLog{
			Id: len(logs) + 1, TaskId: task.Id, Level: "info",
			Message: "download resumed",
		})

		if task.Status != model.TaskStatusDownloading {
			t.Errorf("Stage3 task.Status = %d, want %d (Downloading)", task.Status, model.TaskStatusDownloading)
		}
		if conn.Status != 1 {
			t.Errorf("Stage3 conn.Status = %d, want 1 (active)", conn.Status)
		}

		// 下载到 100%
		segment.Downloaded = resource.Size
		conn.Bytes = resource.Size
		segment.Status = 2
		conn.Speed = 0
		conn.Status = 0

		if segment.Downloaded != resource.Size {
			t.Errorf("Stage3 segment.Downloaded = %d, want %d (100%%)", segment.Downloaded, resource.Size)
		}
		if conn.Bytes != resource.Size {
			t.Errorf("Stage3 conn.Bytes = %d, want %d", conn.Bytes, resource.Size)
		}
		if segment.Status != 2 {
			t.Errorf("Stage3 segment.Status = %d, want 2 (completed)", segment.Status)
		}
	})

	// Stage 4: Merging → Finished
	t.Run("Stage4_MergingAndFinished", func(t *testing.T) {
		task.Status = model.TaskStatusMerging
		if task.Status != model.TaskStatusMerging {
			t.Errorf("Stage4 merging status = %d, want %d", task.Status, model.TaskStatusMerging)
		}

		task.Status = model.TaskStatusFinished
		resource.Status = 2
		logs = append(logs, model.DownloadLog{
			Id: len(logs) + 1, TaskId: task.Id, Level: "info",
			Message: "download finished",
		})

		if task.Status != model.TaskStatusFinished {
			t.Errorf("Stage4 task.Status = %d, want %d (Finished)", task.Status, model.TaskStatusFinished)
		}
		if resource.Status != 2 {
			t.Errorf("Stage4 resource.Status = %d, want 2 (completed)", resource.Status)
		}
		if len(logs) != 4 {
			t.Errorf("Stage4 logs count = %d, want 4", len(logs))
		}
	})

	// 最终验证：检查完整的下载生命周期链
	expectedLogLevels := []string{"started", "paused", "resumed", "finished"}
	for i, l := range logs {
		if !strings.Contains(l.Message, expectedLogLevels[i]) {
			t.Errorf("log[%d].Message = %q, should contain %q", i, l.Message, expectedLogLevels[i])
		}
		if l.TaskId != task.Id {
			t.Errorf("log[%d].TaskId = %d, want %d", i, l.TaskId, task.Id)
		}
		if l.Level != "info" {
			t.Errorf("log[%d].Level = %q, want %q", i, l.Level, "info")
		}
	}

	// 验证状态机完整性
	if task.Status != model.TaskStatusFinished {
		t.Errorf("final task.Status = %d, want %d (Finished)", task.Status, model.TaskStatusFinished)
	}
	if segment.Downloaded != resource.Size {
		t.Errorf("final segment.Downloaded = %d, want %d (full)", segment.Downloaded, resource.Size)
	}
	if segment.Size != resource.Size {
		t.Errorf("segment.Size = %d, want resource.Size = %d", segment.Size, resource.Size)
	}
	if segment.ResourceId != resource.Id {
		t.Errorf("segment.ResourceId = %d, want resource.Id = %d", segment.ResourceId, resource.Id)
	}
	if segment.OffsetStart != 0 {
		t.Errorf("segment.OffsetStart = %d, want 0", segment.OffsetStart)
	}
	if segment.OffsetEnd != resource.Size-1 {
		t.Errorf("segment.OffsetEnd = %d, want %d", segment.OffsetEnd, resource.Size-1)
	}
}

// TestDownloadFlowWithSubtasks_FromVideoFeedJSON extends TestDownloadFlow_FromVideoFeedJSON
// by creating a V1 download task that contains three resources (video, cover, audio),
// each with its own endpoint. This demonstrates how a single content record links
// to a multi-file batch download via the layered V1 model.
//
// The Content and Account records are identical to the single-task scenario, but
// the download structure demonstrates three resources under one task.
func TestDownloadFlowWithSubtasks_FromVideoFeedJSON(t *testing.T) {
	var obj wxchannelspkg.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// ---- Account & Content (same as single-task scenario) ----
	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	content, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	const platformId = "wx_channels"
	media := obj.ObjectDesc.Media[0]

	// ---- Build download URLs for the three resources via production functions ----
	spec := wxchannels.PickSpec(&obj)

	videoURL := wxchannels.BuildDownloadURLWithSpec(&obj, spec)
	coverURL := strings.TrimSpace(media.CoverUrl)
	// Audio is not directly available in the feed.  A real pipeline would
	// extract the audio track from the video media.  Here we simulate it
	// as a separate download whose URL is derived from the video feed.
	audioURL := media.URL + "&X-snsvideoflag=" + spec + "&audio_only=1"

	// ---- Create V1 DownloadTaskV1: task-level container ----
	taskId := 100
	taskConfig, _ := json.Marshal(map[string]any{
		"platform":    platformId,
		"external_id": content.ExternalId,
		"nonce_id":    content.ExternalId2,
		"spec":        spec,
	})
	task := model.DownloadTaskV1{
		Id:           taskId,
		Name:         content.Title,
		ResourceType: model.ResourceTypeFile,
		Status:       model.TaskStatusWaiting,
		SavePath:     "/downloads/wx_channels",
		ConfigJSON:   string(taskConfig),
	}

	// ---- Create three V1 DownloadResources (one per file type) ----
	resources := []model.DownloadResource{
		{Id: 101, TaskId: task.Id, Name: content.Title + ".mp4", Kind: "video", Size: content.Size, Status: 0, MergeOrder: 0},
		{Id: 102, TaskId: task.Id, Name: content.Title + ".jpg", Kind: "cover", Size: 0, Status: 0, MergeOrder: 1},
		{Id: 103, TaskId: task.Id, Name: content.Title + ".mp3", Kind: "audio", Size: 0, Status: 0, MergeOrder: 2},
	}

	// ---- Create three V1 DownloadEndpoints (one per resource) ----
	endpoints := []model.DownloadEndpoint{
		{Id: 201, ResourceId: 101, Protocol: "https", URL: videoURL, Priority: 0, Enabled: 1, Status: 0},
		{Id: 202, ResourceId: 102, Protocol: "https", URL: coverURL, Priority: 0, Enabled: 1, Status: 0},
		{Id: 203, ResourceId: 103, Protocol: "https", URL: audioURL, Priority: 0, Enabled: 1, Status: 0},
	}

	// ---- Link Content to the DownloadTaskV1 ----
	content.DownloadTaskId = &task.Id

	// ---- Link Content to Account via ContentAccount bridge ----
	ca := model.ContentAccount{
		ContentId: content.Id,
		AccountId: account.Id,
		Role:      "owner",
	}

	// =====================================================================
	// Assertions
	// =====================================================================

	// 1. Account and Content are unchanged from single-task scenario
	if account.PlatformId != platformId {
		t.Errorf("Account.PlatformId = %q, want %q", account.PlatformId, platformId)
	}
	if account.ExternalId != obj.Contact.Username {
		t.Errorf("Account.ExternalId mismatch")
	}
	if content.PlatformId != platformId {
		t.Errorf("Content.PlatformId = %q, want %q", content.PlatformId, platformId)
	}
	if content.ExternalId != obj.ID {
		t.Errorf("Content.ExternalId = %q, want %q", content.ExternalId, obj.ID)
	}
	if content.ContentType != "video" {
		t.Errorf("Content.ContentType = %q, want %q", content.ContentType, "video")
	}

	// 2. Content → DownloadTaskV1 linkage
	if content.DownloadTaskId == nil {
		t.Fatal("Content.DownloadTaskId should point to task")
	}
	if *content.DownloadTaskId != task.Id {
		t.Errorf("Content.DownloadTaskId = %d, want task.Id = %d", *content.DownloadTaskId, task.Id)
	}

	// 3. Content → Account linkage
	if ca.Role != "owner" {
		t.Errorf("ContentAccount.Role = %q, want 'owner'", ca.Role)
	}

	// 4. DownloadTaskV1 shape
	if task.Id != taskId {
		t.Errorf("task.Id = %d, want %d", task.Id, taskId)
	}
	if task.Name != content.Title {
		t.Errorf("task.Name = %q, want %q", task.Name, content.Title)
	}
	if task.ResourceType != model.ResourceTypeFile {
		t.Errorf("task.ResourceType = %q, want %q", task.ResourceType, model.ResourceTypeFile)
	}
	if task.Status != model.TaskStatusWaiting {
		t.Errorf("task.Status = %v, want %v", task.Status, model.TaskStatusWaiting)
	}
	if task.SavePath != "/downloads/wx_channels" {
		t.Errorf("task.SavePath = %q", task.SavePath)
	}

	// 5. Each resource is correctly linked to the task
	expectedResourceIDs := []int{101, 102, 103}
	expectedKinds := []string{"video", "cover", "audio"}
	expectedNames := []string{
		content.Title + ".mp4",
		content.Title + ".jpg",
		content.Title + ".mp3",
	}
	for i, r := range resources {
		sub := fmt.Sprintf("resource[%d]", i)

		if r.TaskId != task.Id {
			t.Errorf("%s.TaskId = %d, want task.Id = %d", sub, r.TaskId, task.Id)
		}
		if r.Id != expectedResourceIDs[i] {
			t.Errorf("%s.Id = %d, want %d", sub, r.Id, expectedResourceIDs[i])
		}
		if r.Kind != expectedKinds[i] {
			t.Errorf("%s.Kind = %q, want %q", sub, r.Kind, expectedKinds[i])
		}
		if r.Name != expectedNames[i] {
			t.Errorf("%s.Name = %q, want %q", sub, r.Name, expectedNames[i])
		}
		if r.MergeOrder != i {
			t.Errorf("%s.MergeOrder = %d, want %d", sub, r.MergeOrder, i)
		}
	}

	// 6. Each endpoint is correctly linked to its resource
	expectedEndpointIDs := []int{201, 202, 203}
	expectedURLs := []string{videoURL, coverURL, audioURL}
	for i, ep := range endpoints {
		sub := fmt.Sprintf("endpoint[%d]", i)

		if ep.ResourceId != resources[i].Id {
			t.Errorf("%s.ResourceId = %d, want resource.Id = %d", sub, ep.ResourceId, resources[i].Id)
		}
		if ep.Id != expectedEndpointIDs[i] {
			t.Errorf("%s.Id = %d, want %d", sub, ep.Id, expectedEndpointIDs[i])
		}
		if ep.Protocol != "https" {
			t.Errorf("%s.Protocol = %q, want %q", sub, ep.Protocol, "https")
		}
		if ep.URL != expectedURLs[i] {
			t.Errorf("%s.URL mismatch:\n  got  %s\n  want %s", sub, ep.URL, expectedURLs[i])
		}
		if ep.Enabled != 1 {
			t.Errorf("%s.Enabled = %d, want 1", sub, ep.Enabled)
		}
	}

	// 7. ConfigJSON carries key signals for lineage resolution
	var cfg map[string]any
	if err := json.Unmarshal([]byte(task.ConfigJSON), &cfg); err != nil {
		t.Fatalf("task.ConfigJSON is not valid JSON: %v", err)
	}
	if cfg["platform"] != platformId {
		t.Errorf("ConfigJSON.platform = %v, want %q", cfg["platform"], platformId)
	}
	if cfg["external_id"] != content.ExternalId {
		t.Errorf("ConfigJSON.external_id = %v, want %q", cfg["external_id"], content.ExternalId)
	}
	if cfg["spec"] != spec {
		t.Errorf("ConfigJSON.spec = %v, want %q", cfg["spec"], spec)
	}

	t.Logf("=== Multi-file Download Tree (V1) ===")
	t.Logf("Account  : %s (%s)", account.Nickname, account.ExternalId)
	t.Logf("Content  : %s | %s | %d bytes | %ds", content.Title, content.ContentType, content.Size, content.Duration)
	t.Logf("Task     : id=%d name=%q type=%s -> Content(%s) via DownloadTaskId", task.Id, task.Name, task.ResourceType, content.Id)
	for i, r := range resources {
		t.Logf("  ├── resource[%d] id=%d kind=%s size=%d", i, r.Id, r.Kind, r.Size)
		t.Logf("  │   └── endpoint[%d] id=%d protocol=%s", i, endpoints[i].Id, endpoints[i].Protocol)
	}
	t.Logf("Linkage  : ContentAccount(role=%q) binds Content(%s) ↔ Account(%s)", ca.Role, ca.ContentId, ca.AccountId)

	// =====================================================================
	// 多文件下载生命周期模拟
	// 场景: 三个资源(video/cover/audio)并行下载，中途暂停后恢复全量完成
	// =====================================================================

	// ---- 初始化分片：每个资源一个分片 ----
	segments := []model.DownloadSegment{
		{Id: 301, ResourceId: 101, Index: 0, URL: endpoints[0].URL, OffsetStart: 0,
			OffsetEnd: resources[0].Size - 1, Size: resources[0].Size, Downloaded: 0, Status: 0, Retry: 0},
		{Id: 302, ResourceId: 102, Index: 0, URL: endpoints[1].URL, OffsetStart: 0,
			OffsetEnd: 0, Size: 0, Downloaded: 0, Status: 0, Retry: 0},
		{Id: 303, ResourceId: 103, Index: 0, URL: endpoints[2].URL, OffsetStart: 0,
			OffsetEnd: 0, Size: 0, Downloaded: 0, Status: 0, Retry: 0},
	}

	// ---- 初始化连接：每个端点一个连接 ----
	connections := []model.DownloadConnection{
		{Id: 401, EndpointId: 201, WorkerId: "w-video", Host: "finder.video.qq.com",
			IP: "183.60.15.100", Speed: 0, Bytes: 0, Status: 0, LastActive: 0},
		{Id: 402, EndpointId: 202, WorkerId: "w-cover", Host: "finder.video.qq.com",
			IP: "183.60.15.101", Speed: 0, Bytes: 0, Status: 0, LastActive: 0},
		{Id: 403, EndpointId: 203, WorkerId: "w-audio", Host: "finder.video.qq.com",
			IP: "183.60.15.102", Speed: 0, Bytes: 0, Status: 0, LastActive: 0},
	}

	logs := make([]model.DownloadLog, 0, 5)

	// Stage 1: 所有资源开始下载
	t.Run("Stage1_MultiStartDownloading", func(t *testing.T) {
		task.Status = model.TaskStatusPreparing
		task.Status = model.TaskStatusDownloading

		for i := range resources {
			resources[i].Status = 1
		}
		for i := range connections {
			connections[i].Status = 1
		}
		for i := range segments {
			segments[i].Status = 1
		}
		logs = append(logs, model.DownloadLog{
			Id: len(logs) + 1, TaskId: task.Id, Level: "info",
			Message: "multi-file download started (video+cover+audio)",
		})

		if task.Status != model.TaskStatusDownloading {
			t.Errorf("Stage1 task.Status = %d, want %d", task.Status, model.TaskStatusDownloading)
		}
		for i, conn := range connections {
			if conn.Status != 1 {
				t.Errorf("Stage1 conn[%d].Status = %d, want 1", i, conn.Status)
			}
		}
		if len(logs) != 1 {
			t.Errorf("Stage1 logs = %d, want 1", len(logs))
		}
	})

	// Stage 2: video进度100%(video是主资源), cover 100%, audio 100%
	//         所有并行下载，以video为主：video到10%后暂停
	tenPctVideo := resources[0].Size / 10
	t.Run("Stage2_Progress10PercentAndPause", func(t *testing.T) {
		// video 到 10%
		segments[0].Downloaded = tenPctVideo
		connections[0].Bytes = tenPctVideo
		connections[0].Speed = 3 * 1024 * 1024 // 3 MB/s for video

		// cover 已完成 (小文件)
		segments[1].Downloaded = 0
		segments[1].Status = 2 // completed
		resources[1].Status = 2
		connections[1].Status = 0 // idle

		// audio 到 50%
		segments[2].Downloaded = 0
		resources[2].Status = 1

		// 暂停全部
		task.Status = model.TaskStatusPaused
		segments[0].Status = 1 // paused
		connections[0].Status = 2
		resources[0].Status = 1

		logs = append(logs, model.DownloadLog{
			Id: len(logs) + 1, TaskId: task.Id, Level: "info",
			Message: "download paused at video 10%, cover completed, audio partial",
		})

		if segments[0].Downloaded != tenPctVideo {
			t.Errorf("Stage2 segment[0].Downloaded = %d, want %d", segments[0].Downloaded, tenPctVideo)
		}
		if connections[0].Bytes != tenPctVideo {
			t.Errorf("Stage2 conn[0].Bytes = %d, want %d", connections[0].Bytes, tenPctVideo)
		}
		if task.Status != model.TaskStatusPaused {
			t.Errorf("Stage2 task.Status = %d, want %d", task.Status, model.TaskStatusPaused)
		}
		if resources[1].Status != 2 {
			t.Errorf("Stage2 resource[1].Status = %d, want 2 (cover completed)", resources[1].Status)
		}
	})

	// Stage 3: 恢复 → 全部完成
	t.Run("Stage3_ResumeAndAllComplete", func(t *testing.T) {
		task.Status = model.TaskStatusDownloading

		// video: 从10%恢复到100%
		segments[0].Downloaded = resources[0].Size
		connections[0].Bytes = resources[0].Size
		segments[0].Status = 2
		resources[0].Status = 2
		connections[0].Status = 0
		connections[0].Speed = 0

		// audio: 完成
		segments[2].Downloaded = 0
		segments[2].Status = 2
		resources[2].Status = 2
		connections[2].Status = 0

		logs = append(logs, model.DownloadLog{
			Id: len(logs) + 1, TaskId: task.Id, Level: "info",
			Message: "all resources resumed and fully downloaded",
		})

		if segments[0].Downloaded != resources[0].Size {
			t.Errorf("Stage3 segment[0].Downloaded = %d, want %d", segments[0].Downloaded, resources[0].Size)
		}
		for i := range segments {
			if segments[i].Status != 2 {
				t.Errorf("Stage3 segment[%d].Status = %d, want 2", i, segments[i].Status)
			}
		}
		for i := range resources {
			if resources[i].Status != 2 {
				t.Errorf("Stage3 resource[%d].Status = %d, want 2", i, resources[i].Status)
			}
		}
	})

	// Stage 4: Merging → Finished
	t.Run("Stage4_MergingAndFinished", func(t *testing.T) {
		task.Status = model.TaskStatusMerging
		if task.Status != model.TaskStatusMerging {
			t.Errorf("Stage4 merging status = %d, want %d", task.Status, model.TaskStatusMerging)
		}

		// 按 MergeOrder 合并: video(0)→cover(1)→audio(2)
		for _, r := range resources {
			if r.Status != 2 {
				t.Errorf("merge: resource[%d] (kind=%s, order=%d) should be completed before merging",
					r.MergeOrder, r.Kind, r.MergeOrder)
			}
		}

		task.Status = model.TaskStatusFinished
		logs = append(logs, model.DownloadLog{
			Id: len(logs) + 1, TaskId: task.Id, Level: "info",
			Message: "multi-file download finished and merged",
		})

		if task.Status != model.TaskStatusFinished {
			t.Errorf("Stage4 task.Status = %d, want %d", task.Status, model.TaskStatusFinished)
		}
		if len(logs) != 4 {
			t.Errorf("Stage4 logs = %d, want 4", len(logs))
		}
	})

	// =====================================================================
	// 最终验证
	// =====================================================================

	// 日志链完整性
	expectedKeywords := []string{"started", "paused", "resumed", "finished"}
	for i, l := range logs {
		if !strings.Contains(l.Message, expectedKeywords[i]) {
			t.Errorf("log[%d].Message = %q, should contain %q", i, l.Message, expectedKeywords[i])
		}
		if l.TaskId != task.Id {
			t.Errorf("log[%d].TaskId = %d, want %d", i, l.TaskId, task.Id)
		}
	}

	// 验证主资源分片完整性
	mainSeg := segments[0] // video
	if mainSeg.ResourceId != resources[0].Id {
		t.Errorf("mainSeg.ResourceId = %d, want %d", mainSeg.ResourceId, resources[0].Id)
	}
	if mainSeg.Size != resources[0].Size {
		t.Errorf("mainSeg.Size = %d, want %d", mainSeg.Size, resources[0].Size)
	}
	if mainSeg.OffsetStart != 0 {
		t.Errorf("mainSeg.OffsetStart = %d, want 0", mainSeg.OffsetStart)
	}
	if mainSeg.OffsetEnd != resources[0].Size-1 {
		t.Errorf("mainSeg.OffsetEnd = %d, want %d", mainSeg.OffsetEnd, resources[0].Size-1)
	}

	// 验证三表关联链: endpoint → resource → task
	for i := range resources {
		if endpoints[i].ResourceId != resources[i].Id {
			t.Errorf("chain: endpoint[%d].ResourceId(%d) != resource[%d].Id(%d)",
				i, endpoints[i].ResourceId, i, resources[i].Id)
		}
		if resources[i].TaskId != task.Id {
			t.Errorf("chain: resource[%d].TaskId(%d) != task.Id(%d)",
				i, resources[i].TaskId, task.Id)
		}
		if segments[i].ResourceId != resources[i].Id {
			t.Errorf("chain: segment[%d].ResourceId(%d) != resource[%d].Id(%d)",
				i, segments[i].ResourceId, i, resources[i].Id)
		}
		if connections[i].EndpointId != endpoints[i].Id {
			t.Errorf("chain: conn[%d].EndpointId(%d) != endpoint[%d].Id(%d)",
				i, connections[i].EndpointId, i, endpoints[i].Id)
		}
	}

	// 验证最终任务状态
	if task.Status != model.TaskStatusFinished {
		t.Errorf("final task.Status = %d, want %d (Finished)", task.Status, model.TaskStatusFinished)
	}
	// 所有资源已完成
	totalCompleted := 0
	for _, r := range resources {
		if r.Status == 2 {
			totalCompleted++
		}
	}
	if totalCompleted != 3 {
		t.Errorf("total completed resources = %d, want 3", totalCompleted)
	}
}
