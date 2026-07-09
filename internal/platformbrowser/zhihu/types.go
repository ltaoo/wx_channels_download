package zhihu

import "wx_channel/internal/interceptor"

type Profile = interceptor.PlatformBrowserProfile

type RawPayload struct {
	ZhihuContentKind        string `json:"zhihu_content_kind"`
	ZhihuContentToken       string `json:"zhihu_content_token"`
	ZhihuQuestionToken      string `json:"zhihu_question_token"`
	ZhihuQuestionURL        string `json:"zhihu_question_url"`
	ZhihuAuthorMemberHashID string `json:"zhihu_author_member_hash_id"`
	ZhihuDateCreated        string `json:"zhihu_date_created"`
	ZhihuDateModified       string `json:"zhihu_date_modified"`
	ZhihuUpvoteNum          int    `json:"zhihu_upvote_num"`
	ZhihuCommentNum         int    `json:"zhihu_comment_num"`
	ZhihuFeedID             string `json:"zhihu_feed_id"`
}
