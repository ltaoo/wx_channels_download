package wxmp_test

import (
	"encoding/json"
	"testing"

	wxmp "wx_channel/internal/adapter/officialaccount"
)

const audioJSON = `{
	"base_resp": {
		"ret": 0,
		"errmsg": "ok",
		"wxtoken": 777,
		"cookie_count": 0,
		"sessionid": "svr_f5088f0fe82"
	},
	"user_name": "gh_3eb029e8c70b",
	"nick_name": "几乎满级",
	"round_head_img": "http://mmbiz.qpic.cn/mmbiz_png/6SfPkcUDfbS0MtT957c1axiaus4bQbNZ0QTyhfh6Mic8rVMNOO89gBfN5zianCthktGtwuVwD3bV5MYvYYwWAYgIg/0?wx_fmt=png",
	"title": "2024年11月12号参考快讯",
	"desc": "1、2025年度省级公务员考试报名已经开始，多个省份发布了招考公告，标志着公职人员选拔的启动。今年的省考计划更加倾向于招聘应届高校毕业生和基层一线人员。部分省份（如上海、浙江、江苏、山东等）将部分职位的招录年龄放宽至40岁。\n&amp;nbsp;\n2、湖南怀化市鹤城区长泥坡村的发型师晓华因真诚服务在网络走红，吸引了大量游客。7天内，村庄人流量超过20万人次，带动消费达2000万元，总消费拉动约1.2亿元。当地商务部门组织了“怀小创”直播团队，推动了60（有的报道称 80 余家）余家特色商家入驻，提升了商业活力。同时，怀化市通过线上线下宣传，提升了知名度，并举办便民节活动，提供了丰富的商品和服务。\n&amp;nbsp;\n3、11月8日，全国人大表决通过《中华人民共和国学前教育法》，将于2025年6月1日起实施。该法明确学前教育为公益普惠性质，规定禁止使用公共资金或资产举办营利性民办幼儿园，幼儿园不得作为企业资产在境内外上市，确保专注于教育本身而非商业化。同时，推动普惠性学前教育资源供给，强调政府主导，并规范社会力量参与。该法还规定，存在犯罪或严重不当行为的人禁止从事学前教育，幼儿园必须对教师进行背景检查，并对未履行安全责任的单位追责。\n&amp;nbsp;\n4、11月11日，商务部发布公告，宣布自11月15日起对原产于欧盟的进口白兰地实施临时反倾销措施。根据调查，初步认定欧盟进口白兰地存在倾销行为，威胁国内产业。进口经营者需按公告规定的保证金比率向海关提供保证金或保函。\n&amp;nbsp;\n5、苹果第二代 Vision Pro 预计明年秋季或 2026 年春季上市，外观设计大致不变，但芯片将从M2升级到M5，性能会大幅提升。为避免技术过时，苹果计划与首款 M5 Mac 同步发布。有分析师指出，苹果已推迟推出更便宜版本的计划，重点转向第二代产品。\n&amp;nbsp;\n6、有 OpenAI 的员工表示，OpenAI的新模型Orion进展放缓，虽然在语言任务上超越现有模型，但在编码等任务上未必有显著改进。Orion的进步放缓可能与高质量训练数据短缺有关，OpenAI开始使用AI生成的数据进行训练，这可能导致Orion与 GPT-4 等旧模型相似。此外，OpenAI的首席安全研究员Lilian Weng于11月8日离职，近期这一系列离职情况让人们担忧OpenAI是否真关注安全问题。\n&amp;nbsp;\n7、11月12日，比特币突破8.8万美元，过去一周上涨超16%，市值达到1.72万亿美元。其他加密货币如以太坊和币安币也随之上涨。特朗普曾表示，如果重返白宫，会将比特币纳入美国战略储备。彭博社分析称，特朗普当选带来友好监管预期，同时美联储降息也推动了比特币价格上涨。\n&amp;nbsp;\n8、波音计划裁减17,000 个岗位，约占全球员工数量的 10%，同时补偿因罢工而失去工资的员工。波音还考虑以60亿美元出售子公司杰普森导航部门，以减轻580亿美元债务。上月，波音筹集了240亿美元资金以维持信用评级。路透社分析指出，出售资产和裁员有助于波音集中资源于民用飞机制造等核心业务。\n&amp;nbsp;\n9、大疆计划于明年中期发布自研扫地机器人，定价接近高端市场。公司希望通过扫地机器人等新品推动增长，并减少对无人机的依赖。虽然扫地机器人与无人机的技术有重叠，大疆已研发该产品4年，但面临激烈的市场竞争，且多个技术问题已被其他品牌优化，能否成功推出竞争力产品仍是未知数。\n&amp;nbsp;\n10、特斯拉在美国推出电动皮卡Cybertruck的租赁服务，月租999美元起。Cybertruck初始版本售价超10万美元，远高于2019年公布的3.99万美元，并且订单积压严重，销量落后于Model Y和Model 3，租赁服务可能是为了刺激需求。\n&amp;nbsp;\n11、奥迪与上汽合作推出新品牌“AUDI”，该品牌将使用全部大写字母替代传统的四环标志，并计划在未来三年推出三款纯电动车。奥迪正在重塑品牌形象，力求摆脱燃油车形象，以提升在中国市场的表现。然而，没了四环标志可能需要更多资源来建立新品牌的知名度。\n&amp;nbsp;\n12、高端羽绒服品牌加拿大鹅发最新财报，7月至9月季度营收同比下降5%至1.93亿美元，但大中华区销售增长了5.7%。加拿大鹅正在多元化产品，推出雨衣等非冬季服饰，并通过抖音直播渠道提升电商收入。此外，加拿大鹅在中国继续扩展专卖店。不过，由于美国奢侈品需求疲软，公司下调了全年收入预期。\n&amp;nbsp;\n13、国际电商巨头eBay接入支付宝，允许中国消费者在跨境购物时直接使用支付宝支付。此前，eBay仅支持PayPal、Apple Pay和信用卡支付。此外，京东也在10月底重新开通支付宝支付，这是双方13年后再次合作。阿里、腾讯、京东、美团等互联网巨头“拆墙”趋势明显，如支付宝与京东买药合作、淘宝支持微信支付等互通合作，提升了支付便捷性。同时，支付宝与华为、小米等合作，推广“碰一下”支付技术，无需扫码即可完成支付。",
	"content_noencode": "1、2025年度省级公务员考试报名已经开始，多个省份发布了招考公告，标志着公职人员选拔的启动。今年的省考计划更加倾向于招聘应届高校毕业生和基层一线人员。部分省份（如上海、浙江、江苏、山东等）将部分职位的招录年龄放宽至40岁。\n&nbsp;\n2、湖南怀化市鹤城区长泥坡村的发型师晓华因真诚服务在网络走红，吸引了大量游客。7天内，村庄人流量超过20万人次，带动消费达2000万元，总消费拉动约1.2亿元。当地商务部门组织了“怀小创”直播团队，推动了60（有的报道称 80 余家）余家特色商家入驻，提升了商业活力。同时，怀化市通过线上线下宣传，提升了知名度，并举办便民节活动，提供了丰富的商品和服务。\n&nbsp;\n3、11月8日，全国人大表决通过《中华人民共和国学前教育法》，将于2025年6月1日起实施。该法明确学前教育为公益普惠性质，规定禁止使用公共资金或资产举办营利性民办幼儿园，幼儿园不得作为企业资产在境内外上市，确保专注于教育本身而非商业化。同时，推动普惠性学前教育资源供给，强调政府主导，并规范社会力量参与。该法还规定，存在犯罪或严重不当行为的人禁止从事学前教育，幼儿园必须对教师进行背景检查，并对未履行安全责任的单位追责。\n&nbsp;\n4、11月11日，商务部发布公告，宣布自11月15日起对原产于欧盟的进口白兰地实施临时反倾销措施。根据调查，初步认定欧盟进口白兰地存在倾销行为，威胁国内产业。进口经营者需按公告规定的保证金比率向海关提供保证金或保函。\n&nbsp;\n5、苹果第二代 Vision Pro 预计明年秋季或 2026 年春季上市，外观设计大致不变，但芯片将从M2升级到M5，性能会大幅提升。为避免技术过时，苹果计划与首款 M5 Mac 同步发布。有分析师指出，苹果已推迟推出更便宜版本的计划，重点转向第二代产品。\n&nbsp;\n6、有 OpenAI 的员工表示，OpenAI的新模型Orion进展放缓，虽然在语言任务上超越现有模型，但在编码等任务上未必有显著改进。Orion的进步放缓可能与高质量训练数据短缺有关，OpenAI开始使用AI生成的数据进行训练，这可能导致Orion与 GPT-4 等旧模型相似。此外，OpenAI的首席安全研究员Lilian Weng于11月8日离职，近期这一系列离职情况让人们担忧OpenAI是否真关注安全问题。\n&nbsp;\n7、11月12日，比特币突破8.8万美元，过去一周上涨超16%，市值达到1.72万亿美元。其他加密货币如以太坊和币安币也随之上涨。特朗普曾表示，如果重返白宫，会将比特币纳入美国战略储备。彭博社分析称，特朗普当选带来友好监管预期，同时美联储降息也推动了比特币价格上涨。\n&nbsp;\n8、波音计划裁减17,000 个岗位，约占全球员工数量的 10%，同时补偿因罢工而失去工资的员工。波音还考虑以60亿美元出售子公司杰普森导航部门，以减轻580亿美元债务。上月，波音筹集了240亿美元资金以维持信用评级。路透社分析指出，出售资产和裁员有助于波音集中资源于民用飞机制造等核心业务。\n&nbsp;\n9、大疆计划于明年中期发布自研扫地机器人，定价接近高端市场。公司希望通过扫地机器人等新品推动增长，并减少对无人机的依赖。虽然扫地机器人与无人机的技术有重叠，大疆已研发该产品4年，但面临激烈的市场竞争，且多个技术问题已被其他品牌优化，能否成功推出竞争力产品仍是未知数。\n&nbsp;\n10、特斯拉在美国推出电动皮卡Cybertruck的租赁服务，月租999美元起。Cybertruck初始版本售价超10万美元，远高于2019年公布的3.99万美元，并且订单积压严重，销量落后于Model Y和Model 3，租赁服务可能是为了刺激需求。\n&nbsp;\n11、奥迪与上汽合作推出新品牌“AUDI”，该品牌将使用全部大写字母替代传统的四环标志，并计划在未来三年推出三款纯电动车。奥迪正在重塑品牌形象，力求摆脱燃油车形象，以提升在中国市场的表现。然而，没了四环标志可能需要更多资源来建立新品牌的知名度。\n&nbsp;\n12、高端羽绒服品牌加拿大鹅发最新财报，7月至9月季度营收同比下降5%至1.93亿美元，但大中华区销售增长了5.7%。加拿大鹅正在多元化产品，推出雨衣等非冬季服饰，并通过抖音直播渠道提升电商收入。此外，加拿大鹅在中国继续扩展专卖店。不过，由于美国奢侈品需求疲软，公司下调了全年收入预期。\n&nbsp;\n13、国际电商巨头eBay接入支付宝，允许中国消费者在跨境购物时直接使用支付宝支付。此前，eBay仅支持PayPal、Apple Pay和信用卡支付。此外，京东也在10月底重新开通支付宝支付，这是双方13年后再次合作。阿里、腾讯、京东、美团等互联网巨头“拆墙”趋势明显，如支付宝与京东买药合作、淘宝支持微信支付等互通合作，提升了支付便捷性。同时，支付宝与华为、小米等合作，推广“碰一下”支付技术，无需扫码即可完成支付。",
	"create_time": "2024-11-12 16:24",
	"cdn_url": "",
	"link": "https://mp.weixin.qq.com/s?__biz=MzkwODQzNzk1OQ==&amp;mid=2247486025&amp;idx=1&amp;sn=c81360456e73680aa120e191ccad32e5&amp;chksm=c10730388a4b1e5d39e5e6b9f3c41d6049ddc6d068d9b974d3b5998013d06b4166d08e6ff190#rd",
	"source_url": "",
	"can_share": 1,
	"alias": "",
	"type": 10002,
	"author": "",
	"is_limit_user": 0,
	"show_cover_pic": 0,
	"advertisement_info": [],
	"ori_create_time": 1731399898,
	"user_uin": "2993894182",
	"total_item_num": 1,
	"is_async": 1,
	"comment_id": "3721314505049849859",
	"img_format": "",
	"svr_time": 1783694103,
	"copyright_info": {
		"copyright_stat": 0,
		"is_cartoon_copyright": 0
	},
	"can_reward": 0,
	"signature": "感谢支持、关注！备用号“魏咕咕响”",
	"in_mm": 1,
	"app_id": "wx368698fa12e70106",
	"show_comment": 0,
	"can_use_page": 0,
	"hd_head_img": "http://wx.qlogo.cn/mmhead/DmTSLTdleesgzwqgBruffOOiako9Hicr49ITgEOtvPvFjaETGic0HmsIkv6Npn8pndCFJOGg7MtloM/0",
	"del_reason_id": 0,
	"srcid": "0710qFdimZDgwU3UgBx12xAy",
	"is_wxg_stuff_uin": 0,
	"need_report_cost": 0,
	"use_tx_video_player": 0,
	"is_only_read": 1,
	"req_id": "1022WY9M2zF8KvQUu8oU76tc",
	"use_outer_link": 0,
	"ban_scene": 0,
	"csp_nonce_str": 1098516126,
	"msg_daily_idx": 0,
	"ori_head_img_url": "http://wx.qlogo.cn/mmhead/DmTSLTdleesgzwqgBruffOOiako9Hicr49ITgEOtvPvFjaETGic0HmsIkv6Npn8pndCFJOGg7MtloM/132",
	"filter_time": 1731399879,
	"appmsg_fe_filter": "contenteditable",
	"is_login": 1,
	"page_type": 2,
	"item_show_type": 7,
	"voice_in_appmsg": [
		{
			"voice_id": "MzkwODQzNzk1OV8yMjQ3NDg2MDI0",
			"sn": "595a162a89d4b3edbac6c4b23ef3cf4d",
			"voice_md5": "734f8728dba432eb21f415f5f1aff3ef",
			"listen_id": "222449335240197750"
		}
	],
	"video_page_info": {
		"mp_video_trans_info": [],
		"drama_video_info": {},
		"drama_info": {}
	},
	"malicious_title_reason_id": 0,
	"voice_page_info": {
		"voice_id": "MzkwODQzNzk1OV8yMjQ3NDg2MDI0",
		"duration": 323,
		"high_size": 2588675,
		"low_size": 662938,
		"accept_aac": 1,
		"voice_md5": "734f8728dba432eb21f415f5f1aff3ef",
		"voice_verify_state": 3,
		"cover_url": "http://wx.qlogo.cn/mmopen/PwPnibrSfJoV2sHQ8rIvY3njib59FktmVO4ibzkFqm11WibibOZz4Zc8k6XI2gZB0OaDhibYUqgLs96mbooDoibVHLKV3lczia8v7HVC/0",
		"desc": "",
		"title": "2024年11月12号参考快讯",
		"listen_id": "222449335240197750",
		"light_cover_color": "#B2FFDC",
		"dark_cover_color": "#004D2A"
	},
	"picture_page_info_list": [],
	"locationlist": [],
	"hotspotinfolist": [],
	"isnew": 0,
	"malicious_content_type": 0,
	"is_top_stories": 0,
	"video_ids": [],
	"isprofileblock": 0,
	"cdn_url_235_1": "",
	"cdn_url_1_1": "",
	"more_read_type": 0,
	"appmsg_like_type": 2,
	"ori_send_time": 1731399898,
	"show_top_bar": 0,
	"related_tag": [],
	"user_info": {
		"show_top_bar": 0,
		"enter_id": 827789730,
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
		"tts_heard_person_cnt": 4,
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
			"like_count": 0,
			"old_like_count": 0,
			"share_count": 0,
			"comment_count": 0,
			"get_data_succ": 1,
			"collect_count": 0,
			"show_collect": 1,
			"show_collect_gray": 0,
			"read_num": 3,
			"listen_audio_uv": 4,
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
				"friend_info": []
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
			"recommend_info_buffer": "IoQBeyJtcF9zY2VuZSI6MCwibXBfc3ViX3NjZW5lIjowLCJfX2JpeiI6Ik16a3dPRFF6TnprMU9RPT0iLCJhcHBtc2dfaWQiOjIyNDc0ODYwMjUsIml0ZW1faWR4IjoxLCJtcF9nZXRfYThrZXlfc2NlbmUiOjcsInRyYWNlX2ZsYWciOjB9KhwQCRoYCg4Ix4/Yxw4QydTXrwgYARAAGAAgBygA"
		},
		"show_comment_entrance": 2,
		"share_h5info": {
			"underline_url": "https://mp.weixin.qq.com/mp/underline?action=get_appmsg_segment&clicktype=2&show_comment=1#wechat_redirect",
			"interaction_url": "https://mp.weixin.qq.com/mp/getinteraction?action=get_usertypelist&type=0&get_save=1&get_lastread=1&clicktype=2#wechat_redirect"
		},
		"short_link": "https://mp.weixin.qq.com/s/-f5JWhqgUTJZL6b-OGyQug",
		"quote_list": [],
		"red_flower_like_info": {
			"is_red_flower_like": 0
		},
		"indentity_id": "zSoOXxKy75HnZGbfs2lZfA",
		"get_search_keyword_realtime": 0,
		"voice_page_user_info": {
			"added_in_listenlater": 0
		},
		"voice_page_added_in_listenlater": 0,
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
	"voice_in_appmsg_list_json": "{\"voice_in_appmsg\":[{\"voice_id\":\"MzkwODQzNzk1OV8yMjQ3NDg2MDI0\",\"sn\":\"595a162a89d4b3edbac6c4b23ef3cf4d\",\"voice_md5\":\"734f8728dba432eb21f415f5f1aff3ef\",\"listen_id\":\"222449335240197750\"}]}",
	"live_info": [],
	"lang": "zh_CN",
	"cdn_url_16_9": "",
	"real_item_show_type": 7,
	"url_item_show_type": 0,
	"video_page_infos": [],
	"can_use_wecoin": 1,
	"wecoin_tips": 0,
	"front_end_additional_fields": {
		"is_auto_type_setting": 3,
		"save_type": 0,
		"template_version": "92908883"
	},
	"open_fansmsg": 0,
	"is_cooling_appmsg": 0,
	"ip_wording": {
		"country_name": "中国",
		"country_id": "156",
		"province_name": "广西"
	},
	"show_ip_wording": 1,
	"is_acct_area_shield": 0,
	"shield_acct_areaids": [],
	"style_type": 3,
	"shield_areas_info": [],
	"create_timestamp": 1731399898,
	"picture_list_in_pictext": [],
	"servicetype": 0,
	"segment_comment_id": "0",
	"ad_mark_status": 0,
	"hide_ad_mark_on_cps": 0,
	"finder_audio_card": "{\"list\":[]}",
	"claim_source": {
		"claim_source_type": 2,
		"claim_source": "素材来源官方媒体/网络新闻"
	},
	"extra_comment_id": "3721314471629635587",
	"last_text": [],
	"wash_status": 0,
	"enterid": 1783694101,
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
	"new_service_type": 1,
	"product_activity": {},
	"rt_biz_info": {},
	"redpacket_cover_list": [],
	"footer_gift_activity": {},
	"verify_status": 0,
	"is_phacct_verify": 0,
	"watermark_setting": 3,
	"title_gen_type": 0,
	"appmsg_listen_id": "150391741204461706",
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
}
`

func TestArticleToProfile_FromAudioJSON(t *testing.T) {
	var pageJSON wxmp.CgiDataNew
	if err := json.Unmarshal([]byte(audioJSON), &pageJSON); err != nil {
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
	if profile.Title != "2024年11月12号参考快讯" {
		t.Errorf("Title = %q, want %q", profile.Title, "2024年11月12号参考快讯")
	}
	if profile.CoverURL != "" {
		t.Errorf("CoverURL = %q, want empty", profile.CoverURL)
	}
	if profile.PublishTime != 1731399898 {
		t.Errorf("PublishTime = %d, want 1731399898", profile.PublishTime)
	}
	if profile.Author.ExternalId != "gh_3eb029e8c70b" {
		t.Errorf("Author.ExternalId = %q, want %q", profile.Author.ExternalId, "gh_3eb029e8c70b")
	}
	if profile.Author.Nickname != "几乎满级" {
		t.Errorf("Author.Nickname = %q, want %q", profile.Author.Nickname, "几乎满级")
	}
	if profile.Author.AvatarURL == "" {
		t.Error("Author.AvatarURL should not be empty")
	}
}
