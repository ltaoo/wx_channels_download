package wxchannels_test

import (
	"encoding/json"
	"testing"

	wxchannels "wx_channel/pkg/scraper/wxchannels"
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
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	account, err := obj.ToAccount()
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
	if account.Id != "wx_channels:"+expectedUsername {
		t.Errorf("Id = %q", account.Id)
	}
}

func TestToContent_FromVideoFeedJSON(t *testing.T) {
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal([]byte(videoFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	content, err := obj.ToContent()
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
	if content.Id != "wx_channels:14962486294771997060" {
		t.Errorf("Id = %q", content.Id)
	}
	expectedContentURL := "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&token=2lt8WBSnjTkTjXXRcWF576SLtqb9LdRn1Cliaa0icf5zFjCLyBFNe1e3eKzhzzEc5h05O81ibb3hwbVTVywYQAQbSQzZkHicCqabpEdwBzhTgdyPiakaMMw7n96CtNxoPbKkQxiaYOzPImgS9ZG3kDzKcLjMEyIIVGYuibzdHECVIOFibOQGL4pWibDRRD6VcpGApwhugo6k9Mq48YAov7zg751dO260H5iaGeEkJZWhKhib0hib4W0&basedata=CAMSBnhXVDE1OCJaCgoKBnhXVDE1OBAACgoKBnhXVDE1NxAACgoKBnhXVDE1NhAACgoKBnhXVDExMxAACgoKBnhXVDExMhAACgoKBnhXVDExMRAACgcKA3hBMBAACgcKA3hBMhAA&sign=AgZzkYT5vBvSWwKe5MpufA75x2T3Xnnz7PtuTK98WxdVbZm4Grpnyl52sDN4W6CI562FVgGaZ-_tYlBjCRLdIQ&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAABRfl4aFfX8vo5XJgBRahAAAADnaHZTnGbFfAj9RgZXfw6ViUCWOt8LYujr%2BrkpCHNy7PD375%2BDqLzGDCk8ibQxWRl9tKOjUKAhiL4%3D&svrnonce=1783693350"
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
