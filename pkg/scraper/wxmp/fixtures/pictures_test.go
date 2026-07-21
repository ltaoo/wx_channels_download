package wxmp_test

import (
	"encoding/json"
	"testing"

	wxmp "wx_channel/pkg/scraper/wxmp"
)

const picturesJSON = `{
	"base_resp": {
		"ret": 0,
		"errmsg": "ok",
		"wxtoken": 777,
		"cookie_count": 0,
		"sessionid": "svr_6e389ae0e64"
	},
	"user_name": "gh_8951dcd584fe",
	"nick_name": "日照茶人茶事",
	"round_head_img": "http://mmbiz.qpic.cn/mmbiz_png/SZnrAVfCkic3OFY6yqkdN4ib5KAjdva8YPo2T7bjN6EMDdopCEZ158KK6sD4lft9Yd7LUVWicYcIdquH7d3HdCI1g/0?wx_fmt=png",
	"title": "凤求凰",
	"desc": "元周子\n\n一生一世凤求凰，\n风凰广场佳偶双。\n琴瑟和鸣金银雀，\n船儿思我太疯狂。",
	"content_noencode": "元周子\n\n一生一世凤求凰，\n风凰广场佳偶双。\n琴瑟和鸣金银雀，\n船儿思我太疯狂。",
	"create_time": "2026-02-16 21:04",
	"cdn_url": "http://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicWjQnj0hnwpyoe2ml30P0lafSlnqItOicjTxE6fBtK37Ty9icPnKltBxmWZX3xG1sGR4y8icWKovf5BXOYrTtPOhPgH4KUvGmXRiaU/0?wx_fmt=jpeg",
	"link": "https://mp.weixin.qq.com/s?__biz=Mzg3MDYyMTIyNQ==&amp;mid=2247484047&amp;idx=1&amp;sn=ed2f165e36707b45e4fdc01ad959d3d2&amp;chksm=cfdc4758d7379716ba6ddcc796b21ceab2eff685d2512d097e37ddff7f2c31339581d906b1a3#rd",
	"source_url": "",
	"can_share": 1,
	"alias": "SDXOX888",
	"type": 10002,
	"author": "",
	"is_limit_user": 0,
	"show_cover_pic": 0,
	"advertisement_info": [],
	"ori_create_time": 1771247066,
	"user_uin": "2993894182",
	"total_item_num": 1,
	"is_async": 1,
	"comment_id": "4389839024140500996",
	"img_format": "jpeg",
	"svr_time": 1783694316,
	"copyright_info": {
		"copyright_stat": 0,
		"is_cartoon_copyright": 0
	},
	"can_reward": 0,
	"signature": "山东新偶像1971",
	"in_mm": 1,
	"app_id": "wx9b2213eeb204a525",
	"show_comment": 0,
	"can_use_page": 0,
	"hd_head_img": "http://wx.qlogo.cn/mmhead/Q3auHgzwzM49RiauUE7DAosQMdBytfTYfcO4mJOulGImjq9SMeBWyIg/0",
	"del_reason_id": 0,
	"srcid": "0710E8pSrJ5L1rD6nM8mcqsg",
	"is_wxg_stuff_uin": 0,
	"need_report_cost": 0,
	"use_tx_video_player": 0,
	"is_only_read": 1,
	"req_id": "1022cfcXZV43Dy09zVrfCX7r",
	"use_outer_link": 0,
	"ban_scene": 0,
	"csp_nonce_str": 188567504,
	"msg_daily_idx": 0,
	"ori_head_img_url": "http://wx.qlogo.cn/mmhead/Q3auHgzwzM49RiauUE7DAosQMdBytfTYfcO4mJOulGImjq9SMeBWyIg/132",
	"filter_time": 1771247064,
	"appmsg_fe_filter": "contenteditable",
	"is_login": 1,
	"page_type": 2,
	"item_show_type": 8,
	"voice_in_appmsg": [],
	"video_page_info": {
		"mp_video_trans_info": [],
		"drama_video_info": {},
		"drama_info": {}
	},
	"malicious_title_reason_id": 0,
	"picture_page_info_list": [
		{
			"cdn_url": "https://mmbiz.qpic.cn/mmbiz_jpg/jkww1kIUWicXD1aq0gS4WsDfvpqqQkqibwxaxjmoibw9PMYO5GSyUUcDre6gCF5oTc40rYqp4I2bXLBdaloU0Rs1tHFLU7iaRkayL55lboVqEbA/0?wx_fmt=jpeg",
			"width": 690,
			"height": 1035,
			"theme_color": "rgb(179,141,156)",
			"is_qr_code": 0,
			"poi_info": [],
			"wxa_info": [],
			"live_photo": {
				"format_info": []
			},
			"disable_theme_color": true,
			"bind_ad_info": [],
			"cps_ad_info": [],
			"pic_window_product": {
				"product_encrypt_key": "",
				"product_type": 0,
				"title": "",
				"data_type": 0,
				"product_id": ""
			},
			"show_watermark": true,
			"bottom_right_brightness": 0.11649811,
			"watermark_info": {
				"cdn_url": "http://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicWvr9YwPbDSM4gC1155tIZwiasHN5M6PPnUibsp25eWaZ86R37bVMy7hxW4FUYBcE1ialU0LLTljibLiajiclLqZgkChvNLgX9RqGq1A/0?wx_fmt=jpeg",
				"is_uploader": true
			},
			"spot_product_info": [],
			"share_cover": {
				"file_id": 100000394,
				"width": 690,
				"height": 920,
				"cdn_url": "https://mmbiz.qpic.cn/mmbiz_jpg/jkww1kIUWicXsUTUXrWxAYVSNm05FUkUe70Njtu7ADPtJXQ50amTJ714Lz7eBVtPXzkWSvFFyAoIbuNz5QDT5cY85cibB3OpsjYmAsxqo6cTw/0?wx_fmt=jpeg",
				"crop_info": "{\"ori_url\":\"http://318.wxapp.tc.qq.com/318/20304/stodownload?m=a662f092e1b5523e2c3084191acfd4d1&amp;filekey=30350201010421301f0202013e040253480410a662f092e1b5523e2c3084191acfd4d1020304cf5a040d00000004627466730000000132&amp;hy=SH&amp;storeid=26993159400063744e6b4fe290000013e00004f50534825f607d156bc94f4b&amp;bizid=1023\",\"x1\":0.0,\"y1\":0.055555556,\"x2\":1.0,\"y2\":0.944444418}"
			}
		},
		{
			"cdn_url": "https://mmbiz.qpic.cn/mmbiz_jpg/jkww1kIUWicVjTQJzpQxSb8Kb63weGR7IABLcWHrfMR63x88MCibSR17X0TTQE38Hg6ago1tUdS3xpqQqqzJNUEGUMd9QibHvZricHiaiavWRYvW4/0?wx_fmt=jpeg",
			"width": 690,
			"height": 920,
			"theme_color": "rgb(180,147,132)",
			"is_qr_code": 0,
			"poi_info": [],
			"wxa_info": [],
			"live_photo": {
				"format_info": []
			},
			"disable_theme_color": true,
			"bind_ad_info": [],
			"cps_ad_info": [],
			"pic_window_product": {
				"product_encrypt_key": "",
				"product_type": 0,
				"title": "",
				"data_type": 0,
				"product_id": ""
			},
			"show_watermark": true,
			"bottom_right_brightness": 0.32378733,
			"watermark_info": {
				"cdn_url": "http://mmbiz.qpic.cn/mmbiz_jpg/jkww1kIUWicUavSZOCQdNtN4YaHVVAmAsRYI3b6XtDVo7e7s1k2jj8tFh8SOmnpVj8GBkibkibsibUQLwa27Ku3ccS40vSSwic95ibeXUejYNRrJ0/0?wx_fmt=jpeg",
				"is_uploader": true
			},
			"spot_product_info": [],
			"share_cover": {
				"file_id": 0,
				"width": 0,
				"height": 0,
				"cdn_url": "",
				"crop_info": ""
			}
		},
		{
			"cdn_url": "https://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicWEBRXs2M51AsQYU7UNyUjJN4HyawwpNEbNaNlKOEH66umhneSpWialEqCaXWplSbLKp3ynD7WxCUSMhI9Tgc1ibiauBdqj2R26kc/0?wx_fmt=jpeg",
			"width": 690,
			"height": 919,
			"theme_color": "rgb(8,10,14)",
			"is_qr_code": 0,
			"poi_info": [],
			"wxa_info": [],
			"live_photo": {
				"format_info": []
			},
			"disable_theme_color": true,
			"bind_ad_info": [],
			"cps_ad_info": [],
			"pic_window_product": {
				"product_encrypt_key": "",
				"product_type": 0,
				"title": "",
				"data_type": 0,
				"product_id": ""
			},
			"show_watermark": true,
			"bottom_right_brightness": 0.76631635,
			"watermark_info": {
				"cdn_url": "http://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicVh50jiadYByQNRZRHlb5LDJe7FiaTp9CQHM1TETcS4ibdvQtnicqEHBf3F77NGqDIF60TxuA1sUE4pRG0Cshnibp0gYszFvoicQxv6c/0?wx_fmt=jpeg",
				"is_uploader": true
			},
			"spot_product_info": [],
			"share_cover": {
				"file_id": 0,
				"width": 0,
				"height": 0,
				"cdn_url": "",
				"crop_info": ""
			}
		},
		{
			"cdn_url": "https://mmbiz.qpic.cn/mmbiz_jpg/jkww1kIUWicWselTjcmt8njXatkXskes1CY1uibOxLZ35xPPJyabKOryT718IlmqxUYJLL2WaFlUDtibVtzTlQuiaVCthZRopyGEbk6oWQCtBwY/0?wx_fmt=jpeg",
			"width": 690,
			"height": 388,
			"theme_color": "rgb(30,17,10)",
			"is_qr_code": 0,
			"poi_info": [],
			"wxa_info": [],
			"live_photo": {
				"format_info": []
			},
			"disable_theme_color": true,
			"bind_ad_info": [],
			"cps_ad_info": [],
			"pic_window_product": {
				"product_encrypt_key": "",
				"product_type": 0,
				"title": "",
				"data_type": 0,
				"product_id": ""
			},
			"show_watermark": true,
			"bottom_right_brightness": 0.11327279,
			"watermark_info": {
				"cdn_url": "http://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicWDaEX1667YsCpymLFIUEyPtYAf2kyt60as2s7hpqphHhNwG5q9xNFHQ33XrhLNXolSS2cshAEibX3wnOQicqz5PR7dQicz2YWQmM/0?wx_fmt=jpeg",
				"is_uploader": true
			},
			"spot_product_info": [],
			"share_cover": {
				"file_id": 0,
				"width": 0,
				"height": 0,
				"cdn_url": "",
				"crop_info": ""
			}
		},
		{
			"cdn_url": "https://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicXyryEvyuVfTEm2mUiawufoeeTXphouI35BCic9BWI4HhTuYI4BscXia9S3bOtBn4Dwhuz1ojIROomibPicr9NIPWEVOWNxQ1jULJ3g/0?wx_fmt=jpeg",
			"width": 690,
			"height": 467,
			"theme_color": "rgb(5,1,2)",
			"is_qr_code": 0,
			"poi_info": [],
			"wxa_info": [],
			"live_photo": {
				"format_info": []
			},
			"disable_theme_color": true,
			"bind_ad_info": [],
			"cps_ad_info": [],
			"pic_window_product": {
				"product_encrypt_key": "",
				"product_type": 0,
				"title": "",
				"data_type": 0,
				"product_id": ""
			},
			"show_watermark": true,
			"bottom_right_brightness": 0.14996244,
			"watermark_info": {
				"cdn_url": "http://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicUiaeDUO9vTgMu8LkJSY6jplQAn9Qb13BvjhUaWfHAhB6iahON2wkIjcJWfOdicgPIyFs5ysO4ib2nO9HVtGR1cIGXVyqdokLenJtE/0?wx_fmt=jpeg",
				"is_uploader": true
			},
			"spot_product_info": [],
			"share_cover": {
				"file_id": 0,
				"width": 0,
				"height": 0,
				"cdn_url": "",
				"crop_info": ""
			}
		},
		{
			"cdn_url": "https://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicVMX49ASN3U3FfYMG9BwtxeeEs652XCL7LYQ5WpvRIWy0zfc8Fnsm5j8aKxicoQDX51txGMFzx2D5RGZWn3rUibMfw0MKNRekEgA/0?wx_fmt=jpeg",
			"width": 690,
			"height": 462,
			"theme_color": "rgb(9,16,22)",
			"is_qr_code": 0,
			"poi_info": [],
			"wxa_info": [],
			"live_photo": {
				"format_info": []
			},
			"disable_theme_color": true,
			"bind_ad_info": [],
			"cps_ad_info": [],
			"pic_window_product": {
				"product_encrypt_key": "",
				"product_type": 0,
				"title": "",
				"data_type": 0,
				"product_id": ""
			},
			"show_watermark": true,
			"bottom_right_brightness": 0.056787297,
			"watermark_info": {
				"cdn_url": "http://mmbiz.qpic.cn/mmbiz_jpg/jkww1kIUWicX5h4PCmceyrFO27j8RdqyJiaupttEs1aCh4QAaibrVKvOaIS0kJOEGqH5ROSAbjROmTrGibplWeGf5l0M1dkBzjicVZ8HibKrq9jzU/0?wx_fmt=jpeg",
				"is_uploader": true
			},
			"spot_product_info": [],
			"share_cover": {
				"file_id": 0,
				"width": 0,
				"height": 0,
				"cdn_url": "",
				"crop_info": ""
			}
		}
	],
	"locationlist": [],
	"hotspotinfolist": [],
	"isnew": 0,
	"malicious_content_type": 0,
	"is_top_stories": 0,
	"video_ids": [],
	"isprofileblock": 0,
	"cdn_url_235_1": "",
	"cdn_url_1_1": "http://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicUyebsWSKudpICDiaNT47g0OlJnSEyAgEO0okGP1V9YAWiaYDMeEspxFzrh5Iq9Xfo45a2TXOq7VZ4bGxJUtRteUox8kSFr9aqEI/0?wx_fmt=jpeg",
	"more_read_type": 0,
	"appmsg_like_type": 2,
	"ori_send_time": 1771247066,
	"show_top_bar": 0,
	"related_tag": [],
	"user_info": {
		"show_top_bar": 0,
		"enter_id": 1637065283,
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
				"type": 4,
				"version": 8339499,
				"lang": "zh_CN",
				"fullversion": "8339499-zh_CN-html",
				"versiongroup": "zh_CN-html"
			}
		],
		"isoversea": 0,
		"search_keyword": {
			"item_list": [],
			"exp_info": "",
			"need_baike_preload": true,
			"show_ad_keyword": false,
			"ad_item_list": []
		},
		"tts_heard_person_cnt": 0,
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
			"like_count": 3,
			"old_like_count": 10,
			"share_count": 2,
			"comment_count": 1,
			"get_data_succ": 1,
			"collect_count": 0,
			"show_collect": 1,
			"show_collect_gray": 0,
			"read_num": 520,
			"show_read": 1,
			"show_friend_seen": 2,
			"friend_subscribe_count": 0,
			"is_subscribed": 0,
			"star_num": 0,
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
		"subcount_version": 3,
		"reward_half_panel": 1,
		"listen_player_info": {
			"recommend_info_buffer": "IoQBeyJtcF9zY2VuZSI6MCwibXBfc3ViX3NjZW5lIjowLCJfX2JpeiI6Ik16ZzNNRFl5TVRJeU5RPT0iLCJhcHBtc2dfaWQiOjIyNDc0ODQwNDcsIml0ZW1faWR4IjoxLCJtcF9nZXRfYThrZXlfc2NlbmUiOjcsInRyYWNlX2ZsYWciOjB9KhwQCRoYCg4IqfzTtQ4Qj8XXrwgYARAAGAAgBygA"
		},
		"show_comment_entrance": 2,
		"share_h5info": {
			"underline_url": "https://mp.weixin.qq.com/mp/underline?action=get_appmsg_segment&clicktype=2&show_comment=1#wechat_redirect",
			"interaction_url": "https://mp.weixin.qq.com/mp/getinteraction?action=get_usertypelist&type=0&get_save=1&get_lastread=1&clicktype=2#wechat_redirect"
		},
		"short_link": "https://mp.weixin.qq.com/s/SXyNocq1-K4WkFcI-0aD6w",
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
	"is_area_shield": 0,
	"shield_areaids": [],
	"appmsg_ext_get": {
		"func_flag": 0
	},
	"anchor_tree": [],
	"voice_in_appmsg_list_json": "{\"voice_in_appmsg\":[]}",
	"live_info": [],
	"lang": "zh_CN",
	"cdn_url_16_9": "http://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicWjQnj0hnwpyoe2ml30P0lafSlnqItOicjTxE6fBtK37Ty9icPnKltBxmWZX3xG1sGR4y8icWKovf5BXOYrTtPOhPgH4KUvGmXRiaU/0?wx_fmt=jpeg",
	"real_item_show_type": 8,
	"url_item_show_type": 0,
	"video_page_infos": [],
	"can_use_wecoin": 1,
	"wecoin_tips": 0,
	"front_end_additional_fields": {
		"is_auto_type_setting": 0,
		"save_type": 0,
		"template_version": ""
	},
	"open_fansmsg": 0,
	"is_cooling_appmsg": 0,
	"ip_wording": {
		"country_name": "中国",
		"country_id": "156",
		"province_name": "山东"
	},
	"show_ip_wording": 1,
	"is_acct_area_shield": 0,
	"shield_acct_areaids": [],
	"style_type": 3,
	"shield_areas_info": [],
	"create_timestamp": 1771247066,
	"masonry_feed_info": {
		"version": 1,
		"from_old_app": 0
	},
	"picture_list_in_pictext": [],
	"servicetype": 0,
	"segment_comment_id": "4389839031287595009",
	"ad_mark_status": 0,
	"hide_ad_mark_on_cps": 0,
	"finder_audio_card": "{\"list\":[]}",
	"claim_source": {},
	"extra_comment_id": "4389839030448734209",
	"last_text": [],
	"wash_status": 0,
	"enterid": 1783694315,
	"zhuge_qa_id_list": [],
	"sec_control_info": {
		"list": []
	},
	"cdn_url_3_4": "http://mmbiz.qpic.cn/sz_mmbiz_jpg/jkww1kIUWicUvMicE6ic0vWsTYFmzCW1jH1L5aaIpciaW8I0xch0Qxz1kPBCGpGOaX2a7ib8CmASVZNUWicLCrhricicZ8ib1aRicd7ibN5HRz1BMejtI4/0?wx_fmt=jpeg",
	"window_product_list": [],
	"finder_music_card": "{\"list\":[]}",
	"finder_audio_card_list": {
		"list": []
	},
	"finder_music_card_list": {
		"list": []
	},
	"new_service_type": 1,
	"product_activity": {},
	"rt_biz_info": {},
	"redpacket_cover_list": [],
	"footer_gift_activity": {},
	"verify_status": 0,
	"is_phacct_verify": 0,
	"watermark_setting": 2,
	"title_gen_type": 0,
	"appmsg_listen_id": "150432544708236504",
	"trans_appmsg_info": {},
	"location": {
		"type": 3,
		"poiid": "14277584900817569868",
		"province": "山东省",
		"city": "临沂市",
		"districtid": "",
		"longitude": "118.357040",
		"latitude": "35.074730",
		"name": "临沂鲁商铂尔曼大酒店",
		"address": "山东省临沂市兰山区涑河北街1号",
		"content": "临沂市 · 临沂鲁商铂尔曼大酒店",
		"country": "中国",
		"region": "兰山区",
		"adcode": "371302"
	},
	"poi_info": {
		"type": 3,
		"poiid": "14277584900817569868",
		"districtid": "",
		"name": "临沂鲁商铂尔曼大酒店",
		"address": "山东省临沂市兰山区涑河北街1号",
		"content": "临沂市 · 临沂鲁商铂尔曼大酒店",
		"biz_count": 0
	},
	"fast_send_info": {
		"send_source": 3
	},
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

func TestArticleToProfile_FromPicturesJSON(t *testing.T) {
	var pageJSON wxmp.CgiDataNew
	if err := json.Unmarshal([]byte(picturesJSON), &pageJSON); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	article := &wxmp.WechatOfficialArticle{
		PageJSON: &pageJSON,
	}
	profile, err := wxmp.ArticleToProfile(article, pageJSON.Link)
	if err != nil {
		t.Fatalf("ArticleToProfile: %v", err)
	}

	if profile.ArticleID == "" {
		t.Error("ArticleID should not be empty")
	}
	if profile.Title != "凤求凰" {
		t.Errorf("Title = %q, want %q", profile.Title, "凤求凰")
	}
	if profile.CoverURL == "" {
		t.Error("CoverURL should not be empty")
	}
	if profile.PublishTime != 1771247066 {
		t.Errorf("PublishTime = %d, want 1771247066", profile.PublishTime)
	}
	if profile.Author.ExternalId != "gh_8951dcd584fe" {
		t.Errorf("Author.ExternalId = %q, want %q", profile.Author.ExternalId, "gh_8951dcd584fe")
	}
	if profile.Author.Nickname != "日照茶人茶事" {
		t.Errorf("Author.Nickname = %q, want %q", profile.Author.Nickname, "日照茶人茶事")
	}
	if profile.Author.AvatarURL == "" {
		t.Error("Author.AvatarURL should not be empty")
	}
}
