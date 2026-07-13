package officialaccount_test

import (
	"encoding/json"
	"testing"

	officialaccount "wx_channel/pkg/scraper/officialaccount"
)

const articleJSON = `{
	"base_resp": {
		"ret": 0,
		"errmsg": "ok",
		"wxtoken": 777,
		"cookie_count": 0,
		"sessionid": "svr_858a7b1f65c"
	},
	"user_name": "gh_8843e49ff5fa",
	"nick_name": "阿维AI实验室",
	"round_head_img": "http://mmbiz.qpic.cn/sz_mmbiz_png/DJATu6B3wYdHpicWB3QkiciblDrpp5nJBKYX2BfOwv9DTheUSiaoPG8BlgPruqxY7icRdIibAw1AicDpMyg2JAaENAea1Rn1pzcH6uFY72e0A1pZUY/0?wx_fmt=png",
	"title": "一个重新定义的待办软件",
	"desc": "计划总赶不上变化？一个重新定义的待办软件",
	"content_noencode": "<h1 style=\"font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;letter-spacing: normal;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 22px;font-weight: bold;color: rgb(93, 78, 55);text-align: center;margin: 30px 0px 40px;padding: 0px 10px;line-height: 1.4;\" data-pm-slice=\"0 0 []\"><span leaf=\"\">一个重新定义的待办软件</span></h1><h2 style=\"font-size: 18px;font-weight: bold;color: rgb(93, 78, 55);padding: 12px 15px;background: rgb(245, 240, 232);border-left: 4px solid rgb(192, 57, 43);margin: 0px;line-height: 1.4;\"><span leaf=\"\">计划总赶不上变化？这款待办软件把容错率拉满了</span></h2><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">你是不是也经历过这种崩溃：</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\"><span textstyle=\"\" style=\"font-weight: bold;\">精心排满一天的To-Do清单，结果突然被临时会议打断；</span></span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\"><span textstyle=\"\" style=\"font-weight: bold;\">或者因为状态不佳，看着一堆未完成任务产生强烈的焦虑感？</span></span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\"><span textstyle=\"\" style=\"font-weight: bold;\">更别提那些收藏在微信和抖音里“下次一定看”的干货链接，最后全成了数字垃圾。</span></span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\"><span textstyle=\"\" style=\"font-weight: bold;\">传统待办软件往往只强调“严格执行”，却忽略了人的精力波动和现实变数。</span></span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">今天介绍的Note-Plan，是一款彻底重新定义任务管理的Windows桌面程序。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">它不制造焦虑，而是通过内置的容错机制与自动化流程，让计划真正适配生活节奏。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">如果你受够了机械打卡式的效率工具，不妨看看这款能主动为你兜底的任务管理器。</span></p><h3 style=\"font-size: 16px;font-weight: bold;color: rgb(139, 115, 85);padding: 8px 0px;border-bottom: 2px solid rgb(192, 57, 43);margin: 0px;line-height: 1.4;\"><span leaf=\"\">一句话说清这个产品是什么</span></h3><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">Note-Plan是一款专为现代人设计的轻量级任务管理与灵感收集工具，</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">它打破了传统待办软件“死板执行”的逻辑，</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">将容错机制、自动化归档与极速交互整合在一个Windows程序中。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">它的核心功能聚焦于三大高频场景：</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\"><span textstyle=\"\" style=\"font-size: 18px;font-weight: bold;\">第一，动态容错调度。</span></span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">当你因突发状况或状态不佳无法完成当日计划时，系统默认提供3次“宽恕次数”，并<span textstyle=\"\" style=\"font-weight: bold;\">自动预留20%的缓冲时间</span>。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">一旦任务逾期，它会一键自动迁移至当天并打上红色标记，无需手动重排，直接清空心理负担。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\"><span textstyle=\"\" style=\"font-size: 18px;font-weight: bold;\">第二，一键内容自动化归档。</span></span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">面对抖音、微信中零散但高价值的内容，不再需要反复复制粘贴或手动建文件夹。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">只需输入链接，程序即可自动抓取标题、正文、作者与封面，并直接归档至知识库的“灵感采集”专属文件夹，彻底解决收藏夹吃灰问题。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\"><span textstyle=\"\" style=\"font-size: 18px;font-weight: bold;\">第三，极速灵感捕获。</span></span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">灵感稍纵即逝，Note-Plan支持桌面悬浮窗随时唤起。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">你可以通过Ctrl+Shift+N快捷键或点击FAB浮动按钮快速打开便签，支持6种颜色标记、置顶、收藏与分类管理，未读已读状态一目了然，确保每一个碎片想法都不被遗漏。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">与传统待办应用相比，Note-Plan砍掉了繁重的标签体系与多层级目录，采用“单窗口+快捷键+悬浮窗”的极简交互设计。</span></p><blockquote><p><span leaf=\"\">作为独立开发者利用AI工具全链路开发的产品，其迭代周期短、维护成本极低，因此能够保持纯粹的免费体验，持续为用户提供轻量、高效的效率支持。</span></p></blockquote><h2 style=\"font-size: 18px;font-weight: bold;color: rgb(93, 78, 55);padding: 12px 15px;background: rgb(245, 240, 232);border-left: 4px solid rgb(192, 57, 43);margin: 0px;line-height: 1.4;\"><span leaf=\"\">获取使用</span></h2><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">如果你也厌倦了越做越焦虑的打卡式待办，</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">想尝试一种更懂人性、更贴合实际节奏的任务管理方式，Note-Plan是一个值得立刻试水的选择。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">获取方式非常简单：</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\"><span textstyle=\"\" style=\"font-size: 17px;font-weight: bold;\">请前往本公众号，在对话框回复关键词“产品”，即可直接获取Windows客户端的下载地址。</span></span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">建议先收藏本文备用，遇到任务堆积或灵感断档时随时调用。</span></p><p style=\"color: rgb(74, 74, 74);font-family: -apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Roboto, &quot;Helvetica Neue&quot;, Arial, sans-serif;font-style: normal;font-variant-ligatures: normal;font-variant-caps: normal;font-weight: 400;orphans: 2;text-indent: 0px;text-transform: none;widows: 2;word-spacing: 0px;-webkit-text-stroke-width: 0px;white-space: normal;text-decoration-thickness: initial;text-decoration-style: initial;text-decoration-color: initial;font-size: 15px;line-height: 1.75;margin: 16px 0px;text-align: justify;letter-spacing: 0.5px;\"><span leaf=\"\">也欢迎转发给同样被效率工具困扰的朋友，一起用更聪明的方式管理工作与生活。</span></p><section><span leaf=\"\"><br  /></span></section><p style=\"display: none;\"><mp-style-type data-value=\"3\"></mp-style-type></p>",
	"create_time": "2026-07-10 18:33",
	"cdn_url": "https://mmbiz.qpic.cn/sz_mmbiz_jpg/DJATu6B3wYcMIsGWOr7iaGMMiaicmJ9KKV3iacE2EMxnzz3BofvlCibjPTmAdLtJzEneMAllzgOQ3iaPKbBOeGfVIUP8s1IjicAXWnx5zqnaf3lDYk/0?wx_fmt=jpeg",
	"link": "https://mp.weixin.qq.com/s?__biz=MzkyOTg5NjM3Mw==&amp;mid=2247483741&amp;idx=1&amp;sn=102a5aa1be99feee21ccaf6219898ed0&amp;chksm=c37bc15a2d9d1e2e3d1b9d7e096fc9231afa0763dc7a4e66a44623ed2718e94668420e9ace00#rd",
	"source_url": "",
	"can_share": 1,
	"alias": "",
	"type": 9,
	"author": "阿维AI实验室",
	"is_limit_user": 0,
	"show_cover_pic": 0,
	"advertisement_info": [],
	"ori_create_time": 1783679631,
	"user_uin": "2993894182",
	"total_item_num": 1,
	"is_async": 1,
	"comment_id": "4598422344129626114",
	"img_format": "jpeg",
	"svr_time": 1783693995,
	"copyright_info": {
		"copyright_stat": 1,
		"ori_article_type": "",
		"is_cartoon_copyright": 0
	},
	"can_reward": 1,
	"signature": "分享AI工具，效率技巧，保姆级教程\n每周更新，帮你用AI提升10倍效率",
	"reward_wording": "",
	"in_mm": 1,
	"app_id": "wx4f28db16cc872078",
	"show_comment": 0,
	"can_use_page": 0,
	"hd_head_img": "http://wx.qlogo.cn/mmhead/AbruuZ3ILCn7qOmgPicGG8YpNA36I8ws7EsuFLnF0NQTNwBsC8ia9psqRWS4ibfMK3PPqDyCpuMibCw/0",
	"del_reason_id": 0,
	"srcid": "0710RN9CnujqZEpQnIZS5TBC",
	"is_wxg_stuff_uin": 0,
	"need_report_cost": 0,
	"use_tx_video_player": 0,
	"is_only_read": 1,
	"req_id": "1022nnCni86oonWPTwMQcXny",
	"use_outer_link": 0,
	"ban_scene": 0,
	"csp_nonce_str": 2115880939,
	"msg_daily_idx": 1,
	"ori_head_img_url": "http://wx.qlogo.cn/mmhead/AbruuZ3ILCn7qOmgPicGG8YpNA36I8ws7EsuFLnF0NQTNwBsC8ia9psqRWS4ibfMK3PPqDyCpuMibCw/132",
	"filter_time": 1783679582,
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
	"reward_author_head": "https://wx.qlogo.cn/mmhead/AbruuZ3ILCn7qOmgPicGG8YpNA36I8ws7EsuFLnF0NQTNwBsC8ia9psqRWS4ibfMK3PPqDyCpuMibCw/0",
	"locationlist": [],
	"hotspotinfolist": [],
	"author_id": "ofMoI48423yCAz92pv9ql8UUD5Uo",
	"isnew": 0,
	"malicious_content_type": 0,
	"fasttmpl_version": 8339498,
	"is_top_stories": 0,
	"video_ids": [],
	"isprofileblock": 0,
	"cdn_url_235_1": "https://mmbiz.qpic.cn/sz_mmbiz_jpg/DJATu6B3wYcMIsGWOr7iaGMMiaicmJ9KKV3iacE2EMxnzz3BofvlCibjPTmAdLtJzEneMAllzgOQ3iaPKbBOeGfVIUP8s1IjicAXWnx5zqnaf3lDYk/0?wx_fmt=jpeg",
	"cdn_url_1_1": "https://mmbiz.qpic.cn/mmbiz_jpg/DJATu6B3wYetwRyzAFOzjOXQk2xNYjBRVL4qqJNNqursWUicDemDzgfMDSpgvibDt3WO73BXmvWJy9oWyziabNYZA6B49wRFTSX9VrGia1jNBEA/0?wx_fmt=jpeg",
	"more_read_type": 0,
	"appmsg_like_type": 2,
	"ori_send_time": 1783679631,
	"show_top_bar": 0,
	"related_tag": [],
	"user_info": {
		"show_top_bar": 0,
		"enter_id": 1574863519,
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
					"keyword": "Note-Plan",
					"idx_range_list": [
						{
							"begin_idx": 5,
							"end_idx": 13,
							"section_idx": 7
						},
						{
							"begin_idx": 7,
							"end_idx": 15,
							"section_idx": 22
						}
					],
					"s1s_stat_info": "%7B%22bizuin%22%3A3929896373%2C%22msgid%22%3A2247483741%2C%22msgidx%22%3A1%2C%22docid%22%3A%225430742543006981208%22%2C%22keywordItem%22%3A%7B%22keyword%22%3A%22Note-Plan%22%2C%22section_idx%22%3A7%2C%22begin_idx%22%3A5%2C%22end_idx%22%3A13%2C%22type%22%3A1024%2C%22lemma_id%22%3A%22%22%7D%2C%22category%22%3A%22%E7%A7%91%E6%8A%80_%E8%BD%AF%E4%BB%B6%E5%B7%A5%E5%85%B7%3A0.996628%22%2C%22reqId%22%3A1397032905987564947%2C%22S1SPageType%22%3A1%2C%22strReqId%22%3A%221397032905987564947%22%2C%22orgReqId%22%3A%229688189397549295113%22%2C%22item_show_type%22%3A0%2C%22common_value_expt%22%3A91%2C%22highlight_preload%22%3A0%7D",
					"s1s_context_info": "%7B%22keyword%22%3A%22note-plan%22%2C%22isNeedUpdateGPTInfo%22%3Afalse%2C%22S1SPageType%22%3A1%2C%22search_id%22%3A%229688189397549295113%22%2C%22doc_info%22%3A%7B%22triple%22%3A%7B%22bizuin%22%3A3929896373%2C%22msgid%22%3A2247483741%2C%22msgidx%22%3A1%7D%2C%22docid%22%3A5430742543006981120%2C%22publish_time%22%3A1783679601%7D%2C%22idx_range%22%3A%7B%22section_idx%22%3A7%2C%22begin_idx%22%3A5%2C%22end_idx%22%3A13%7D%2C%22expt_value%22%3A91%2C%22source%22%3A1024%2C%22needPreRender%22%3Afalse%7D",
					"s1s_jsapi_name": "openWXSearchHalfPage",
					"s1s_jsapi_paras": "{\"query\":\"Note-Plan\",\"scene\":139,\"hiddenSearchHeader\":0,\"webviewHeightRatio\":0.699999988,\"kvItems\":[{\"key\":\"mpEndHalfPageResultTab\",\"textValue\":\"0\"},{\"key\":\"firstSearchRequest\",\"uintValue\":1},{\"key\":\"MPHalfSearchAIBox\",\"uintValue\":3}],\"sessionKvItems\":[{\"key\":\"mpEndHalfPageResultTab\",\"textValue\":\"0\"},{\"key\":\"MPHalfSearchAIBox\",\"uintValue\":3}],\"parentType\":135,\"isAutoShowUnitInHalfScreen\":1}",
					"tags": []
				}
			],
			"exp_info": "CLXr9dEOEN3C168IGAEiEzU0MzA3NDI1NDMwMDY5ODEyMDgoicyGrdSd17mGAQ==",
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
			"like_count": 0,
			"old_like_count": 0,
			"share_count": 6,
			"comment_count": 0,
			"get_data_succ": 1,
			"collect_count": 0,
			"show_collect": 1,
			"show_collect_gray": 0,
			"read_num": 305,
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
		"reward_total_count": 0,
		"subcount_version": 3,
		"reward_half_panel": 1,
		"reward_info": {
			"timestamp": 1783693995,
			"rewardsn": "3089c944ac98e18d013c"
		},
		"listen_player_info": {
			"recommend_info_buffer": "IogBeyJtcF9zY2VuZSI6MTY5LCJtcF9zdWJfc2NlbmUiOjIwMCwiX19iaXoiOiJNemt5T1RnNU5qTTNNdz09IiwiYXBwbXNnX2lkIjoyMjQ3NDgzNzQxLCJpdGVtX2lkeCI6MSwibXBfZ2V0X2E4a2V5X3NjZW5lIjo3LCJ0cmFjZV9mbGFnIjowfSoeEAkaGgoOCLXr9dEOEN3C168IGAEQqQEYyAEgBygA"
		},
		"show_comment_entrance": 2,
		"share_h5info": {
			"underline_url": "https://mp.weixin.qq.com/mp/underline?action=get_appmsg_segment&clicktype=2&show_comment=1#wechat_redirect",
			"interaction_url": "https://mp.weixin.qq.com/mp/getinteraction?action=get_usertypelist&type=0&get_save=1&get_lastread=1&clicktype=2#wechat_redirect"
		},
		"short_link": "https://mp.weixin.qq.com/s/IS8lFXpYCq5sK_mqvhZnQA",
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
	"cdn_url_16_9": "",
	"real_item_show_type": 0,
	"url_item_show_type": 0,
	"video_page_infos": [],
	"can_use_wecoin": 1,
	"wecoin_tips": 0,
	"front_end_additional_fields": {
		"is_auto_type_setting": 3,
		"save_type": 0,
		"template_version": "60326389"
	},
	"open_fansmsg": 0,
	"is_cooling_appmsg": 0,
	"ip_wording": {
		"country_name": "中国",
		"country_id": "156",
		"province_name": "山西"
	},
	"show_ip_wording": 1,
	"is_acct_area_shield": 0,
	"shield_acct_areaids": [],
	"style_type": 3,
	"shield_areas_info": [],
	"create_timestamp": 1783679631,
	"picture_list_in_pictext": [],
	"servicetype": 0,
	"segment_comment_id": "4598422356225998853",
	"ad_mark_status": 0,
	"hide_ad_mark_on_cps": 0,
	"finder_audio_card": "{\"list\":[]}",
	"claim_source": {
		"is_user_no_claim_source": 1
	},
	"extra_comment_id": "4598422355571687424",
	"last_text": [],
	"wash_status": 0,
	"enterid": 1783693993,
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
	"watermark_setting": 2,
	"title_gen_type": 0,
	"appmsg_listen_id": "150445275645906635",
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

func TestArticleToProfile_FromArticleJSON(t *testing.T) {
	var pageJSON officialaccount.CgiDataNew
	if err := json.Unmarshal([]byte(articleJSON), &pageJSON); err != nil {
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
	if profile.Title != "一个重新定义的待办软件" {
		t.Errorf("Title = %q, want %q", profile.Title, "一个重新定义的待办软件")
	}
	if profile.Description != "计划总赶不上变化？一个重新定义的待办软件" {
		t.Errorf("Description = %q", profile.Description)
	}
	if profile.SourceURL != pageJSON.Link {
		t.Errorf("SourceURL = %q, want %q", profile.SourceURL, pageJSON.Link)
	}
	if profile.CoverURL == "" {
		t.Error("CoverURL should not be empty")
	}
	if profile.PublishTime != 1783679631 {
		t.Errorf("PublishTime = %d, want 1783679631", profile.PublishTime)
	}
	if profile.Author.ExternalId != "gh_8843e49ff5fa" {
		t.Errorf("Author.ExternalId = %q, want %q", profile.Author.ExternalId, "gh_8843e49ff5fa")
	}
	if profile.Author.Nickname != "阿维AI实验室" {
		t.Errorf("Author.Nickname = %q, want %q", profile.Author.Nickname, "阿维AI实验室")
	}
	if profile.Author.AvatarURL == "" {
		t.Error("Author.AvatarURL should not be empty")
	}
}
