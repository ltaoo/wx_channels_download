package officialaccount_test

import (
	"encoding/json"
	"testing"

	officialaccount "wx_channel/pkg/scraper/officialaccount"
)

const channelsFeedJSON = `{
	"base_resp": {
		"ret": 0,
		"errmsg": "ok",
		"wxtoken": 777,
		"cookie_count": 0,
		"sessionid": "svr_645c824c941"
	},
	"user_name": "gh_cae83b625e1e",
	"nick_name": "小众软件",
	"round_head_img": "http://mmbiz.qpic.cn/mmbiz_png/4icbjwFqP3Mf2vGs3YNFia9N6eXMnwTj9Jwmmt4M6HWVQDXcCiaw5ZxODekciacMMy6vXW0heZ7xibq5F97lOFHvWyw/0?wx_fmt=png",
	"title": "在AppleTV上观看B站视频",
	"desc": "搜索 zilazila 安装就行了。",
	"content_noencode": "<section class=\"channels_iframe_wrp\" nodeleaf=\"\"><mp-common-videosnap class=\"js_uneditable custom_select_card channels_iframe videosnap_video_iframe videosnap_video_iframe\" data-pluginname=\"mpvideosnap\" data-url=\"https://findermp.video.qq.com/251/20304/stodownload?encfilekey=KGDRibp2wkicIG6j8mSvep0GYxhNf07uvZr1X1JBqcicWeDO6JvFMIu0cFrUVjFtDic2ytBjClg2WMh7JW8aByyByA&amp;token=x5Y29zUxcibCRDhRog4BCIY5WVFndHKdaWM0cdyTqxePDtT8hPeEy7fG3nlqyKKAsfEmIniczAKRs26ZuCJr0MjGpLuiasnSjWgNtNjjqEAJunhFg0KvGrxez9QUgXqrqynVQuD6l5kAQKMAL1mnO6R2MQmW1OgwGvnN5SdvbibLjRhpvTQzUTskeZsR616cRM9flVRgC18ibFnX67P3icePWx2ddDG4shCZR4ZYZ1oibgaY7EsZMicH8Up7Tpy1CD0FhII5Kt43hVnHSItvXibXXiaR47WiawbjV0MM6ruXXPMwiaZ61VE&amp;hy=SZ&amp;idx=1&amp;m=&amp;scene=2&amp;uzid=1&amp;wxampicformat=503&amp;picformat=200\" data-headimgurl=\"https://wx.qlogo.cn/finderhead/0CXibl8UPEEXYKMHQrxd4rhogCBTkibbkgC27Z8X38ib0DT15pfA4M06EEUJ2I3bpcPAEO1hnJZibLI/0\" data-username=\"v2_060000231003b20faec8c6eb8d1fc3d0ca0de531b077f8f4e809f1954064ebe13d9408c8cc5c@finder\" data-nickname=\"小众软件\" data-desc=\"在AppleTV 上看B站视频，搜索 zilazila 即可。代码 zilazila://\" data-nonceid=\"4489759665885018130\" data-width=\"1080\" data-height=\"1440\" data-type=\"video\" data-id=\"export/UzFfBgAAxOuhGDhMQjmrjMzT4DCL6ZgEH23Ib8ZIiYBtSWsFzw\"></mp-common-videosnap></section><section><span leaf=\"\">搜索 zilazila 安装就行了。</span></section><section><span leaf=\"\">smb 中输入&nbsp;</span><span leaf=\"\" data-pm-slice=\"1 1 [&quot;para&quot;,null]\">zilazila://</span></section><section><span leaf=\"\">搞定</span></section><p style=\"display: none;\"><mp-style-type data-value=\"3\"></mp-style-type></p>",
	"create_time": "2026-07-07 13:57",
	"cdn_url": "https://mmbiz.qpic.cn/sz_mmbiz_jpg/Ts4Edaib9xxfZj42Ctf0pEhS5tux9LxZjwoWv8KQ1cjLXKUXKxyyTLickbQMob2HSFV9qRzOnic2B6AJ2s397RmfBiaDNLOcS7Iwe3QR3SosZmM/0?wx_fmt=jpeg",
	"link": "https://mp.weixin.qq.com/s?__biz=MjM5NDMwMTI2MA==&amp;mid=2651699745&amp;idx=1&amp;sn=cbf884754eb6df62f29ecc0d5caa2653&amp;chksm=bca1f47383c09dac514fd38ba1c5fbf6cd51143ebf5dae56ea92d8bb62844b16abb7e8d2dd9e#rd",
	"source_url": "",
	"can_share": 1,
	"alias": "appinncom",
	"type": 10002,
	"author": "青小蛙",
	"is_limit_user": 0,
	"show_cover_pic": 0,
	"advertisement_num": 0,
	"advertisement_info": [],
	"ori_create_time": 1783403836,
	"user_uin": "2993894182",
	"total_item_num": 1,
	"is_async": 1,
	"comment_id": "4593795753399140352",
	"img_format": "jpeg",
	"svr_time": 1783694231,
	"copyright_info": {
		"copyright_stat": 1,
		"ori_article_type": "",
		"is_cartoon_copyright": 0
	},
	"can_reward": 1,
	"signature": "推荐有灵魂的电脑软件、手机应用。",
	"reward_wording": "",
	"in_mm": 1,
	"app_id": "wx7619b725211d3612",
	"show_comment": 0,
	"can_use_page": 0,
	"hd_head_img": "http://wx.qlogo.cn/mmhead/Q3auHgzwzM5oB1jblgiarD8aOiaYBeEPnOE9DyicmmBJXIdUHFNC0h8MA/0",
	"del_reason_id": 0,
	"srcid": "0710YuOmv1vTm7BjHRTxQs2z",
	"is_wxg_stuff_uin": 0,
	"need_report_cost": 0,
	"use_tx_video_player": 0,
	"is_only_read": 1,
	"req_id": "1022vGE2JVLr14s6DJxjFycC",
	"use_outer_link": 0,
	"ban_scene": 0,
	"csp_nonce_str": 595054426,
	"msg_daily_idx": 0,
	"ori_head_img_url": "http://wx.qlogo.cn/mmhead/Q3auHgzwzM5oB1jblgiarD8aOiaYBeEPnOE9DyicmmBJXIdUHFNC0h8MA/132",
	"filter_time": 1783403814,
	"appmsg_fe_filter": "contenteditable",
	"is_login": 1,
	"reward_money": 0,
	"item_show_type": 0,
	"voice_in_appmsg": [],
	"video_page_info": {
		"mp_video_trans_info": [],
		"drama_video_info": {},
		"drama_info": {}
	},
	"malicious_title_reason_id": 0,
	"picture_page_info_list": [],
	"show_msg_voice": 0,
	"reward_author_head": "https://mmbiz.qlogo.cn/mmbiz_jpg/0CXibl8UPEEWfp12n1TdXx41Bch3QXibsjEw2r1P4Dxh26WOBPyAr0eWDHtV4mPDeWE0gfp8eSgSx2UGH6BYic0Tg/0?wx_fmt=jpeg",
	"locationlist": [],
	"hotspotinfolist": [],
	"author_id": "ofMoI4zqkPAtvPwuio3aD_19alGc",
	"isnew": 0,
	"malicious_content_type": 0,
	"fasttmpl_version": 8339498,
	"is_top_stories": 0,
	"video_ids": [],
	"isprofileblock": 0,
	"cdn_url_235_1": "https://mmbiz.qpic.cn/sz_mmbiz_jpg/Ts4Edaib9xxfZj42Ctf0pEhS5tux9LxZjwoWv8KQ1cjLXKUXKxyyTLickbQMob2HSFV9qRzOnic2B6AJ2s397RmfBiaDNLOcS7Iwe3QR3SosZmM/0?wx_fmt=jpeg",
	"cdn_url_1_1": "https://mmbiz.qpic.cn/sz_mmbiz_jpg/Ts4Edaib9xxeOcbnjXwIdRExt41mMMKrjzS8ZicHKWSf52j546G7UpneZl7TcXcDHXpu5mwxbib5B1LqjIfmibtw6jwCDetxQHnKBl4SjMuMc3A/0?wx_fmt=jpeg",
	"more_read_type": 0,
	"appmsg_like_type": 2,
	"ori_send_time": 1783403836,
	"show_top_bar": 0,
	"related_tag": [],
	"user_info": {
		"show_top_bar": 0,
		"enter_id": 1458771373,
		"user_uin": "2993894182",
		"is_login": 1,
		"is_wxg_stuff_uin": 0,
		"show_comment": 0,
		"can_share": 1,
		"can_use_page": 0,
		"is_paid": 0,
		"clientversion": "f2641b17",
		"ckeys": [],
		"show_msg_voice": 0,
		"fasttmpl_infos": [
			{
				"type": 0,
				"version": 8339498,
				"lang": "zh_CN",
				"fullversion": "8339498-zh_CN-html",
				"versiongroup": "zh_CN-html"
			}
		],
		"isoversea": 0,
		"search_keyword": {
			"item_list": [
				{
					"keyword": "zilazila",
					"idx_range_list": [
						{
							"begin_idx": 3,
							"end_idx": 10,
							"section_idx": 1
						},
						{
							"begin_idx": 8,
							"end_idx": 15,
							"section_idx": 2
						}
					],
					"s1s_stat_info": "%7B%22bizuin%22%3A2394301260%2C%22msgid%22%3A2651699745%2C%22msgidx%22%3A1%2C%22docid%22%3A%224671258382432945383%22%2C%22keywordItem%22%3A%7B%22keyword%22%3A%22zilazila%22%2C%22section_idx%22%3A1%2C%22begin_idx%22%3A3%2C%22end_idx%22%3A10%2C%22type%22%3A1024%2C%22lemma_id%22%3A%22%22%7D%2C%22category%22%3A%22%E7%A7%91%E6%8A%80_%E8%BD%AF%E4%BB%B6%E5%B7%A5%E5%85%B7%3A0.975083%22%2C%22reqId%22%3A9824816780181874247%2C%22S1SPageType%22%3A1%2C%22strReqId%22%3A%229824816780181874247%22%2C%22orgReqId%22%3A%221185578384754959938%22%2C%22item_show_type%22%3A0%2C%22common_value_expt%22%3A91%2C%22highlight_preload%22%3A0%7D",
					"s1s_context_info": "%7B%22keyword%22%3A%22zilazila%22%2C%22isNeedUpdateGPTInfo%22%3Afalse%2C%22S1SPageType%22%3A1%2C%22search_id%22%3A%221185578384754959938%22%2C%22doc_info%22%3A%7B%22triple%22%3A%7B%22bizuin%22%3A2394301260%2C%22msgid%22%3A2651699745%2C%22msgidx%22%3A1%7D%2C%22docid%22%3A4671258382432944128%2C%22publish_time%22%3A1783403836%7D%2C%22idx_range%22%3A%7B%22section_idx%22%3A1%2C%22begin_idx%22%3A3%2C%22end_idx%22%3A10%7D%2C%22expt_value%22%3A91%2C%22source%22%3A1024%2C%22needPreRender%22%3Afalse%7D",
					"s1s_jsapi_name": "openWXSearchHalfPage",
					"s1s_jsapi_paras": "{\"query\":\"zilazila\",\"scene\":139,\"hiddenSearchHeader\":0,\"webviewHeightRatio\":0.699999988,\"kvItems\":[{\"key\":\"mpEndHalfPageResultTab\",\"textValue\":\"0\"},{\"key\":\"firstSearchRequest\",\"uintValue\":1},{\"key\":\"MPHalfSearchAIBox\",\"uintValue\":3}],\"sessionKvItems\":[{\"key\":\"mpEndHalfPageResultTab\",\"textValue\":\"0\"},{\"key\":\"MPHalfSearchAIBox\",\"uintValue\":3}],\"parentType\":135,\"isAutoShowUnitInHalfScreen\":1}",
					"tags": []
				}
			],
			"exp_info": "CMzG2PUIEKH0tvAJGAEiEzQ2NzEyNTgzODI0MzI5NDUzODMowszN5aaogboQ",
			"need_baike_preload": true,
			"show_ad_keyword": false,
			"ad_item_list": []
		},
		"tts_heard_person_cnt": 1,
		"tts_is_show": 1,
		"frontend_exp": {
			"list": [
				{
					"key": "appmsg_loveline",
					"value": "1"
				},
				{
					"key": "appmsg_underline_translate",
					"value": "1"
				},
				{
					"key": "comment_poi",
					"value": "1"
				},
				{
					"key": "tts_floating",
					"value": "1"
				}
			],
			"succ": true
		},
		"show_version": 2,
		"bar_version": 5,
		"transfer_config": [
			{
				"scope": "mmbizwap_cgi_appmsgad",
				"cgis": [
					"mp/advertisement_report",
					"mp/getappmsgad",
					"mp/ad_video_report",
					"mp/ad_monitor",
					"mp/ad_report",
					"mp/ad_biz_info",
					"mp/ad_complaint",
					"mp/ad",
					"mp/ad_app_info"
				]
			},
			{
				"scope": "mmbizwap_cgi_appmsgext",
				"cgis": [
					"mp/appmsg_comment",
					"mp/getappmsgext",
					"mp/videoplayer",
					"mp/appmsg_video_snap",
					"mp/immersive_player",
					"mp/appmsg_weapp",
					"mp/appmsg_like",
					"mp/newappmsgvote",
					"mp/reward",
					"mp/authorreward",
					"mp/qqmusic",
					"mp/video",
					"mp/qna",
					"mp/searchwordbaike",
					"mp/appmsgthank",
					"mp/creationcenter"
				]
			},
			{
				"scope": "mmbizwap_cgi_misc",
				"cgis": [
					"mp/wapcommreport",
					"mp/underline",
					"mp/relatedarticle",
					"mp/homepage",
					"mp/waerrpage",
					"mp/getverifyinfo",
					"mp/getprofilebizrecommend",
					"mp/infringement",
					"mp/getprofiletransferpage",
					"mp/wacomplain",
					"mp/appmsgreport",
					"mp/getbizbanner"
				]
			}
		],
		"bar_data": 1,
		"appmsg_bar_data": {
			"show_like": 1,
			"show_old_like": 1,
			"show_share": 1,
			"show_like_gray": 0,
			"show_old_like_gray": 0,
			"show_share_gray": 0,
			"comment_enabled": 1,
			"like_count": 6,
			"old_like_count": 41,
			"share_count": 89,
			"comment_count": 39,
			"get_data_succ": 1,
			"collect_count": 45,
			"show_collect": 1,
			"show_collect_gray": 0,
			"read_num": 2806,
			"show_read": 1,
			"show_friend_seen": 2,
			"friend_subscribe_count": 0,
			"is_subscribed": 1,
			"star_num": 17,
			"show_star": 1,
			"can_use_star": 0,
			"original_content_num": 0,
			"verify_status": 0,
			"friend_seen_info": {
				"friend_seen_count": 0,
				"friend_info": [],
				"seen_status": 0,
				"friendlovenum": 0
			}
		},
		"pic_related_rec_info": {},
		"line_info": {
			"use_line": 1,
			"ui_version": 1
		},
		"show_comment_bar": 0,
		"reward_total_count": 1,
		"subcount_version": 3,
		"reward_half_panel": 1,
		"reward_info": {
			"timestamp": 1783694231,
			"rewardsn": "61ca4d8336c217dbea23"
		},
		"listen_player_info": {
			"recommend_info_buffer": "IoQBeyJtcF9zY2VuZSI6MCwibXBfc3ViX3NjZW5lIjowLCJfX2JpeiI6Ik1qTTVORE13TVRJMk1BPT0iLCJhcHBtc2dfaWQiOjI2NTE2OTk3NDUsIml0ZW1faWR4IjoxLCJtcF9nZXRfYThrZXlfc2NlbmUiOjEsInRyYWNlX2ZsYWciOjB9KhwQCRoYCg4IzMbY9QgQofS28AkYARAAGAAgASgA"
		},
		"show_comment_entrance": 2,
		"share_h5info": {
			"underline_url": "https://mp.weixin.qq.com/mp/underline?action=get_appmsg_segment&clicktype=2&show_comment=1#wechat_redirect",
			"interaction_url": "https://mp.weixin.qq.com/mp/getinteraction?action=get_usertypelist&type=0&get_save=1&get_lastread=1&clicktype=2#wechat_redirect"
		},
		"short_link": "https://mp.weixin.qq.com/s/0a_r7kMkJ3iaJMd4YiQxJw",
		"quote_list": [],
		"red_flower_like_info": {
			"is_red_flower_like": 0
		},
		"indentity_id": "zSoOXxKy75HnZGbfs2lZfA",
		"get_search_keyword_realtime": 0,
		"show_friend_love_info": 1,
		"support_view_photo_acct": 1,
		"show_ai_live": 0,
		"pictext_photo_rename": 1,
		"is_latest_patch": 0,
		"comment_activity_id": 0,
		"show_comment_use_photo_tips": false,
		"support_view_photo_profileext": true,
		"version_comment_can_use_photoaccount": false,
		"comment_activity_start_time": 0,
		"comment_activity_end_time": 0
	},
	"ainfos": [],
	"related_article_info": {
		"has_related_article_info": 0
	},
	"has_red_packet_cover": 0,
	"is_pay_subscribe": 0,
	"pay_subscribe_info": {
		"preview_percent": 0,
		"desc": "",
		"fee": 0,
		"gifts_count": 0,
		"wecoin_amount": 0
	},
	"video_in_article": [],
	"appmsgalbuminfo": {
		"album_id": "1329672091876261890",
		"title": "苹果应用推荐",
		"link": "https://mp.weixin.qq.com/mp/appmsgalbum?__biz=MjM5NDMwMTI2MA==&amp;action=getalbum&amp;album_id=1329672091876261890#wechat_redirect",
		"isupdating": 1,
		"content_size": 434,
		"fee": 0,
		"album_needpay": 0,
		"appmsg_needpay": 0,
		"type": 0,
		"continous_read_on": 1,
		"article_titles": [],
		"pre_article_link": "http://mp.weixin.qq.com/s?__biz=MjM5NDMwMTI2MA==&amp;mid=2651699376&amp;idx=1&amp;sn=b9ee33bf586c1d63ec3e090c9185f8e1&amp;chksm=bd70cc938a07458547ef95d924ce18e287c5b26f9036603718f24f93b5531619bac7d3787041#wechat_redirect",
		"next_article_link": "http://mp.weixin.qq.com/s?__biz=MjM5NDMwMTI2MA==&amp;mid=2651699889&amp;idx=2&amp;sn=ac518eaa6ea95aefd0417e26db03e8c7&amp;chksm=bd70ce928a0747844aff6af9462553f818e9e22f434ea9c9bf4e3ae3a9731a315396d497aec8#wechat_redirect",
		"pre_article_title": "在 macOS 菜单栏上实时监控电量、功耗",
		"next_article_title": "iOS捷径支持微信、钉钉、飞书发送消息了",
		"album_id_str": "1329672091876261890",
		"is_wxa_novel": false,
		"category_playlist_info_base64": "ChttcGFsYnVtLTEzMjk2NzIwOTE4NzYyNjE4OTAQEBgCIgblkIjpm4ZIAg=="
	},
	"is_area_shield": 0,
	"shield_areaids": [],
	"appmsg_ext_get": {
		"func_flag": 0
	},
	"anchor_tree": [],
	"voice_in_appmsg_list_json": "{\"voice_in_appmsg\":[]}",
	"public_tag_info": {
		"tags": [
			{
				"tag_name": "苹果应用推荐",
				"tag_link": "https://mp.weixin.qq.com/mp/appmsgalbum?__biz=MjM5NDMwMTI2MA==&amp;action=getalbum&amp;album_id=1329672091876261890#wechat_redirect",
				"tag_content_num": 434,
				"album_id": "1329672091876261890",
				"album_info": {
					"album_id": "1329672091876261890",
					"title": "苹果应用推荐",
					"link": "https://mp.weixin.qq.com/mp/appmsgalbum?__biz=MjM5NDMwMTI2MA==&amp;action=getalbum&amp;album_id=1329672091876261890#wechat_redirect",
					"isupdating": 1,
					"content_size": 434,
					"fee": 0,
					"album_needpay": 0,
					"appmsg_needpay": 0,
					"type": 0,
					"continous_read_on": 1,
					"article_titles": [],
					"album_id_str": "1329672091876261890"
				}
			}
		]
	},
	"video_snap_card": "{\"list\":[{\"username\":\"v2_060000231003b20faec8c6eb8d1fc3d0ca0de531b077f8f4e809f1954064ebe13d9408c8cc5c@finder\",\"export_id\":\"export/UzFfBgAAxOuhGDhMQjmrjMzT4DCL6ZgEH23Ib8ZIiYBtSWsFzw\",\"notice_id\":\"\",\"event_id\":\"\",\"listen_id\":\"\"}]}",
	"live_info": [],
	"lang": "en",
	"cdn_url_16_9": "",
	"real_item_show_type": 0,
	"url_item_show_type": 0,
	"video_page_infos": [],
	"can_use_wecoin": 1,
	"wecoin_tips": 0,
	"front_end_additional_fields": {
		"is_auto_type_setting": 3,
		"save_type": 0,
		"template_version": "35185127"
	},
	"open_fansmsg": 0,
	"is_cooling_appmsg": 0,
	"ip_wording": {
		"country_name": "中国",
		"country_id": "156",
		"province_name": "四川"
	},
	"show_ip_wording": 1,
	"is_acct_area_shield": 0,
	"shield_acct_areaids": [],
	"style_type": 3,
	"shield_areas_info": [],
	"create_timestamp": 1783403836,
	"picture_list_in_pictext": [],
	"servicetype": 0,
	"segment_comment_id": "4593795765730394115",
	"ad_mark_status": 0,
	"hide_ad_mark_on_cps": 0,
	"finder_audio_card": "{\"list\":[]}",
	"claim_source": {
		"is_user_no_claim_source": 1
	},
	"extra_comment_id": "4593795765042528259",
	"last_text": [],
	"wash_status": 0,
	"enterid": 1783694231,
	"zhuge_qa_id_list": [],
	"sec_control_info": {
		"list": []
	},
	"cdn_url_3_4": "",
	"window_product_list": [],
	"finder_music_card": "{\"list\":[]}",
	"finder_audio_card_list": {
		"list": []
	},
	"finder_music_card_list": {
		"list": []
	},
	"new_service_type": 0,
	"product_activity": {},
	"rt_biz_info": {},
	"redpacket_cover_list": [],
	"footer_gift_activity": {},
	"verify_status": 0,
	"is_phacct_verify": 0,
	"watermark_setting": 2,
	"title_gen_type": 0,
	"appmsg_listen_id": "150444993240070143",
	"trans_appmsg_info": {},
	"location": {},
	"topic_infos": [],
	"footer_common_shops": [],
	"footer_product_card": {},
	"desc_empty": false,
	"hashtags": {
		"hashtag": []
	},
	"aigc_pictures": [],
	"private_info": {},
	"biz_type": 1,
	"ai_chat_info": {
		"ai_chat_status": 0,
		"room_info": ""
	},
	"special_biz": false,
	"preload_comment_item_list": []
}`

func TestArticleToProfile_FromChannelsFeedJSON(t *testing.T) {
	var pageJSON officialaccount.CgiDataNew
	if err := json.Unmarshal([]byte(channelsFeedJSON), &pageJSON); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	article := &officialaccount.WechatOfficialArticle{
		PageJSON: &pageJSON,
	}
	profile, err := officialaccount.ArticleToProfile(article, pageJSON.Link)
	if err != nil {
		t.Fatalf("ArticleToProfile: %v", err)
	}

	if profile.ArticleID == "" {
		t.Error("ArticleID should not be empty")
	}
	if profile.Title != "在AppleTV上观看B站视频" {
		t.Errorf("Title = %q, want %q", profile.Title, "在AppleTV上观看B站视频")
	}
	if profile.Description != "搜索 zilazila 安装就行了。" {
		t.Errorf("Description = %q", profile.Description)
	}
	if profile.CoverURL == "" {
		t.Error("CoverURL should not be empty")
	}
	if profile.PublishTime != 1783403836 {
		t.Errorf("PublishTime = %d, want 1783403836", profile.PublishTime)
	}
	if profile.Author.ExternalId != "gh_cae83b625e1e" {
		t.Errorf("Author.ExternalId = %q, want %q", profile.Author.ExternalId, "gh_cae83b625e1e")
	}
	if profile.Author.Nickname != "小众软件" {
		t.Errorf("Author.Nickname = %q, want %q", profile.Author.Nickname, "小众软件")
	}
	if profile.Author.AvatarURL == "" {
		t.Error("Author.AvatarURL should not be empty")
	}
}
