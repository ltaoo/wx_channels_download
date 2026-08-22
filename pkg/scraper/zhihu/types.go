package zhihu

import (
	"encoding/json"
)

type AnswerURL struct {
	QuestionID string
	AnswerID   string
	Canonical  string
}

type QuestionURL struct {
	QuestionID string
	Canonical  string
}

type ArticleURL struct {
	ArticleID string
	Canonical string
}

type User struct {
	ID                string            `json:"id"`
	URL               string            `json:"url"`
	URLToken          string            `json:"urlToken"`
	URLTokenSnake     string            `json:"url_token"`
	Name              string            `json:"name"`
	Headline          string            `json:"headline"`
	HeadlineRender    string            `json:"headlineRender"`
	AvatarURL         string            `json:"avatarUrl"`
	AvatarURLSnake    string            `json:"avatar_url"`
	AvatarURLTemplate string            `json:"avatarUrlTemplate"`
	UseDefaultAvatar  bool              `json:"useDefaultAvatar"`
	IsOrg             bool              `json:"isOrg"`
	Type              string            `json:"type"`
	UserType          string            `json:"userType"`
	Badge             []json.RawMessage `json:"badge"`
	BadgeV2           BadgeV2           `json:"badgeV2"`
	Gender            int               `json:"gender"`
	IsAdvertiser      bool              `json:"isAdvertiser"`
	IsPrivacy         bool              `json:"isPrivacy"`
	IsFollowed        bool              `json:"isFollowed"`
	IPInfo            string            `json:"ipInfo"`
	VIPInfo           VIPInfo           `json:"vipInfo"`
}

type Question struct {
	ID                     string                     `json:"id"`
	Type                   string                     `json:"type"`
	Title                  string                     `json:"title"`
	QuestionType           string                     `json:"questionType"`
	Created                int64                      `json:"created"`
	UpdatedTime            int64                      `json:"updatedTime"`
	URL                    string                     `json:"url"`
	IsMuted                bool                       `json:"isMuted"`
	IsVisible              bool                       `json:"isVisible"`
	IsNormal               bool                       `json:"isNormal"`
	IsEditable             bool                       `json:"isEditable"`
	AdminClosedComment     bool                       `json:"adminClosedComment"`
	HasPublishingDraft     bool                       `json:"hasPublishingDraft"`
	AnswerCount            int                        `json:"answerCount"`
	VisitCount             int                        `json:"visitCount"`
	CommentCount           int                        `json:"commentCount"`
	FollowerCount          int                        `json:"followerCount"`
	CollapsedAnswerCount   int                        `json:"collapsedAnswerCount"`
	Excerpt                string                     `json:"excerpt"`
	CommentPermission      string                     `json:"commentPermission"`
	Detail                 string                     `json:"detail"`
	EditableDetail         string                     `json:"editableDetail"`
	Status                 QuestionStatus             `json:"status"`
	Topics                 []Topic                    `json:"topics"`
	Author                 User                       `json:"author"`
	CanComment             CanComment                 `json:"canComment"`
	ThumbnailInfo          ThumbnailInfo              `json:"thumbnailInfo"`
	ReviewInfo             ReviewInfo                 `json:"reviewInfo"`
	RelatedCards           []json.RawMessage          `json:"relatedCards"`
	MuteInfo               MuteInfo                   `json:"muteInfo"`
	ShowAuthor             bool                       `json:"showAuthor"`
	IsLabeled              bool                       `json:"isLabeled"`
	IsBannered             bool                       `json:"isBannered"`
	ShowEncourageAuthor    bool                       `json:"showEncourageAuthor"`
	VoteupCount            int                        `json:"voteupCount"`
	CanVote                bool                       `json:"canVote"`
	ReactionInstruction    map[string]json.RawMessage `json:"reactionInstruction"`
	InvisibleAuthor        bool                       `json:"invisibleAuthor"`
	AnswerCountDescription string                     `json:"answerCountDescription"`
	Relationship           QuestionRelationship       `json:"relationship"`
}

type Answer struct {
	ID                          string                     `json:"id"`
	Type                        string                     `json:"type"`
	AdminClosedComment          bool                       `json:"adminClosedComment"`
	AllowSegmentInteraction     int                        `json:"allowSegmentInteraction"`
	AnnotationAction            json.RawMessage            `json:"annotationAction"`
	AnswerType                  string                     `json:"answerType"`
	Author                      User                       `json:"author"`
	BizExt                      AnswerBizExt               `json:"bizExt"`
	CanComment                  CanComment                 `json:"canComment"`
	CollapseReason              string                     `json:"collapseReason"`
	CollapsedBy                 string                     `json:"collapsedBy"`
	CommentCount                int                        `json:"commentCount"`
	CommentPermission           string                     `json:"commentPermission"`
	Content                     string                     `json:"content"`
	ContentNeedTruncated        bool                       `json:"contentNeedTruncated"`
	CreatedTime                 int64                      `json:"createdTime"`
	EditableContent             string                     `json:"editableContent"`
	Excerpt                     string                     `json:"excerpt"`
	Extras                      string                     `json:"extras"`
	FavlistsCount               int                        `json:"favlistsCount"`
	ForceLoginWhenClickReadMore bool                       `json:"forceLoginWhenClickReadMore"`
	HasColumn                   bool                       `json:"hasColumn"`
	IPInfo                      string                     `json:"ipInfo"`
	IsCollapsed                 bool                       `json:"isCollapsed"`
	IsCopyable                  bool                       `json:"isCopyable"`
	IsJumpNative                bool                       `json:"isJumpNative"`
	IsLabeled                   bool                       `json:"isLabeled"`
	IsNavigator                 bool                       `json:"isNavigator"`
	IsNormal                    bool                       `json:"isNormal"`
	IsSticky                    bool                       `json:"isSticky"`
	IsVisible                   bool                       `json:"isVisible"`
	NavigatorVote               bool                       `json:"navigatorVote"`
	PodcastAudioEnter           PodcastAudioEnter          `json:"podcastAudioEnter"`
	Question                    QuestionRef                `json:"question"`
	Reaction                    AnswerReaction             `json:"reaction"`
	ReactionInstruction         map[string]json.RawMessage `json:"reactionInstruction"`
	Relationship                AnswerRelationship         `json:"relationship"`
	RelevantInfo                RelevantInfo               `json:"relevantInfo"`
	ReshipmentSettings          string                     `json:"reshipmentSettings"`
	RewardInfo                  RewardInfo                 `json:"rewardInfo"`
	SuggestEdit                 SuggestEdit                `json:"suggestEdit"`
	ThanksCount                 int                        `json:"thanksCount"`
	UpdatedTime                 int64                      `json:"updatedTime"`
	URL                         string                     `json:"url"`
	VoteNextStep                string                     `json:"voteNextStep"`
	VoteupCount                 int                        `json:"voteupCount"`
}

type Article struct {
	ID                          string `json:"id"`
	Title                       string `json:"title"`
	Content                     string `json:"content"`
	ContentNeedTruncated        bool   `json:"contentNeedTruncated"`
	ForceLoginWhenClickReadMore bool   `json:"forceLoginWhenClickReadMore"`
	Excerpt                     string `json:"excerpt"`
	ImageURL                    string `json:"imageUrl"`
	ImageURLAlt                 string `json:"image_url"`
	Author                      User   `json:"author"`
	CreatedTime                 int64  `json:"created"`
	UpdatedTime                 int64  `json:"updated"`
}

type Comment struct {
	ID          string
	ContentHTML string
	ContentText string
	CreatedTime int64
	Author      User
	ReplyTo     *User
	Replies     []Comment
}

type AnswerPage struct {
	URL             AnswerURL
	Source          string
	PageHTML        string
	Question        Question
	Answer          Answer
	Comments        []Comment
	InitialData     *InitialData
	InitialDataJSON json.RawMessage
}

type QuestionPage struct {
	URL             QuestionURL
	Source          string
	PageHTML        string
	Question        Question
	InitialData     *InitialData
	InitialDataJSON json.RawMessage
}

type ArticlePage struct {
	URL             ArticleURL
	Source          string
	PageHTML        string
	Article         Article
	InitialData     *InitialData
	InitialDataJSON json.RawMessage
}

type comment_payload struct {
	ID         json.RawMessage   `json:"id"`
	Content    string            `json:"content"`
	ContentTag string            `json:"content_tag"`
	Created    int64             `json:"created_time"`
	CreatedAt  int64             `json:"createdAt"`
	Author     User              `json:"author"`
	ReplyTo    *User             `json:"reply_to_author"`
	Child      []comment_payload `json:"child_comments"`
}

type comment_response struct {
	Data   []comment_payload `json:"data"`
	Paging comment_paging    `json:"paging"`
}

type comment_paging struct {
	IsEnd bool   `json:"is_end"`
	Next  string `json:"next"`
}

// RecommendFeed represents an item returned by Zhihu's recommendation feed.
type RecommendFeed struct {
	ID           string              `json:"id"`
	Type         string              `json:"type"`
	Offset       int                 `json:"offset"`
	Verb         string              `json:"verb"`
	CreatedTime  int64               `json:"created_time"`
	UpdatedTime  int64               `json:"updated_time"`
	Target       RecommendFeedTarget `json:"target"`
	Brief        string              `json:"brief"`
	AttachedInfo string              `json:"attached_info"`
	ActionCard   bool                `json:"action_card"`
}

type RecommendFeedTarget struct {
	Created                 *int64                     `json:"created,omitempty"`
	CommentCount            int                        `json:"comment_count"`
	Column                  *RecommendFeedColumn       `json:"column,omitempty"`
	Content                 string                     `json:"content"`
	IsLabeled               bool                       `json:"is_labeled"`
	Updated                 *int64                     `json:"updated,omitempty"`
	VisitedCount            int                        `json:"visited_count"`
	Type                    string                     `json:"type"`
	ArticleType             *string                    `json:"article_type,omitempty"`
	Endorsements            []RecommendFeedEndorsement `json:"endorsements,omitempty"`
	CommentPermission       *string                    `json:"comment_permission,omitempty"`
	VoteupCount             int                        `json:"voteup_count"`
	PreviewText             string                     `json:"preview_text"`
	Author                  RecommendFeedAuthor        `json:"author"`
	FavoriteCount           int                        `json:"favorite_count"`
	IsNavigator             bool                       `json:"is_navigator"`
	VoteNextStep            string                     `json:"vote_next_step"`
	AllowSegmentInteraction bool                       `json:"allow_segment_interaction"`
	Voting                  *int                       `json:"voting,omitempty"`
	Linkbox                 *RecommendFeedLinkbox      `json:"linkbox,omitempty"`
	ExcerptNew              string                     `json:"excerpt_new"`
	PreviewType             string                     `json:"preview_type"`
	Thumbnails              []string                   `json:"thumbnails,omitempty"`
	ID                      string                     `json:"id"`
	Excerpt                 string                     `json:"excerpt"`
	Reaction                RecommendFeedReaction      `json:"reaction"`
	URL                     string                     `json:"url"`
	Title                   *string                    `json:"title,omitempty"`
	NavigatorVote           bool                       `json:"navigator_vote"`
	ThanksCount             *int                       `json:"thanks_count,omitempty"`
	IsCopyable              *bool                      `json:"is_copyable,omitempty"`
	Relationship            *RecommendFeedRelationship `json:"relationship,omitempty"`
	Thumbnail               *string                    `json:"thumbnail,omitempty"`
	UpdatedTime             *int64                     `json:"updated_time,omitempty"`
	Question                *RecommendFeedQuestion     `json:"question,omitempty"`
	ReshipmentSettings      *string                    `json:"reshipment_settings,omitempty"`
	AnswerType              *string                    `json:"answer_type,omitempty"`
	CreatedTime             *int64                     `json:"created_time,omitempty"`
	CreationDisclaimer      *string                    `json:"creation_disclaimer,omitempty"`
	ImageURL                *string                    `json:"image_url,omitempty"`
}

type RecommendFeedColumn struct {
	Type              string              `json:"type"`
	Author            RecommendFeedAuthor `json:"author"`
	Title             string              `json:"title"`
	Intro             string              `json:"intro"`
	Updated           int64               `json:"updated"`
	IsFollowing       bool                `json:"is_following"`
	ID                string              `json:"id"`
	ImageURL          string              `json:"imageUrl"`
	CommentPermission string              `json:"comment_permission"`
	URL               string              `json:"url"`
}

type RecommendFeedAuthor struct {
	IsFollowing    bool                 `json:"is_following"`
	IsFollowed     bool                 `json:"is_followed"`
	URLToken       string               `json:"url_token"`
	Headline       string               `json:"headline"`
	Gender         int                  `json:"gender"`
	FollowersCount int                  `json:"followers_count"`
	AvatarURL      string               `json:"avatar_url"`
	IsOrg          bool                 `json:"is_org"`
	Badge          []RecommendFeedBadge `json:"badge,omitempty"`
	ID             string               `json:"id"`
	URL            string               `json:"url"`
	UserType       string               `json:"user_type"`
	Name           string               `json:"name"`
}

type RecommendFeedBadge struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type RecommendFeedEndorsement struct {
	Elements        []RecommendFeedEndorsementElement `json:"elements"`
	SubElements     []any                             `json:"sub_elements"`
	SubElementsType string                            `json:"sub_elements_type"`
	BackgroundColor RecommendFeedColor                `json:"background_color"`
	ActionURL       string                            `json:"action_url"`
	ZA              RecommendFeedEndorsementZA        `json:"za"`
}

type RecommendFeedEndorsementElement struct {
	ImageColor *RecommendFeedColor `json:"image_color,omitempty"`
	Width      *int                `json:"width,omitempty"`
	Height     *int                `json:"height,omitempty"`
	Type       string              `json:"type"`
	ImageKey   *string             `json:"image_key,omitempty"`
	Content    *string             `json:"content,omitempty"`
	FontSize   *int                `json:"font_size,omitempty"`
	FontColor  *RecommendFeedColor `json:"font_color,omitempty"`
	IsBold     *bool               `json:"is_bold,omitempty"`
	MaxLine    *int                `json:"max_line,omitempty"`
}

type RecommendFeedColor struct {
	Alpha float64 `json:"alpha"`
	Group string  `json:"group"`
}

type RecommendFeedEndorsementZA struct {
	BlockText string `json:"block_text"`
	Type      string `json:"type"`
	Text      string `json:"text"`
}

type RecommendFeedLinkbox struct {
	Category string `json:"category"`
	Pic      string `json:"pic"`
	Title    string `json:"title"`
	URL      string `json:"url"`
}

type RecommendFeedReaction struct {
	Relation   RecommendFeedReactionRelation   `json:"relation"`
	Statistics RecommendFeedReactionStatistics `json:"statistics"`
}

type RecommendFeedReactionRelation struct {
	Liked bool `json:"liked"`
	Faved bool `json:"faved"`
}

type RecommendFeedReactionStatistics struct {
	LikeCount int `json:"like_count"`
	Favorites int `json:"favorites"`
}

type RecommendFeedRelationship struct {
	IsNothelp bool `json:"is_nothelp"`
	Voting    int  `json:"voting"`
	IsThanked bool `json:"is_thanked"`
}

type RecommendFeedQuestion struct {
	Title         string                            `json:"title"`
	Excerpt       string                            `json:"excerpt"`
	Relationship  RecommendFeedQuestionRelationship `json:"relationship"`
	Detail        string                            `json:"detail"`
	Type          string                            `json:"type"`
	URL           string                            `json:"url"`
	Author        RecommendFeedAuthor               `json:"author"`
	FollowerCount int                               `json:"follower_count"`
	CommentCount  int                               `json:"comment_count"`
	ID            string                            `json:"id"`
	Created       int64                             `json:"created"`
	BoundTopicIDs []int64                           `json:"bound_topic_ids"`
	QuestionType  string                            `json:"question_type"`
	AnswerCount   int                               `json:"answer_count"`
	IsFollowing   bool                              `json:"is_following"`
}

type RecommendFeedQuestionRelationship struct {
	IsAuthor bool `json:"is_author"`
}
