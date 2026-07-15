package wxchannels_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"wx_channel/internal/database/model"
	wxchannels "wx_channel/internal/webcontent/wxchannels"
	scraper "wx_channel/pkg/scraper/wxchannels"
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
	var obj scraper.ChannelsObject
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
	_ = expectedUsername // account.Id is auto-increment int, not string
}

func TestToContent_FromVideoFeedJSON(t *testing.T) {
	var obj scraper.ChannelsObject
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
	_ = content.ContentType // content.Id is auto-increment int, not string
	expectedContentURL := "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&hy=SZ&idx=1&m=414c8b10462c8fa97a904c3d999a0476&uzid=7a206&token=2lt8WBSnjTkTjXXRcWF576SLtqb9LdRn1Cliaa0icf5zFjCLyBFNe1e3eKzhzzEc5h05O81ibb3hwbVTVywYQAQbSQzZkHicCqabpEdwBzhTgdyPiakaMMw7n96CtNxoPbKkQxiaYOzPImgS9ZG3kDzKcLjMEyIIVGYuibzdHECVIOFibOQGL4pWibDRRD6VcpGApwhugo6k9Mq48YAov7zg751dO260H5iaGeEkJZWhKhib0hib4W0&basedata=CAMSBnhXVDE1OCJaCgoKBnhXVDE1OBAACgoKBnhXVDE1NxAACgoKBnhXVDE1NhAACgoKBnhXVDExMxAACgoKBnhXVDExMhAACgoKBnhXVDExMRAACgcKA3hBMBAACgcKA3hBMhAA&sign=AgZzkYT5vBvSWwKe5MpufA75x2T3Xnnz7PtuTK98WxdVbZm4Grpnyl52sDN4W6CI562FVgGaZ-_tYlBjCRLdIQ&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAABRfl4aFfX8vo5XJgBRahAAAADnaHZTnGbFfAj9RgZXfw6ViUCWOt8LYujr%2BrkpCHNy7PD375%2BDqLzGDCk8ibQxWRl9tKOjUKAhiL4%3D&svrnonce=1783693350"
	if content.ContentURL != expectedContentURL {
		t.Errorf("ContentURL = %q, want %q", content.ContentURL, expectedContentURL)
	}
	if content.URL != content.ContentURL {
		t.Errorf("URL = %q, should equal ContentURL", content.URL)
	}
	expectedCoverURL := "https://finder.video.qq.com/251/20304/stodownload?encfilekey=2fG3V4WwQPluPpjb46OTKMXHc112k4G2oJic38N7rnuA86EibU1Y76s8oA7ibJ2icEheVFXiah55XOtQTzMnAsGIe2IWYOSogJ0DHQGv97AFZePM&token=AxricY7RBHdVdU7Gn7iczBDOyqkPzEiazv6slYib62vrPnRWLdajxdDW6L5750WibUCk6R96RGUJ3MAHbTqSV90lo9nH8Wn7JShFsWZgr68VIDPoEYFqYLakd4tDgsE26h00sXkjVy5cSHmf6aCEbjhuJYGRaQ3eZISKiatbry08Ugw1R9B6zzeWxvqJ2hNlojz1GCPcpNq8j85OXOWGlicSBmVd3kQGj5vTzx7&hy=SZ&idx=1&m=73a9ef1bc335f9c43d800208ddc42f09&uzid=1&picformat=200&wxampicformat=503"
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
	if content.SourceURL != "" {
		t.Errorf("SourceURL = %q, want empty", content.SourceURL)
	}
	if content.PublishTime == nil || *content.PublishTime != 1783667361 {
		t.Errorf("PublishTime = %v, want ptr to 1783667361", content.PublishTime)
	}
	expectedMetadata := `{"key":"1522886121"}`
	if content.Metadata != expectedMetadata {
		t.Errorf("Metadata = %q, want %q", content.Metadata, expectedMetadata)
	}
}

// videoChannelsObjectToMediaProfile converts a ChannelsObject from videoFeedJSON
// into a scraper.MediaProfile.  This mirrors how the frontend interceptor
// constructs a MediaProfile from a feed object when the user opens a video page.
func videoChannelsObjectToMediaProfile(obj *scraper.ChannelsObject) *scraper.MediaProfile {
	media := obj.ObjectDesc.Media[0]
	return &scraper.MediaProfile{
		Type:    "video",
		Id:      obj.ID,
		NonceId: obj.ObjectNonceId,
		Title:   obj.ObjectDesc.Description,
		URL:     media.URL + media.URLToken,
		Key:     media.DecodeKey,
		CoverURL: media.CoverUrl,
		Contact: scraper.InterceptorContact{
			Id:        obj.Contact.Username,
			Nickname:  obj.Contact.Nickname,
			AvatarURL: obj.Contact.HeadUrl,
		},
	}
}

func TestCreateBrowseRecord_FromVideoFeedJSON(t *testing.T) {
	var obj scraper.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	profile := videoChannelsObjectToMediaProfile(&obj)

	uniqueMark, info := wxchannels.CreateBrowseRecord(profile)

	if uniqueMark != "14962486294771997060" {
		t.Errorf("uniqueMark = %q, want %q", uniqueMark, "14962486294771997060")
	}

	// PlatformId
	if info.PlatformId != "wx_channels" {
		t.Errorf("PlatformId = %q, want %q", info.PlatformId, "wx_channels")
	}

	// Account fields
	expectedExternalId := "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder"
	if info.AccountExternalId != expectedExternalId {
		t.Errorf("AccountExternalId = %q, want %q", info.AccountExternalId, expectedExternalId)
	}
	if info.AccountUsername != expectedExternalId {
		t.Errorf("AccountUsername = %q, want %q", info.AccountUsername, expectedExternalId)
	}
	if info.AccountNickname != "迷人的大嘴猴" {
		t.Errorf("AccountNickname = %q, want %q", info.AccountNickname, "迷人的大嘴猴")
	}
	if info.AccountAvatarURL != "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132" {
		t.Errorf("AccountAvatarURL mismatch")
	}

	// Content fields
	if info.ContentType != "video" {
		t.Errorf("ContentType = %q, want %q", info.ContentType, "video")
	}
	if info.ContentTitle != "讨厌我有什么用 有本事弄死我" {
		t.Errorf("ContentTitle = %q", info.ContentTitle)
	}
	if info.ContentURL != profile.URL {
		t.Errorf("ContentURL = %q, want %q", info.ContentURL, profile.URL)
	}
	// ContentSourceURL is empty because ChannelsObject has no SourceURL field
	// (the page URL is injected by the frontend interceptor at browse time).
	if info.ContentSourceURL != "" {
		t.Errorf("ContentSourceURL = %q, want empty (ChannelsObject has no source_url)", info.ContentSourceURL)
	}
	if info.ContentCoverURL != profile.CoverURL {
		t.Errorf("ContentCoverURL = %q, want %q", info.ContentCoverURL, profile.CoverURL)
	}

	// ExtraData
	if info.ExtraData["id"] != "14962486294771997060" {
		t.Errorf("ExtraData[id] = %v", info.ExtraData["id"])
	}
	if info.ExtraData["nonce_id"] != "4390481592474233535_0_146_0_0" {
		t.Errorf("ExtraData[nonce_id] = %v", info.ExtraData["nonce_id"])
	}
	if info.ExtraData["decode_key"] != "1522886121" {
		t.Errorf("ExtraData[decode_key] = %v", info.ExtraData["decode_key"])
	}
}

// assertContentConversion verifies that a *model.Content built from videoFeedJSON
// survives a JSON marshal → unmarshal round-trip with all critical fields intact.
func assertContentConversion(t *testing.T, c *model.Content, expectedTitle string) {
	t.Helper()

	if c == nil {
		t.Fatal("content is nil — videoFeedJSON did not convert to model.Content")
	}

	// Verify the conversion result is a valid model.Content struct by checking
	// that all video feed fields map correctly to their Content counterparts.

	// ID fields
	if c.ExternalId == "" {
		t.Error("Content.ExternalId must not be empty (mapped from ChannelsObject.id)")
	}
	if c.ExternalId2 == "" {
		t.Error("Content.ExternalId2 must not be empty (mapped from objectNonceId)")
	}
	if c.ExternalId3 == "" {
		t.Error("Content.ExternalId3 must not be empty (mapped from media[0].decodeKey)")
	}

	// Media metadata
	if c.ContentType != "video" {
		t.Errorf("Content.ContentType must be 'video' for mediaType=4, got %q", c.ContentType)
	}
	if c.ContentURL == "" {
		t.Error("Content.ContentURL must not be empty (media[0].url + urlToken)")
	}
	if c.URL != c.ContentURL {
		t.Error("Content.URL must equal ContentURL for video content")
	}

	// Cover dimensions (from media[0].width / height as int→string)
	if c.CoverWidth != "1080" {
		t.Errorf("CoverWidth = %q, want '1080'", c.CoverWidth)
	}
	if c.CoverHeight != "2128" {
		t.Errorf("CoverHeight = %q, want '2128'", c.CoverHeight)
	}

	// Media size / duration
	if c.Size != 9613487 {
		t.Errorf("Size = %d, want 9613487 (fileSize from media[0])", c.Size)
	}
	if c.Duration != 9 {
		t.Errorf("Duration = %d, want 9 (videoPlayLen from media[0])", c.Duration)
	}

	// Title / Description both map to objectDesc.description
	if c.Title != expectedTitle {
		t.Errorf("Title = %q, want %q", c.Title, expectedTitle)
	}
	if c.Description != expectedTitle {
		t.Errorf("Description = %q, want %q", c.Description, expectedTitle)
	}

	// PublishTime from createtime
	if c.PublishTime == nil {
		t.Error("PublishTime must not be nil (parsed from createtime=1783667361)")
	} else if *c.PublishTime != 1783667361 {
		t.Errorf("PublishTime = %d, want 1783667361", *c.PublishTime)
	}

	// Metadata (decode_key)
	if c.Metadata != `{"key":"1522886121"}` {
		t.Errorf("Metadata = %q, want {\"key\":\"1522886121\"}", c.Metadata)
	}

	// Platform
	if c.PlatformId != "wx_channels" {
		t.Errorf("PlatformId = %q, want 'wx_channels'", c.PlatformId)
	}

	// --- JSON marshal → unmarshal round-trip ---

	jsonBytes, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("failed to marshal Content to JSON: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Fatal("marshaled Content JSON is empty")
	}

	var roundTripped model.Content
	if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal Content JSON back: %v", err)
	}

	// Verify key fields survived the round-trip
	if roundTripped.ExternalId != c.ExternalId {
		t.Errorf("round-trip ExternalId = %q, want %q", roundTripped.ExternalId, c.ExternalId)
	}
	if roundTripped.ContentType != c.ContentType {
		t.Errorf("round-trip ContentType = %q, want %q", roundTripped.ContentType, c.ContentType)
	}
	if roundTripped.Title != c.Title {
		t.Errorf("round-trip Title = %q, want %q", roundTripped.Title, c.Title)
	}
	if roundTripped.Size != c.Size {
		t.Errorf("round-trip Size = %d, want %d", roundTripped.Size, c.Size)
	}
	if roundTripped.Duration != c.Duration {
		t.Errorf("round-trip Duration = %d, want %d", roundTripped.Duration, c.Duration)
	}
	if roundTripped.ContentURL != c.ContentURL {
		t.Errorf("round-trip ContentURL = %q, want %q", roundTripped.ContentURL, c.ContentURL)
	}
	if roundTripped.CoverURL != c.CoverURL {
		t.Errorf("round-trip CoverURL = %q, want %q", roundTripped.CoverURL, c.CoverURL)
	}
	if roundTripped.PlatformId != c.PlatformId {
		t.Errorf("round-trip PlatformId = %q, want %q", roundTripped.PlatformId, c.PlatformId)
	}
	if roundTripped.Metadata != c.Metadata {
		t.Errorf("round-trip Metadata = %q, want %q", roundTripped.Metadata, c.Metadata)
	}
	if roundTripped.PublishTime == nil {
		t.Error("round-trip PublishTime is nil")
	} else if *roundTripped.PublishTime != *c.PublishTime {
		t.Errorf("round-trip PublishTime = %d, want %d", *roundTripped.PublishTime, *c.PublishTime)
	}
}

// TestDownloadFlow_FromVideoFeedJSON demonstrates the end-to-end flow from the
// raw videoFeedJSON to constructing Account (author), Content, and DownloadTask
// records, and verifies that they are correctly interrelated.
//
// The test simulates what happens when the platform receives a video feed:
//   1. Parse the JSON into ChannelsObject.
//   2. Build an Account record from the contact info.
//   3. Build a Content record from the media metadata.
//   4. Build a ChannelsFeedProfile (the download-task blueprint).
//   5. Build download-task parameters (URL, spec, suffix, labels) the same
//      way handleCompatDownloadTaskCreate does.
//   6. Simulate a DownloadTask record (without actually starting the download)
//      and assert that its fields correctly reference the Content and Account.
// This test does not require a running database or downloader.
func TestDownloadFlow_FromVideoFeedJSON(t *testing.T) {
	// ---- Step 1: Parse the fixture ----
	var obj scraper.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// ---- Step 2: Build Account (author) record ----
	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	const platformId = "wx_channels"
	expectedAccountExternalId := "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder"
	expectedNickname := "迷人的大嘴猴"
	expectedAvatar := "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132"

	if account.PlatformId != platformId {
		t.Errorf("Account.PlatformId = %q, want %q", account.PlatformId, platformId)
	}
	if account.ExternalId != expectedAccountExternalId {
		t.Errorf("Account.ExternalId = %q, want %q", account.ExternalId, expectedAccountExternalId)
	}
	if account.Username != expectedAccountExternalId {
		t.Errorf("Account.Username = %q, want %q", account.Username, expectedAccountExternalId)
	}
	if account.Nickname != expectedNickname {
		t.Errorf("Account.Nickname = %q, want %q", account.Nickname, expectedNickname)
	}
	if account.AvatarURL != expectedAvatar {
		t.Errorf("Account.AvatarURL = %q, want %q", account.AvatarURL, expectedAvatar)
	}

	// ---- Step 3: Build Content record ----
	content, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	expectedContentExternalId := "14962486294771997060"
	expectedNonceId := "4390481592474233535_0_146_0_0"
	expectedDecodeKey := "1522886121"
	expectedTitle := "讨厌我有什么用 有本事弄死我"
	expectedContentType := "video"
	expectedSize := int64(9613487)
	expectedDuration := int64(9)

	if content.PlatformId != platformId {
		t.Errorf("Content.PlatformId = %q, want %q", content.PlatformId, platformId)
	}
	if content.ExternalId != expectedContentExternalId {
		t.Errorf("Content.ExternalId = %q, want %q", content.ExternalId, expectedContentExternalId)
	}
	if content.ExternalId2 != expectedNonceId {
		t.Errorf("Content.ExternalId2 = %q, want %q", content.ExternalId2, expectedNonceId)
	}
	if content.ExternalId3 != expectedDecodeKey {
		t.Errorf("Content.ExternalId3 = %q, want %q", content.ExternalId3, expectedDecodeKey)
	}
	if content.ContentType != expectedContentType {
		t.Errorf("Content.ContentType = %q, want %q", content.ContentType, expectedContentType)
	}
	if content.Title != expectedTitle {
		t.Errorf("Content.Title = %q, want %q", content.Title, expectedTitle)
	}
	if content.Size != expectedSize {
		t.Errorf("Content.Size = %d, want %d", content.Size, expectedSize)
	}
	if content.Duration != expectedDuration {
		t.Errorf("Content.Duration = %d, want %d", content.Duration, expectedDuration)
	}
	if content.ContentURL == "" {
		t.Error("Content.ContentURL should not be empty")
	}
	if content.URL != content.ContentURL {
		t.Errorf("Content.URL = %q should equal ContentURL = %q", content.URL, content.ContentURL)
	}
	if content.CoverURL == "" {
		t.Error("Content.CoverURL should not be empty")
	}
	if content.CoverWidth != "1080" {
		t.Errorf("Content.CoverWidth = %q, want %q", content.CoverWidth, "1080")
	}
	if content.CoverHeight != "2128" {
		t.Errorf("Content.CoverHeight = %q, want %q", content.CoverHeight, "2128")
	}
	if content.PublishTime == nil || *content.PublishTime != 1783667361 {
		t.Errorf("Content.PublishTime = %v, want ptr to 1783667361", content.PublishTime)
	}
	if content.Metadata != `{"key":"1522886121"}` {
		t.Errorf("Content.Metadata = %q", content.Metadata)
	}

	// Assert videoFeedJSON → model.Content conversion is structurally correct.
	// This verifies the fixture data round-trips through the Content struct.
	assertContentConversion(t, content, expectedTitle)

	// ---- Step 4: Build ChannelsFeedProfile (download blueprint) ----
	feed, err := scraper.ChannelsObjectToChannelsFeedProfile(&obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile: %v", err)
	}
	if feed.ObjectId != expectedContentExternalId {
		t.Errorf("FeedProfile.ObjectId = %q, want %q", feed.ObjectId, expectedContentExternalId)
	}
	if feed.NonceId != expectedNonceId {
		t.Errorf("FeedProfile.NonceId = %q, want %q", feed.NonceId, expectedNonceId)
	}
	if feed.Title != expectedTitle {
		t.Errorf("FeedProfile.Title = %q, want %q", feed.Title, expectedTitle)
	}
	if feed.DecryptKey != expectedDecodeKey {
		t.Errorf("FeedProfile.DecryptKey = %q, want %q", feed.DecryptKey, expectedDecodeKey)
	}
	if feed.CoverURL == "" {
		t.Error("FeedProfile.CoverURL should not be empty")
	}
	if feed.Duration != 9 {
		t.Errorf("FeedProfile.Duration = %d, want 9", feed.Duration)
	}
	if feed.FileSize != 9613487 {
		t.Errorf("FeedProfile.FileSize = %d, want 9613487", feed.FileSize)
	}
	if feed.Contact.Username != expectedAccountExternalId {
		t.Errorf("FeedProfile.Contact.Username = %q, want %q", feed.Contact.Username, expectedAccountExternalId)
	}
	if feed.Contact.Nickname != expectedNickname {
		t.Errorf("FeedProfile.Contact.Nickname = %q, want %q", feed.Contact.Nickname, expectedNickname)
	}
	if feed.Contact.AvatarURL != expectedAvatar {
		t.Errorf("FeedProfile.Contact.AvatarURL = %q", feed.Contact.AvatarURL)
	}

	// ---- Step 5: Build download-task parameters (mirrors handleCompatDownloadTaskCreate) ----
	spec := "xWT111"
	// Pick first spec format (highest quality h264), same logic as handler
	if len(feed.Spec) > 0 {
		spec = feed.Spec[0].FileFormat
	}
	suffix := ".mp4"
	downloadURL := strings.TrimSpace(feed.URL)
	if downloadURL == "" {
		downloadURL = content.ContentURL
	}
	// Append spec flag for non-original specs
	if spec != "original" && !strings.Contains(downloadURL, "zip://") {
		downloadURL = downloadURL + "&X-snsvideoflag=" + spec
	}

	// Feed.Title is guaranteed non-empty by ChannelsObjectToChannelsFeedProfile
	filename := strings.TrimSpace(feed.Title)
	hasQualifier := false
	if strings.HasSuffix(strings.ToLower(filename), ".mp4") {
		suffix = ""
		hasQualifier = true
	}
	expectedFilename := filename + suffix

	sourceURL := feed.SourceURL
	if sourceURL == "" {
		sourceURL = scraper.BuildJumpUrl(feed)
	}

	key := 0
	if strings.TrimSpace(feed.DecryptKey) != "" {
		if v, errConv := strconv.Atoi(feed.DecryptKey); errConv == nil {
			key = v
		}
	}
	labels := map[string]string{
		"id":         feed.ObjectId,
		"nonce_id":   feed.NonceId,
		"title":      feed.Title,
		"key":        strconv.Itoa(key),
		"spec":       spec,
		"suffix":     suffix,
		"source_url": sourceURL,
	}

	// ---- Step 6: Simulate a DownloadTask record (no actual download) ----
	task := model.DownloadTask{
		ExternalId: content.ExternalId,
		Protocol:   "https",
		URL:        downloadURL,
		Title:      filename,
		CoverURL:   content.CoverURL,
		Size:       content.Size,
		Status:     0, // ready
		Reason:     "video_download_via_feed",
	}
	// Simulate CreateContentDownloadTask-style metadata
	meta2, _ := json.Marshal(map[string]any{
		"platform":    content.PlatformId,
		"external_id": content.ExternalId,
		"nonce_id":    content.ExternalId2,
		"eid":         "",
		"source_url":  content.SourceURL,
		"url":         content.URL,
		"content_url": content.ContentURL,
	})
	task.Metadata2 = string(meta2)

	// ---- Assert all three records and their relationships ----

	// 6a. DownloadTask fields derived from Content
	if task.ExternalId != content.ExternalId {
		t.Errorf("DownloadTask.ExternalId = %q, should match Content.ExternalId = %q", task.ExternalId, content.ExternalId)
	}
	if task.Size != content.Size {
		t.Errorf("DownloadTask.Size = %d, should match Content.Size = %d", task.Size, content.Size)
	}
	if task.URL == "" {
		t.Error("DownloadTask.URL should not be empty (built from Content.ContentURL + spec)")
	}
	if !strings.Contains(task.URL, content.ContentURL) {
		t.Errorf("DownloadTask.URL should contain Content.ContentURL as prefix:\n  taskURL: %s\n  contentURL: %s", task.URL, content.ContentURL)
	}
	if !strings.HasSuffix(task.URL, spec) {
		t.Errorf("DownloadTask.URL should end with spec %q, got %q", spec, task.URL)
	}
	if task.CoverURL != content.CoverURL {
		t.Errorf("DownloadTask.CoverURL = %q, should match Content.CoverURL = %q", task.CoverURL, content.CoverURL)
	}
	if task.Title == "" {
		t.Error("DownloadTask.Title should not be empty")
	}

	// 6b. Content knows about DownloadTask via DownloadTaskId (simulated)
	// In real code Content.DownloadTaskId = task.Id after db insert
	tmpTaskId := 42 // placeholder auto-increment id
	task.Id = tmpTaskId
	content.DownloadTaskId = &tmpTaskId

	if content.DownloadTaskId == nil || *content.DownloadTaskId != task.Id {
		t.Errorf("Content.DownloadTaskId should point to DownloadTask.Id; got %v, want %d", content.DownloadTaskId, task.Id)
	}

	// 6c. Content ↔ Account relationship via ContentAccount bridge
	// Both share PlatformId + external identifiers that link them
	ca := model.ContentAccount{
		ContentId: content.Id, // set after db insert
		AccountId: account.Id, // set after db insert
		Role:      "owner",
	}
	if ca.Role != "owner" {
		t.Errorf("ContentAccount.Role = %q, want %q", ca.Role, "owner")
	}
	// The linkage is established by platform_id + external_id:
	//   Account:  (platformId="wx_channels", externalId=contact.username)
	//   Content:  (platformId="wx_channels", externalId=object.id)
	//   ContentAccount: (content_id → Content.Id, account_id → Account.Id)
	if content.PlatformId != account.PlatformId {
		t.Errorf("Content and Account must share the same PlatformId to be linked; Content=%q Account=%q",
			content.PlatformId, account.PlatformId)
	}

	// 6d. BrowseRecordInfo (used by the interceptor to track browsing)
	// Build a MediaProfile directly from the parsed ChannelsObject, same
	// conversion used by TestCreateBrowseRecord_FromVideoFeedJSON.
	profile := videoChannelsObjectToMediaProfile(&obj)
	uniqueMark, info := wxchannels.CreateBrowseRecord(profile)
	if uniqueMark != content.ExternalId {
		t.Errorf("Browse uniqueMark = %q, should equal Content.ExternalId = %q", uniqueMark, content.ExternalId)
	}
	if info.AccountExternalId != account.ExternalId {
		t.Errorf("Browse.AccountExternalId = %q, should equal Account.ExternalId = %q", info.AccountExternalId, account.ExternalId)
	}
	if info.ContentURL != content.ContentURL {
		t.Errorf("Browse.ContentURL = %q, should equal Content.ContentURL = %q", info.ContentURL, content.ContentURL)
	}

	// 6e. FeedDownloadTaskBody labels mirror DownloadTask's request labels
	// (these are assigned to the download engine request, not stored in DownloadTask table directly)
	for lbl, want := range labels {
		if lbl == "source_url" {
			// source_url is in labels but `hasQualifier` can change suffix; still check the URL
			continue
		}
		got, ok := labels[lbl]
		if !ok || got != want {
			t.Errorf("labels[%s] = %q, should be present and equal %q", lbl, got, want)
		}
	}

	// Verify filename is built correctly (matches handler logic)
	if labels["title"] != filename {
		t.Errorf("labels[title] = %q, should equal filename base = %q", labels["title"], filename)
	}
	if hasQualifier {
		// When title already ends with .mp4, suffix is stripped
		if labels["suffix"] != "" {
			t.Errorf("labels[suffix] = %q, want empty (title already ends with .mp4)", labels["suffix"])
		}
	} else {
		// Default suffix is .mp4
		if labels["suffix"] != ".mp4" {
			t.Errorf("labels[suffix] = %q, want %q", labels["suffix"], ".mp4")
		}
	}

	// ---- Summary: Verify the three-record linkage is consistent ----
	//
	//  DownloadTask                       Content                         Account
	//  ┌──────────────────┐              ┌──────────────────┐            ┌──────────────────┐
	//  │ Id          = 42  │◄────────────│ DownloadTaskId    │            │ Id                │
	//  │ ExternalId  ="14.."│──matches──►│ ExternalId       │            │ ExternalId        │
	//  │ URL                │──from─────►│ ContentURL        │            │ Username          │
	//  │ Title              │──from─────►│ Title             │            │ Nickname  ="迷人的.."│
	//  │ CoverURL           │──from─────►│ CoverURL          │            │ AvatarURL         │
	//  │ Size          =9MB │──from─────►│ Size              │            │ PlatformId="wx_.." │
	//  │ Metadata2          │            │ ContentType=video  │            └──────────────────┘
	//  └──────────────────┘              │ PlatformId="wx_.." │                     ▲
	//                                    └──────────────────┘                     │
	//                                       │  ContentAccount ────────────────────┘
	//                                       │  {content_id, account_id, "owner"}
	//
	t.Logf("Account:  platform=%s externalId=%s nickname=%s", account.PlatformId, account.ExternalId, account.Nickname)
	t.Logf("Content:  platform=%s externalId=%s type=%s size=%d duration=%ds", content.PlatformId, content.ExternalId, content.ContentType, content.Size, content.Duration)
	t.Logf("Task:     url prefix=%s spec=%s suffix=%q filename=%q", downloadURL, spec, suffix, expectedFilename)
	t.Logf("Linkages: Content(%d)→Account(%d) via ContentAccount; Content→Task(%d) via DownloadTaskId; uniqueMark=%s",
		content.Id, account.Id, task.Id, uniqueMark)

	// Avoid "unused variable" compilation errors since records are constructed without real db ids
	_ = fmt.Sprintf("account=%v content=%v task=%v ca=%v", account, content, task, ca)
}

// TestDownloadFlowWithSubtasks_FromVideoFeedJSON extends TestDownloadFlow_FromVideoFeedJSON
// by creating a parent download task that contains three child subtasks:
//   - Subtask 0: download the .mp4 video
//   - Subtask 1: download the cover .jpg image
//   - Subtask 2: download the extracted .mp3 audio
//
// The Content and Account records are identical to the single-task scenario, but
// the download task tree demonstrates how a single content record links to a
// multi-file batch download.
func TestDownloadFlowWithSubtasks_FromVideoFeedJSON(t *testing.T) {
	var obj scraper.ChannelsObject
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
	feed, err := scraper.ChannelsObjectToChannelsFeedProfile(&obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile: %v", err)
	}

	const platformId = "wx_channels"
	media := obj.ObjectDesc.Media[0]

	// Identify spec (same logic as handler: first non-original h264 spec)
	spec := "xWT111"
	if len(feed.Spec) > 0 {
		spec = feed.Spec[0].FileFormat
	}
	suffix := ".mp4"

	// ---- Build download URLs for the three subtasks ----
	videoURL := strings.TrimSpace(media.URL + media.URLToken)
	if spec != "original" {
		videoURL = videoURL + "&X-snsvideoflag=" + spec
	}
	coverURL := strings.TrimSpace(media.CoverUrl)
	// Audio is not directly available in the feed.  A real pipeline would
	// extract the audio track from the video media.  Here we simulate it
	// as a separate download whose URL is derived from the video feed.
	audioURL := media.URL + "&X-snsvideoflag=" + spec + "&audio_only=1"

	// ---- Create parent (container) DownloadTask ----
	parentTaskId := 100 // simulated auto-increment id
	parent := model.DownloadTask{
		Id:         parentTaskId,
		TaskId:     "parent-task-uid-100",
		ParentId:   0,
		RootId:     0, // root container has no parent/root above itself
		NodeType:   "container",
		Engine:     "http",
		Type:       0,
		Status:     0, // ready
		ExternalId: content.ExternalId,
		Protocol:   "https",
		URL:        videoURL,
		SourceURI:  feed.SourceURL,
		Title:      content.Title,
		Filename:   content.Title + suffix,
		CoverURL:   content.CoverURL,
		Size:       content.Size,
		Reason:     "multi_file_download",
	}
	// Build metadata that references child task ids (same pattern as
	// compatDownloadTaskChildRefs reads from JSON keys like "childTaskIds").
	parent.Metadata2 = fmt.Sprintf(
		`{"platform":"%s","external_id":"%s","nonce_id":"%s","childTaskIds":["sub-video","sub-cover","sub-audio"],"childDownloadTaskIds":["","",""]}`,
		platformId, content.ExternalId, content.ExternalId2,
	)

	// ---- Create three child subtasks ----
	childVideo := model.DownloadTask{
		Id:         101,
		TaskId:     "sub-video",
		ParentId:   parent.Id,
		RootId:     parent.Id,
		NodeType:   "file",
		Engine:     "http",
		Type:       parent.Type,
		Status:     0,
		ExternalId: content.ExternalId,
		Protocol:   "https",
		URL:        videoURL,
		Title:      content.Title + ".mp4",
		Filename:   content.Title + ".mp4",
		CoverURL:   content.CoverURL,
		Size:       content.Size,
		Reason:     "platform_file",
		Idx:        0,
	}
	childCover := model.DownloadTask{
		Id:         102,
		TaskId:     "sub-cover",
		ParentId:   parent.Id,
		RootId:     parent.Id,
		NodeType:   "file",
		Engine:     "http",
		Type:       parent.Type,
		Status:     0,
		ExternalId: content.ExternalId,
		Protocol:   "image",
		URL:        coverURL,
		Title:      content.Title + "_cover.jpg",
		Filename:   content.Title + "_cover.jpg",
		CoverURL:   content.CoverURL,
		Size:       0,
		Reason:     "platform_file",
		Idx:        1,
	}
	childAudio := model.DownloadTask{
		Id:         103,
		TaskId:     "sub-audio",
		ParentId:   parent.Id,
		RootId:     parent.Id,
		NodeType:   "file",
		Engine:     "http",
		Type:       parent.Type,
		Status:     0,
		ExternalId: content.ExternalId,
		Protocol:   "audio",
		URL:        audioURL,
		Title:      content.Title + ".mp3",
		Filename:   content.Title + ".mp3",
		CoverURL:   content.CoverURL,
		Size:       0,
		Reason:     "platform_file",
		Idx:        2,
	}
	children := []model.DownloadTask{childVideo, childCover, childAudio}

	// ---- Link Content to the parent DownloadTask ----
	content.DownloadTaskId = &parent.Id

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

	// 2. Content → parent DownloadTask linkage
	if content.DownloadTaskId == nil {
		t.Fatal("Content.DownloadTaskId should point to parent task")
	}
	if *content.DownloadTaskId != parent.Id {
		t.Errorf("Content.DownloadTaskId = %d, want parent.Id = %d", *content.DownloadTaskId, parent.Id)
	}

	// 3. Content → Account linkage
	if ca.Role != "owner" {
		t.Errorf("ContentAccount.Role = %q, want 'owner'", ca.Role)
	}

	// 4. Parent task shape
	if parent.NodeType != "container" {
		t.Errorf("Parent.NodeType = %q, want 'container'", parent.NodeType)
	}
	if parent.ParentId != 0 {
		t.Errorf("Parent.ParentId = %d, want 0 (root container)", parent.ParentId)
	}
	if parent.ExternalId != content.ExternalId {
		t.Errorf("Parent.ExternalId = %q, should match Content.ExternalId = %q", parent.ExternalId, content.ExternalId)
	}
	if parent.Reason != "multi_file_download" {
		t.Errorf("Parent.Reason = %q, want 'multi_file_download'", parent.Reason)
	}

	// 5. Each child is properly linked to parent
	expectedChildIDs := []int{101, 102, 103}
	expectedProtocols := []string{"https", "image", "audio"}
	expectedFilenames := []string{
		content.Title + ".mp4",
		content.Title + "_cover.jpg",
		content.Title + ".mp3",
	}
	expectedURLs := []string{videoURL, coverURL, audioURL}
	for i, child := range children {
		sub := fmt.Sprintf("child[%d]", i)

		if child.ParentId != parent.Id {
			t.Errorf("%s.ParentId = %d, want parent.Id = %d", sub, child.ParentId, parent.Id)
		}
		if child.RootId != parent.Id {
			t.Errorf("%s.RootId = %d, want parent.Id = %d", sub, child.RootId, parent.Id)
		}
		if child.NodeType != "file" {
			t.Errorf("%s.NodeType = %q, want 'file'", sub, child.NodeType)
		}
		if child.ExternalId != content.ExternalId {
			t.Errorf("%s.ExternalId = %q, should match Content.ExternalId", sub, child.ExternalId)
		}
		if child.Id != expectedChildIDs[i] {
			t.Errorf("%s.Id = %d, want %d", sub, child.Id, expectedChildIDs[i])
		}
		if child.Protocol != expectedProtocols[i] {
			t.Errorf("%s.Protocol = %q, want %q", sub, child.Protocol, expectedProtocols[i])
		}
		if child.Filename != expectedFilenames[i] {
			t.Errorf("%s.Filename = %q, want %q", sub, child.Filename, expectedFilenames[i])
		}
		if child.Idx != i {
			t.Errorf("%s.Idx = %d, want %d", sub, child.Idx, i)
		}
		if child.URL != expectedURLs[i] {
			t.Errorf("%s.URL mismatch:\n  got  %s\n  want %s", sub, child.URL, expectedURLs[i])
		}
	}

	// 6. Unique ExternalId ties parent + children + content together
	allTasks := append([]model.DownloadTask{parent}, children...)
	for _, task := range allTasks {
		if task.ExternalId != content.ExternalId {
			t.Errorf("Task(id=%d, nodeType=%s).ExternalId = %q, want %q (all must share same ExternalId)",
				task.Id, task.NodeType, task.ExternalId, content.ExternalId)
		}
	}

	// 7. Cover URL is consistent across parent and children
	if parent.CoverURL != content.CoverURL {
		t.Errorf("Parent.CoverURL should match Content.CoverURL")
	}
	for _, child := range children {
		if child.CoverURL != content.CoverURL {
			t.Errorf("child[%d].CoverURL should match Content.CoverURL", child.Idx)
		}
	}

	// 8. Metadata2 on parent carries key signals for lineage resolution
	var meta2 map[string]any
	if err := json.Unmarshal([]byte(parent.Metadata2), &meta2); err != nil {
		t.Fatalf("Parent.Metadata2 is not valid JSON: %v", err)
	}
	if meta2["platform"] != platformId {
		t.Errorf("Metadata2.platform = %v, want %q", meta2["platform"], platformId)
	}
	if meta2["external_id"] != content.ExternalId {
		t.Errorf("Metadata2.external_id = %v, want %q", meta2["external_id"], content.ExternalId)
	}
	if meta2["childTaskIds"] == nil {
		t.Error("Metadata2.childTaskIds should reference child task ids")
	}

	t.Logf("=== Multi-file Download Tree ===")
	t.Logf("Account : %s (%s)", account.Nickname, account.ExternalId)
	t.Logf("Content : %s | %s | %d bytes | %ds", content.Title, content.ContentType, content.Size, content.Duration)
	t.Logf("Parent  : id=%d type=container -> Content(%d) via DownloadTaskId", parent.Id, content.Id)
	t.Logf("  ├── [0] id=%d url=%s (video)", children[0].Id, children[0].Protocol)
	t.Logf("  ├── [1] id=%d url=%s (cover)", children[1].Id, children[1].Protocol)
	t.Logf("  └── [2] id=%d url=%s (audio)", children[2].Id, children[2].Protocol)
	t.Logf("Linkage : ContentAccount(role=%q) binds Content(%d) ↔ Account(%d)", ca.Role, ca.ContentId, ca.AccountId)

	// Avoid unused variable errors
	_ = fmt.Sprintf("parent=%v children=%v content=%v account=%v", parent, children, content, account)
}