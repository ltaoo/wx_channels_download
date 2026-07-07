export {};

declare global {
  type KnownContentPlatformID =
    | "douyin"
    | "zhihu"
    | "bilibili"
    | "xiaohongshu"
    | "officialaccount"
    | "weibo"
    | "wx_channels";

  type ContentPlatformID = KnownContentPlatformID | (string & {});

  type KnownProbeContentType =
    | "video"
    | "image_album"
    | "note"
    | "post"
    | "article"
    | "question"
    | "answer"
    | "live"
    | "collection"
    | "account"
    | "topic";

  type ProbeContentType = KnownProbeContentType | (string & {});

  type ProbeContentVisibility =
    | "public"
    | "private"
    | "deleted"
    | "restricted"
    | "login_required"
    | (string & {});

  type ProbeTimestamp = number | string;

  type ProbeContentStats = {
    view_count?: number;
    play_count?: number;
    like_count?: number;
    comment_count?: number;
    share_count?: number;
    collect_count?: number;
    danmaku_count?: number;
    repost_count?: number;
    [key: string]: unknown;
  };

  type ProbeContentAuthor = {
    id?: string;
    username?: string;
    nickname?: string;
    avatar_url?: string;
    profile_url?: string;
    verified?: boolean | string;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeContentMention = {
    id?: string;
    username?: string;
    nickname?: string;
    url?: string;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeContentTopicRef = {
    id?: string;
    name?: string;
    url?: string;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeContentLocation =
    | string
    | {
        id?: string;
        name?: string;
        address?: string;
        latitude?: number;
        longitude?: number;
        raw?: unknown;
        [key: string]: unknown;
      };

  type ProbeImageResource = {
    id?: string;
    url?: string;
    thumbnail_url?: string;
    width?: number;
    height?: number;
    format?: string;
    is_gif?: boolean;
    caption?: string;
    ocr_text?: string;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeVideoResource = {
    id?: string;
    url?: string;
    cover_url?: string;
    duration?: number;
    width?: number;
    height?: number;
    fps?: number;
    bitrate?: number;
    format?: string;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeAudioResource = {
    id?: string;
    url?: string;
    title?: string;
    duration?: number;
    bitrate?: number;
    format?: string;
    sample_rate?: number;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeLinkCard = {
    id?: string;
    url?: string;
    title?: string;
    description?: string;
    cover_url?: string;
    site_name?: string;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeProductCard = {
    id?: string;
    title?: string;
    url?: string;
    cover_url?: string;
    price?: number | string;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeSubtitleInfo = {
    language?: string;
    title?: string;
    url?: string;
    format?: string;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeChapter = {
    title?: string;
    start_time?: number;
    end_time?: number;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeMusicInfo = {
    id?: string;
    title?: string;
    author?: string;
    url?: string;
    cover_url?: string;
    duration?: number;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeBaseContent = {
    platform?: ContentPlatformID;
    content_type?: ProbeContentType;
    content_id?: string;
    canonical_url?: string;
    source_url?: string;
    title?: string;
    description?: string;
    text?: string;
    cover_url?: string;
    author?: ProbeContentAuthor | string;
    publish_time?: ProbeTimestamp;
    update_time?: ProbeTimestamp;
    stats?: ProbeContentStats;
    tags?: string[];
    topics?: Array<string | ProbeContentTopicRef>;
    mentions?: Array<string | ProbeContentMention>;
    location?: ProbeContentLocation;
    visibility?: ProbeContentVisibility;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ProbeVideoContent = ProbeBaseContent & {
    content_type: "video";
    duration?: number;
    width?: number;
    height?: number;
    fps?: number;
    bitrate?: number;
    format?: string;
    preview_images?: Array<string | ProbeImageResource>;
    subtitle_info?: ProbeSubtitleInfo | ProbeSubtitleInfo[];
    chapters?: ProbeChapter[];
    music?: ProbeMusicInfo;
    is_original?: boolean;
    is_repost?: boolean;
  };

  type ProbeImageAlbumContent = ProbeBaseContent & {
    content_type: "image_album";
    images?: ProbeImageResource[];
    image_count?: number;
    cover_index?: number;
    width?: number;
    height?: number;
    format?: string;
    is_gif?: boolean;
    ocr_text?: string;
    captions?: string[];
  };

  type ProbeNoteContent = ProbeBaseContent & {
    content_type: "note";
    note_id?: string;
    note_type?: "image" | "video" | (string & {});
    title?: string;
    text?: string;
    images?: ProbeImageResource[];
    video?: ProbeVideoResource;
    cover_url?: string;
    tags?: string[];
    location?: ProbeContentLocation;
    product_cards?: ProbeProductCard[];
    collect_count?: number;
  };

  type ProbePostContent = ProbeBaseContent & {
    content_type: "post";
    post_id?: string;
    text?: string;
    rich_text?: string;
    images?: ProbeImageResource[];
    video?: ProbeVideoResource;
    link_cards?: ProbeLinkCard[];
    topics?: Array<string | ProbeContentTopicRef>;
    mentions?: Array<string | ProbeContentMention>;
    repost_of?: ProbePostContent;
    quote_of?: ProbePostContent;
    comment_count?: number;
    repost_count?: number;
  };

  type ProbeArticleContent = ProbeBaseContent & {
    content_type: "article";
    article_id?: string;
    title?: string;
    author?: ProbeContentAuthor | string;
    digest?: string;
    body_html?: string;
    body_text?: string;
    cover_url?: string;
    images?: ProbeImageResource[];
    embedded_media?: Array<
      ProbeImageResource | ProbeVideoResource | ProbeAudioResource
    >;
    word_count?: number;
    reading_time?: number;
    source_url?: string;
    copyright_info?: string | Record<string, unknown>;
  };

  type ProbeQuestionContent = ProbeBaseContent & {
    content_type: "question";
    question_id?: string;
    title?: string;
    detail?: string;
    topics?: Array<string | ProbeContentTopicRef>;
    answer_count?: number;
    follower_count?: number;
    best_answers?: ProbeAnswerContent[];
    created_time?: ProbeTimestamp;
    updated_time?: ProbeTimestamp;
  };

  type ProbeAnswerContent = ProbeBaseContent & {
    content_type: "answer";
    answer_id?: string;
    question_id?: string;
    question_title?: string;
    author?: ProbeContentAuthor;
    body_html?: string;
    body_text?: string;
    excerpt?: string;
    images?: ProbeImageResource[];
    video?: ProbeVideoResource;
    vote_count?: number;
    comment_count?: number;
    created_time?: ProbeTimestamp;
    updated_time?: ProbeTimestamp;
  };

  type ProbeLiveContent = ProbeBaseContent & {
    content_type: "live";
    room_id?: string;
    live_id?: string;
    title?: string;
    status?: "live" | "ended" | "scheduled" | (string & {});
    anchor?: ProbeContentAuthor;
    cover_url?: string;
    start_time?: ProbeTimestamp;
    end_time?: ProbeTimestamp;
    viewer_count?: number;
    reservation_count?: number;
    products?: ProbeProductCard[];
    replay_info?: Record<string, unknown>;
  };

  type ProbeCollectionContent = ProbeBaseContent & {
    content_type: "collection";
    collection_id?: string;
    title?: string;
    description?: string;
    owner?: ProbeContentAuthor;
    cover_url?: string;
    item_count?: number;
    items?: PlatformProbeContent[];
    updated_time?: ProbeTimestamp;
  };

  type ProbeAccountContent = ProbeBaseContent & {
    content_type: "account";
    account_id?: string;
    username?: string;
    nickname?: string;
    avatar_url?: string;
    bio?: string;
    verified?: boolean | string;
    follower_count?: number;
    following_count?: number;
    content_count?: number;
    profile_url?: string;
  };

  type ProbeTopicContent = ProbeBaseContent & {
    content_type: "topic";
    topic_id?: string;
    name?: string;
    description?: string;
    cover_url?: string;
    follower_count?: number;
    view_count?: number;
    post_count?: number;
    related_topics?: ProbeContentTopicRef[];
  };

  type ProbeUnknownContent = ProbeBaseContent & {
    content_type?: Exclude<ProbeContentType, KnownProbeContentType>;
  };

  type PlatformProbeContent =
    | ProbeVideoContent
    | ProbeImageAlbumContent
    | ProbeNoteContent
    | ProbePostContent
    | ProbeArticleContent
    | ProbeQuestionContent
    | ProbeAnswerContent
    | ProbeLiveContent
    | ProbeCollectionContent
    | ProbeAccountContent
    | ProbeTopicContent
    | ProbeUnknownContent;

  type ProbeRelatedResource = {
    type?: ProbeContentType | "image" | "audio" | (string & {});
    id?: string;
    url?: string;
    title?: string;
    description?: string;
    cover_url?: string;
    duration?: number;
    metadata?: Record<string, unknown>;
    raw?: unknown;
    [key: string]: unknown;
  };

  type ContentProbe = {
    platform?: ContentPlatformID;
    content_type?: ProbeContentType;
    content?: PlatformProbeContent;
    related_resources?: ProbeRelatedResource[];
    warnings?: string[];
    raw?: unknown;
    [key: string]: unknown;
  };

  type ContentProbeResponse = {
    run_id?: string;
    probe_id?: string;
    probe?: ContentProbe;
    content?: PlatformProbeContent;
    related_resources?: ProbeRelatedResource[];
    existing?: unknown[];
    form?: unknown[];
    output?: Record<string, unknown>;
    workflow?: unknown;
    raw?: unknown;
    [key: string]: unknown;
  };
}
