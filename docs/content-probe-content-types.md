# Probe 内容类型抽象

本文只描述 Probe 阶段从 URL 请求到的“平台内容实体”。Probe 阶段不描述最终下载产物，因为此时还没有经过用户选择，也不能确定最终会保存成视频、HTML、压缩包或其他文件。

## 核心原则

`content_type` 描述平台内容形态，不描述文件格式。

例如：

- 小红书 URL：优先是 `note`，再用 `note_type` 表示图文还是视频。
- 微博 URL：优先是 `post`，内部可能带图片、视频、链接卡片或转发。
- 公众号 URL：优先是 `article`，正文里的图片、音频、视频只是内嵌资源。
- 知乎 URL：区分 `question`、`answer`、`article`，不要都归成文章。
- B 站 URL：视频稿件是 `video`，动态是 `post`，专栏是 `article`，直播间是 `live`。

Probe 的目标应是描述 URL 指向的内容实体：

```js
{
  platform,
  content_type,
  content,
  related_resources,
  raw
}
```

## 通用内容字段

无论平台和内容形态，通常都会有这些字段：

```js
{
  platform,              // douyin / zhihu / bilibili / xiaohongshu / officialaccount / weibo / wx_channels
  content_type,          // video / note / post / article / answer / question / live / collection ...
  content_id,            // 平台内稳定 ID
  canonical_url,         // 平台规范 URL
  source_url,            // 用户输入或当前访问 URL

  title,
  description,
  text,
  cover_url,

  author: {
    id,
    username,
    nickname,
    avatar_url,
    profile_url,
    verified,
  },

  publish_time,
  update_time,

  stats: {
    view_count,
    play_count,
    like_count,
    comment_count,
    share_count,
    collect_count,
    danmaku_count,
    repost_count,
  },

  tags,
  topics,
  mentions,
  location,

  visibility,            // public / private / deleted / restricted / login_required
  raw                    // 平台原始数据，便于以后扩展
}
```

## 主要内容类型

### `video`

短视频、长视频、视频回答、视频号视频、微博视频、B 站稿件。

字段：

- `duration`
- `width`
- `height`
- `fps`
- `bitrate`
- `format`
- `cover_url`
- `preview_images`
- `subtitle_info`
- `chapters`
- `music`
- `is_original`
- `is_repost`

### `image_album`

图集、图片笔记、多图微博、视频号图片内容。

字段：

- `images[]`
- `image_count`
- `cover_index`
- `width`
- `height`
- `format`
- `is_gif`
- `ocr_text`
- `captions`

### `note`

小红书这类“图文/视频 + 正文”的笔记型内容。

字段：

- `note_id`
- `note_type: image | video`
- `title`
- `text`
- `images[]`
- `video`
- `cover_url`
- `tags`
- `location`
- `product_cards[]`
- `collect_count`

### `post`

微博、B 站动态、知乎想法、视频号动态这类信息流动态。

字段：

- `post_id`
- `text`
- `rich_text`
- `images[]`
- `video`
- `link_cards[]`
- `topics[]`
- `mentions[]`
- `repost_of`
- `quote_of`
- `comment_count`
- `repost_count`

### `article`

公众号文章、知乎文章、B 站专栏、微博长文。

字段：

- `article_id`
- `title`
- `author`
- `digest`
- `body_html`
- `body_text`
- `cover_url`
- `images[]`
- `embedded_media[]`
- `word_count`
- `reading_time`
- `source_url`
- `copyright_info`

### `question`

知乎问题这类问答容器。

字段：

- `question_id`
- `title`
- `detail`
- `topics[]`
- `answer_count`
- `follower_count`
- `best_answers[]`
- `created_time`
- `updated_time`

### `answer`

知乎回答。

字段：

- `answer_id`
- `question_id`
- `question_title`
- `author`
- `body_html`
- `body_text`
- `excerpt`
- `images[]`
- `video`
- `vote_count`
- `comment_count`
- `created_time`
- `updated_time`

### `live`

直播间或直播回放。

字段：

- `room_id`
- `live_id`
- `title`
- `status: live | ended | scheduled`
- `anchor`
- `cover_url`
- `start_time`
- `end_time`
- `viewer_count`
- `reservation_count`
- `products[]`
- `replay_info`

### `collection`

合集、播放列表、系列、专栏目录、收藏夹。

字段：

- `collection_id`
- `title`
- `description`
- `owner`
- `cover_url`
- `item_count`
- `items[]`
- `updated_time`

### `account`

账号主页本身也是 URL 可指向的内容实体，但它不是内容作品。

字段：

- `account_id`
- `username`
- `nickname`
- `avatar_url`
- `bio`
- `verified`
- `follower_count`
- `following_count`
- `content_count`
- `profile_url`

### `topic`

话题、标签、超话、知乎话题、B 站分区/标签。

字段：

- `topic_id`
- `name`
- `description`
- `cover_url`
- `follower_count`
- `view_count`
- `post_count`
- `related_topics[]`

## 平台映射

| 平台 | 主要内容类型 |
| --- | --- |
| 抖音 | `video`、`image_album`、`post`、`live`、`collection` |
| 知乎 | `question`、`answer`、`article`、`video`、`post`、`topic` |
| B 站 | `video`、`post`、`article`、`live`、`collection`、`account` |
| 小红书 | `note`、`image_album`、`video`、`live`、`topic`、`account` |
| 公众号 | `article`、`collection`、`account`，文章内可嵌 `image/audio/video` |
| 微博 | `post`、`image_album`、`video`、`article`、`live`、`topic`、`account` |
| 视频号 | `video`、`image_album`、`post`、`live`、`account` |

## 设计备注

Probe 阶段只表达平台数据，不表达最终下载结果。是否下载视频、提取音频、保存正文、打包图片，属于后续解析、用户选择或任务创建阶段的职责。
