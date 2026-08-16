-- Roll back the squashed content text-track migration first.
-- Restore the v2 compatibility columns before rebuilding its subtitle tables.
ALTER TABLE `content_video` ADD COLUMN `has_subtitle` INTEGER DEFAULT 0;
ALTER TABLE `content_video` ADD COLUMN `subtitle_url` TEXT;
ALTER TABLE `content_audio` ADD COLUMN `lyrics_url` TEXT;

UPDATE `content_video`
SET
  `has_subtitle` = CASE WHEN EXISTS (
    SELECT 1
    FROM `content_text_track`
    WHERE `content_text_track`.`content_id` = `content_video`.`id`
      AND `content_text_track`.`type` IN ('subtitle', 'caption', 'transcript')
      AND `content_text_track`.`deleted_at` IS NULL
  ) THEN 1 ELSE 0 END,
  `subtitle_url` = COALESCE((
    SELECT `content_text_track_source`.`url`
    FROM `content_text_track`
    JOIN `content_text_track_source`
      ON `content_text_track_source`.`track_id` = `content_text_track`.`id`
    WHERE `content_text_track`.`content_id` = `content_video`.`id`
      AND `content_text_track`.`type` IN ('subtitle', 'caption', 'transcript')
      AND `content_text_track`.`deleted_at` IS NULL
      AND `content_text_track_source`.`deleted_at` IS NULL
      AND TRIM(COALESCE(`content_text_track_source`.`url`, '')) <> ''
    ORDER BY
      `content_text_track`.`is_default` DESC,
      CASE `content_text_track_source`.`format`
        WHEN 'vtt' THEN 0 WHEN 'srt' THEN 1 WHEN 'ttml' THEN 2 ELSE 3
      END,
      `content_text_track`.`id`,
      `content_text_track_source`.`asset_id`
    LIMIT 1
  ), '');

UPDATE `content_audio`
SET `lyrics_url` = COALESCE((
  SELECT `content_text_track_source`.`url`
  FROM `content_text_track`
  JOIN `content_text_track_source`
    ON `content_text_track_source`.`track_id` = `content_text_track`.`id`
  WHERE `content_text_track`.`content_id` = `content_audio`.`id`
    AND `content_text_track`.`type` = 'lyrics'
    AND `content_text_track`.`deleted_at` IS NULL
    AND `content_text_track_source`.`deleted_at` IS NULL
    AND TRIM(COALESCE(`content_text_track_source`.`url`, '')) <> ''
  ORDER BY
    `content_text_track`.`is_default` DESC,
    CASE `content_text_track_source`.`format`
      WHEN 'lrc' THEN 0 WHEN 'vtt' THEN 1 WHEN 'srt' THEN 2 ELSE 3
    END,
    `content_text_track`.`id`,
    `content_text_track_source`.`asset_id`
  LIMIT 1
), '');

CREATE TABLE `content_video_subtitle_track` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `video_id` TEXT NOT NULL,
  `track_key` TEXT NOT NULL,
  `language_code` TEXT NOT NULL DEFAULT 'und',
  `language_name` TEXT,
  `label` TEXT,
  `kind` TEXT,
  `is_default` INTEGER NOT NULL DEFAULT 0,
  `is_forced` INTEGER NOT NULL DEFAULT 0,
  `is_auto_generated` INTEGER NOT NULL DEFAULT 0,
  `is_hearing_impaired` INTEGER NOT NULL DEFAULT 0,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER,
  UNIQUE (`video_id`, `track_key`),
  FOREIGN KEY (`video_id`) REFERENCES `content_video` (`id`)
);

CREATE INDEX `idx_content_video_subtitle_track_video`
ON `content_video_subtitle_track` (`video_id`);
CREATE INDEX `idx_content_video_subtitle_track_language`
ON `content_video_subtitle_track` (`language_code`);
CREATE INDEX `idx_content_video_subtitle_track_deleted_at`
ON `content_video_subtitle_track` (`deleted_at`);

CREATE TABLE `content_video_subtitle_source` (
  `asset_id` INTEGER PRIMARY KEY,
  `track_id` INTEGER NOT NULL,
  `source_key` TEXT NOT NULL,
  `format` TEXT,
  `url` TEXT,
  `url_expires_at` INTEGER,
  `encoding` TEXT,
  `metadata` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER,
  UNIQUE (`track_id`, `source_key`),
  FOREIGN KEY (`asset_id`) REFERENCES `content_asset` (`id`),
  FOREIGN KEY (`track_id`) REFERENCES `content_video_subtitle_track` (`id`)
);

CREATE INDEX `idx_content_video_subtitle_source_track`
ON `content_video_subtitle_source` (`track_id`);
CREATE INDEX `idx_content_video_subtitle_source_deleted_at`
ON `content_video_subtitle_source` (`deleted_at`);

INSERT INTO `content_video_subtitle_track` (
  `id`, `video_id`, `track_key`, `language_code`, `language_name`, `label`,
  `kind`, `is_default`, `is_forced`, `is_auto_generated`,
  `is_hearing_impaired`, `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `content_text_track`.`id`, `content_text_track`.`content_id`,
  `content_text_track`.`track_key`, `content_text_track`.`language_code`,
  `content_text_track`.`language_name`, `content_text_track`.`label`,
  `content_text_track`.`type`, `content_text_track`.`is_default`,
  `content_text_track`.`is_forced`, `content_text_track`.`is_auto_generated`,
  `content_text_track`.`is_hearing_impaired`, `content_text_track`.`created_at`,
  `content_text_track`.`updated_at`, `content_text_track`.`deleted_at`
FROM `content_text_track`
JOIN `content_video`
  ON `content_video`.`id` = `content_text_track`.`content_id`
WHERE `content_text_track`.`type` IN ('subtitle', 'caption', 'transcript');

INSERT INTO `content_video_subtitle_source` (
  `asset_id`, `track_id`, `source_key`, `format`, `url`, `url_expires_at`,
  `encoding`, `metadata`, `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `content_text_track_source`.`asset_id`, `content_text_track_source`.`track_id`,
  `content_text_track_source`.`source_key`, `content_text_track_source`.`format`,
  `content_text_track_source`.`url`, `content_text_track_source`.`url_expires_at`,
  `content_text_track_source`.`encoding`, `content_text_track_source`.`metadata`,
  `content_text_track_source`.`created_at`, `content_text_track_source`.`updated_at`,
  `content_text_track_source`.`deleted_at`
FROM `content_text_track_source`
JOIN `content_video_subtitle_track`
  ON `content_video_subtitle_track`.`id` = `content_text_track_source`.`track_id`;

DROP TABLE `content_text_track_source`;
DROP TABLE `content_text_track`;

DROP INDEX IF EXISTS `idx_content_relation_target_type_sort`;
DROP INDEX IF EXISTS `idx_content_relation_source_type_sort`;


DROP INDEX IF EXISTS `idx_content_influencer_content_role`;
DROP INDEX IF EXISTS `idx_content_influencer_influencer`;

ALTER TABLE `content_influencer` RENAME TO `content_influencer_roles`;

CREATE TABLE `content_influencer` (
  `content_id` TEXT NOT NULL,
  `influencer_id` INTEGER NOT NULL,
  `role` TEXT DEFAULT 'creator',
  `created_at` INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (`content_id`, `influencer_id`)
);

INSERT OR IGNORE INTO `content_influencer` (`content_id`, `influencer_id`, `role`, `created_at`)
SELECT `content_id`, `influencer_id`, `role`, `created_at`
FROM `content_influencer_roles`
ORDER BY `sort_order`, `role`;

DROP TABLE `content_influencer_roles`;
CREATE INDEX IF NOT EXISTS `idx_content_influencer_influencer`
ON `content_influencer` (`influencer_id`);

DROP INDEX IF EXISTS `idx_influencer_imdb_id`;
DROP INDEX IF EXISTS `idx_influencer_douban_id`;
DROP INDEX IF EXISTS `idx_influencer_tmdb_id`;
DROP INDEX IF EXISTS `idx_influencer_name`;
ALTER TABLE `influencer` DROP COLUMN `metadata_json`;
ALTER TABLE `influencer` DROP COLUMN `imdb_id`;
ALTER TABLE `influencer` DROP COLUMN `douban_id`;
ALTER TABLE `influencer` DROP COLUMN `tmdb_id`;
ALTER TABLE `influencer` DROP COLUMN `profile`;
ALTER TABLE `influencer` DROP COLUMN `known_for_department`;
ALTER TABLE `influencer` DROP COLUMN `place_of_birth`;
ALTER TABLE `influencer` DROP COLUMN `birthday`;
ALTER TABLE `influencer` DROP COLUMN `profile_path`;
ALTER TABLE `influencer` DROP COLUMN `biography`;
ALTER TABLE `influencer` DROP COLUMN `alias`;

DROP INDEX IF EXISTS `idx_content_series_imdb_id`;
DROP INDEX IF EXISTS `idx_content_series_douban_id`;
DROP INDEX IF EXISTS `idx_content_series_tmdb_id`;
DROP TABLE IF EXISTS `content_series`;

DROP INDEX IF EXISTS `idx_content_episode_imdb_id`;
DROP INDEX IF EXISTS `idx_content_episode_douban_id`;
DROP INDEX IF EXISTS `idx_content_episode_tmdb_id`;
DROP TABLE IF EXISTS `content_episode`;

DROP TABLE IF EXISTS `content_conversation_message_part`;
DROP TABLE IF EXISTS `content_conversation_message`;
DROP TABLE IF EXISTS `content_conversation_branch`;
DROP TABLE IF EXISTS `content_conversation`;

DROP INDEX IF EXISTS `idx_novel_chapter_novel_extra`;
ALTER TABLE `content_novel_chapter` DROP COLUMN `is_extra`;

DROP INDEX IF EXISTS `idx_download_resource_content`;
DROP TABLE IF EXISTS `download_resource_asset`;
DROP TABLE IF EXISTS `content_video_subtitle_source`;
DROP TABLE IF EXISTS `content_video_subtitle_track`;
DROP TABLE IF EXISTS `content_video_variant`;
DROP TABLE IF EXISTS `content_asset_link`;
DROP TABLE IF EXISTS `content_asset`;
DROP TABLE IF EXISTS `content_relation`;
DROP INDEX IF EXISTS `idx_novel_chapter_identity`;
DROP INDEX IF EXISTS `idx_novel_volume_identity`;
DROP INDEX IF EXISTS `idx_content_image_identity`;
ALTER TABLE `content_image` DROP COLUMN `image_key`;
ALTER TABLE `content_novel_chapter` DROP COLUMN `volume_key`;
ALTER TABLE `content_novel_chapter` DROP COLUMN `chapter_key`;
ALTER TABLE `content_novel_volume` DROP COLUMN `volume_key`;
DROP INDEX IF EXISTS `idx_content_subtype`;
UPDATE `content`
SET `type` = `subtype`
WHERE (`type` = 'video' AND `subtype` IN ('short_video', 'long_video', 'movie', 'film', 'tv_episode', 'clip', 'live_replay'))
   OR (`type` = 'audio' AND `subtype` IN ('music', 'audiobook', 'voice', 'radio', 'space_recording'))
   OR (`type` = 'album' AND `subtype` IN ('image_set', 'gallery', 'photo_album', 'illustration_set'))
   OR (`type` = 'article' AND `subtype` IN ('blog', 'news', 'newsletter', 'question', 'answer', 'wiki'))
   OR (`type` = 'post' AND `subtype` IN ('microblog', 'tweet', 'status', 'thread', 'comment'))
   OR (`type` = 'document' AND `subtype` IN ('ebook', 'pdf', 'slides', 'spreadsheet'))
   OR (`type` = 'live' AND `subtype` IN ('livestream', 'audio_room'))
   OR (`type` = 'collection' AND `subtype` IN ('playlist', 'series', 'feed', 'bookmark_collection'))
   OR (`type` = 'conversation' AND `subtype` IN ('chat', 'ai_chat', 'human_chat', 'email_thread'));
ALTER TABLE `content` DROP COLUMN `subtype`;
