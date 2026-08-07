package zhihu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// InitialData is the JSON payload embedded in Zhihu pages as
// <script id="js-initialData" type="text/json">.
//
// Zhihu changes this SSR payload frequently. The stable content entities are
// modeled below, while Raw preserves the complete original JSON for callers
// that need fields outside the typed projection.
type InitialData struct {
	InitialState InitialState      `json:"initialState"`
	SubAppName   string            `json:"subAppName"`
	SpanName     string            `json:"spanName"`
	CanaryConfig map[string]string `json:"canaryConfig"`
	Raw          json.RawMessage   `json:"-"`
}

type InitialState struct {
	Common      InitialCommon   `json:"common"`
	Loading     InitialLoading  `json:"loading"`
	Entities    InitialEntities `json:"entities"`
	CurrentUser string          `json:"currentUser"`
	Account     json.RawMessage `json:"account,omitempty"`
	Settings    json.RawMessage `json:"settings,omitempty"`
	People      json.RawMessage `json:"people,omitempty"`
	Env         json.RawMessage `json:"env,omitempty"`
	Question    json.RawMessage `json:"question,omitempty"`
	Answers     json.RawMessage `json:"answers,omitempty"`
	Topic       json.RawMessage `json:"topic,omitempty"`
	Articles    json.RawMessage `json:"articles,omitempty"`
	Topstory    json.RawMessage `json:"topstory,omitempty"`
	Search      json.RawMessage `json:"search,omitempty"`
}

type InitialCommon struct {
	Ask    json.RawMessage `json:"ask,omitempty"`
	Cities struct {
		CityData []json.RawMessage `json:"cityData"`
	} `json:"cities"`
}

type InitialLoading struct {
	Global struct {
		Count int `json:"count"`
	} `json:"global"`
	Local map[string]bool `json:"local"`
}

type InitialEntities struct {
	Users         map[string]User            `json:"users"`
	Questions     map[string]Question        `json:"questions"`
	Answers       map[string]Answer          `json:"answers"`
	Articles      map[string]Article         `json:"articles"`
	Columns       map[string]json.RawMessage `json:"columns"`
	Topics        map[string]json.RawMessage `json:"topics"`
	Roundtables   map[string]json.RawMessage `json:"roundtables"`
	Favlists      map[string]json.RawMessage `json:"favlists"`
	Comments      map[string]json.RawMessage `json:"comments"`
	Notifications map[string]json.RawMessage `json:"notifications"`
	Ebooks        map[string]json.RawMessage `json:"ebooks"`
	Activities    map[string]json.RawMessage `json:"activities"`
	Feeds         map[string]json.RawMessage `json:"feeds"`
	Pins          map[string]json.RawMessage `json:"pins"`
	Promotions    map[string]json.RawMessage `json:"promotions"`
	Drafts        map[string]json.RawMessage `json:"drafts"`
	Chats         map[string]json.RawMessage `json:"chats"`
	Posts         map[string]Article         `json:"posts"`
	Zvideos       map[string]json.RawMessage `json:"zvideos"`
	EduCourses    map[string]json.RawMessage `json:"eduCourses"`
	LineComments  map[string]json.RawMessage `json:"lineComments"`
	Projects      map[string]json.RawMessage `json:"projects"`
}

type QuestionRef struct {
	Created      int64                      `json:"created"`
	ID           string                     `json:"id"`
	QuestionType string                     `json:"questionType"`
	Relationship map[string]json.RawMessage `json:"relationship"`
	Title        string                     `json:"title"`
	Type         string                     `json:"type"`
	UpdatedTime  int64                      `json:"updatedTime"`
	URL          string                     `json:"url"`
}

type BadgeV2 struct {
	Title        string            `json:"title"`
	MergedBadges []json.RawMessage `json:"mergedBadges"`
	DetailBadges []json.RawMessage `json:"detailBadges"`
	Icon         string            `json:"icon"`
	NightIcon    string            `json:"nightIcon"`
}

type VIPInfo struct {
	IsVIP           bool             `json:"isVip"`
	VIPType         int              `json:"vipType"`
	RenameDays      string           `json:"renameDays"`
	EntranceV2      *json.RawMessage `json:"entranceV2"`
	RenameFrequency int              `json:"renameFrequency"`
	RenameAwaitDays int              `json:"renameAwaitDays"`
	VIPIcon         VIPIcon          `json:"vipIcon"`
}

type VIPIcon struct {
	NightModeURL string `json:"nightModeUrl"`
	URL          string `json:"url"`
}

type Topic struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	TopicType string `json:"topicType"`
}

type CanComment struct {
	Status bool   `json:"status"`
	Reason string `json:"reason"`
}

type QuestionStatus struct {
	IsLocked   bool `json:"isLocked"`
	IsClose    bool `json:"isClose"`
	IsEvaluate bool `json:"isEvaluate"`
	IsSuggest  bool `json:"isSuggest"`
}

type ThumbnailInfo struct {
	Count      int               `json:"count"`
	Type       string            `json:"type"`
	Thumbnails []json.RawMessage `json:"thumbnails"`
}

type ReviewInfo struct {
	Type            string `json:"type"`
	Tips            string `json:"tips"`
	EditTips        string `json:"editTips"`
	IsReviewing     bool   `json:"isReviewing"`
	EditIsReviewing bool   `json:"editIsReviewing"`
}

type MuteInfo struct {
	Type string `json:"type"`
}

type QuestionRelationship struct {
	IsAuthor           bool `json:"isAuthor"`
	IsFollowing        bool `json:"isFollowing"`
	IsAnonymous        bool `json:"isAnonymous"`
	CanLock            bool `json:"canLock"`
	CanStickAnswers    bool `json:"canStickAnswers"`
	CanCollapseAnswers bool `json:"canCollapseAnswers"`
	Voting             int  `json:"voting"`
}

type AnswerBizExt struct {
	ShareGuide struct {
		HasPositiveBubble    bool `json:"hasPositiveBubble"`
		HasTimeBubble        bool `json:"hasTimeBubble"`
		HitShareGuideCluster bool `json:"hitShareGuideCluster"`
	} `json:"shareGuide"`
}

type PodcastAudioEnter struct {
	ActionURL string `json:"actionUrl"`
	SubType   string `json:"subType"`
	Text      string `json:"text"`
	TextColor string `json:"textColor"`
	TextSize  int    `json:"textSize"`
}

type AnswerReaction struct {
	ImageReactions map[string]json.RawMessage `json:"imageReactions"`
	Relation       struct {
		CurrentUserIsNavigator bool   `json:"currentUserIsNavigator"`
		Faved                  bool   `json:"faved"`
		Following              bool   `json:"following"`
		IsAuthor               bool   `json:"isAuthor"`
		IsNavigatorVote        bool   `json:"isNavigatorVote"`
		Liked                  bool   `json:"liked"`
		Subcribed              bool   `json:"subcribed"`
		Vote                   string `json:"vote"`
		VoteNextStep           string `json:"voteNextStep"`
	} `json:"relation"`
	Statistics struct {
		ApplaudCount            int               `json:"applaudCount"`
		BulletCount             int               `json:"bulletCount"`
		CommentCount            int               `json:"commentCount"`
		DownVoteCount           int               `json:"downVoteCount"`
		Favorites               int               `json:"favorites"`
		InterestPlayCount       int               `json:"interestPlayCount"`
		LikeCount               int               `json:"likeCount"`
		PlaincontentLikeCount   int               `json:"plaincontentLikeCount"`
		PlaincontentVoteUpCount int               `json:"plaincontentVoteUpCount"`
		PlayCount               int               `json:"playCount"`
		PVCount                 int               `json:"pvCount"`
		QuestionAnswerCount     int               `json:"questionAnswerCount"`
		QuestionFollowerCount   int               `json:"questionFollowerCount"`
		Republishers            []json.RawMessage `json:"republishers"`
		ShareCount              int               `json:"shareCount"`
		SubscribeCount          int               `json:"subscribeCount"`
		UpVoteCount             int               `json:"upVoteCount"`
	} `json:"statistics"`
}

type AnswerRelationship struct {
	IsAuthor         bool              `json:"isAuthor"`
	IsAuthorized     bool              `json:"isAuthorized"`
	IsFavorited      bool              `json:"isFavorited"`
	IsNothelp        bool              `json:"isNothelp"`
	IsThanked        bool              `json:"isThanked"`
	UpvotedFollowees []json.RawMessage `json:"upvotedFollowees"`
	Voting           int               `json:"voting"`
}

type RelevantInfo struct {
	IsRelevant   bool   `json:"isRelevant"`
	RelevantText string `json:"relevantText"`
	RelevantType string `json:"relevantType"`
}

type RewardInfo struct {
	CanOpenReward     bool   `json:"canOpenReward"`
	IsRewardable      bool   `json:"isRewardable"`
	RewardMemberCount int    `json:"rewardMemberCount"`
	RewardTotalMoney  int    `json:"rewardTotalMoney"`
	Tagline           string `json:"tagline"`
}

type SuggestEdit struct {
	Reason          string `json:"reason"`
	Status          bool   `json:"status"`
	Tip             string `json:"tip"`
	Title           string `json:"title"`
	UnnormalDetails struct {
		Description string `json:"description"`
		Note        string `json:"note"`
		Reason      string `json:"reason"`
		ReasonID    int    `json:"reasonId"`
		Status      string `json:"status"`
	} `json:"unnormalDetails"`
	URL string `json:"url"`
}

func ExtractInitialDataJSON(body []byte) (json.RawMessage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(doc.Find("script#js-initialData").First().Text())
	if raw == "" {
		return nil, fmt.Errorf("missing zhihu initial data")
	}
	if !json.Valid([]byte(raw)) {
		var payload any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return nil, fmt.Errorf("invalid zhihu initial data json: %w", err)
		}
		return nil, fmt.Errorf("invalid zhihu initial data json")
	}
	return json.RawMessage(append([]byte(nil), raw...)), nil
}

func ParseInitialData(body []byte) (*InitialData, error) {
	raw, err := ExtractInitialDataJSON(body)
	if err != nil {
		return nil, err
	}
	var data InitialData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	data.Raw = raw
	return &data, nil
}
