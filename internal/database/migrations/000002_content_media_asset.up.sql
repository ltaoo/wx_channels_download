-- General archive taxonomy and stable downloadable/generated assets.
ALTER TABLE `content` ADD COLUMN `subtype` TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS `idx_content_subtype` ON `content` (`subtype`);

-- Collapse historical adapter-specific type names into stable archive
-- families while preserving the original distinction as subtype.
UPDATE `content`
SET `subtype` = LOWER(TRIM(`type`))
WHERE TRIM(COALESCE(`subtype`, '')) = ''
  AND LOWER(TRIM(`type`)) IN (
    'short_video', 'long_video', 'movie', 'film', 'tv_episode', 'clip', 'live_replay',
    'music', 'audiobook', 'voice', 'radio', 'space_recording',
    'image_set', 'gallery', 'photo_album', 'illustration_set',
    'blog', 'news', 'newsletter', 'question', 'answer', 'wiki',
    'microblog', 'tweet', 'status', 'thread', 'comment',
    'ebook', 'pdf', 'slides', 'spreadsheet',
    'livestream', 'audio_room',
    'playlist', 'series', 'feed', 'bookmark_collection',
    'chat', 'ai_chat', 'human_chat', 'email_thread'
  );

UPDATE `content`
SET `type` = CASE
  WHEN LOWER(TRIM(`type`)) IN ('short_video', 'long_video', 'movie', 'film', 'tv_episode', 'clip', 'live_replay') THEN 'video'
  WHEN LOWER(TRIM(`type`)) IN ('music', 'audiobook', 'voice', 'radio', 'space_recording') THEN 'audio'
  WHEN LOWER(TRIM(`type`)) IN ('image_set', 'gallery', 'photo_album', 'illustration_set') THEN 'album'
  WHEN LOWER(TRIM(`type`)) IN ('blog', 'news', 'newsletter', 'question', 'answer', 'wiki') THEN 'article'
  WHEN LOWER(TRIM(`type`)) IN ('microblog', 'tweet', 'status', 'thread', 'comment') THEN 'post'
  WHEN LOWER(TRIM(`type`)) IN ('ebook', 'pdf', 'slides', 'spreadsheet') THEN 'document'
  WHEN LOWER(TRIM(`type`)) IN ('livestream', 'audio_room') THEN 'live'
  WHEN LOWER(TRIM(`type`)) IN ('playlist', 'series', 'feed', 'bookmark_collection') THEN 'collection'
  WHEN LOWER(TRIM(`type`)) IN ('chat', 'ai_chat', 'human_chat', 'email_thread') THEN 'conversation'
  ELSE LOWER(TRIM(`type`))
END
WHERE TRIM(COALESCE(`subtype`, '')) <> '';

CREATE TABLE IF NOT EXISTS `content_relation` (
  `source_content_id` TEXT NOT NULL,
  `target_content_id` TEXT NOT NULL,
  `type` TEXT NOT NULL,
  `sort_order` INTEGER NOT NULL DEFAULT 0,
  `metadata` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (`source_content_id`, `target_content_id`, `type`),
  FOREIGN KEY (`source_content_id`) REFERENCES `content` (`id`),
  FOREIGN KEY (`target_content_id`) REFERENCES `content` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_content_relation_source`
ON `content_relation` (`source_content_id`);
CREATE INDEX IF NOT EXISTS `idx_content_relation_target`
ON `content_relation` (`target_content_id`);
CREATE INDEX IF NOT EXISTS `idx_content_relation_type`
ON `content_relation` (`type`);

CREATE TABLE IF NOT EXISTS `content_asset` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `content_id` TEXT NOT NULL,
  `kind` TEXT NOT NULL,
  `role` TEXT NOT NULL,
  `asset_key` TEXT NOT NULL,
  `label` TEXT,
  `language_code` TEXT,
  `mime_type` TEXT,
  `size` INTEGER NOT NULL DEFAULT 0,
  `sort_order` INTEGER NOT NULL DEFAULT 0,
  `metadata` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER,
  UNIQUE (`content_id`, `role`, `asset_key`),
  FOREIGN KEY (`content_id`) REFERENCES `content` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_content_asset_content`
ON `content_asset` (`content_id`);

CREATE INDEX IF NOT EXISTS `idx_content_asset_kind`
ON `content_asset` (`kind`);

CREATE INDEX IF NOT EXISTS `idx_content_asset_role`
ON `content_asset` (`role`);

CREATE INDEX IF NOT EXISTS `idx_content_asset_language`
ON `content_asset` (`language_code`);

CREATE INDEX IF NOT EXISTS `idx_content_asset_deleted_at`
ON `content_asset` (`deleted_at`);

-- Stable identities for nested novel entities. Legacy rows use URL identities
-- when available and fall back to their database IDs, never sequence index.
ALTER TABLE `content_novel_volume` ADD COLUMN `volume_key` TEXT NOT NULL DEFAULT '';
UPDATE `content_novel_volume`
SET `volume_key` = 'legacy:' || `id`
WHERE TRIM(COALESCE(`volume_key`, '')) = '';
CREATE UNIQUE INDEX IF NOT EXISTS `idx_novel_volume_identity`
ON `content_novel_volume` (`novel_id`, `volume_key`);

ALTER TABLE `content_novel_chapter` ADD COLUMN `chapter_key` TEXT NOT NULL DEFAULT '';
ALTER TABLE `content_novel_chapter` ADD COLUMN `volume_key` TEXT NOT NULL DEFAULT '';
UPDATE `content_novel_chapter`
SET `chapter_key` = CASE
  WHEN TRIM(COALESCE(`url`, '')) <> '' THEN 'url:' || TRIM(`url`)
  ELSE 'legacy:' || `id`
END
WHERE TRIM(COALESCE(`chapter_key`, '')) = '';
CREATE UNIQUE INDEX IF NOT EXISTS `idx_novel_chapter_identity`
ON `content_novel_chapter` (`novel_id`, `chapter_key`);

ALTER TABLE `content_image` ADD COLUMN `image_key` TEXT NOT NULL DEFAULT '';
UPDATE `content_image`
SET `image_key` = CASE
  WHEN TRIM(COALESCE(`url`, '')) <> '' THEN 'url:' || TRIM(`url`)
  ELSE 'legacy:' || `id`
END
WHERE TRIM(COALESCE(`image_key`, '')) = '';
CREATE UNIQUE INDEX IF NOT EXISTS `idx_content_image_identity`
ON `content_image` (`album_id`, `image_key`);

CREATE TABLE IF NOT EXISTS `content_asset_link` (
  `content_id` TEXT NOT NULL,
  `subject_type` TEXT NOT NULL,
  `subject_key` TEXT NOT NULL,
  `asset_id` INTEGER NOT NULL,
  `relation` TEXT NOT NULL,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (`content_id`, `subject_type`, `subject_key`, `asset_id`, `relation`),
  FOREIGN KEY (`content_id`) REFERENCES `content` (`id`),
  FOREIGN KEY (`asset_id`) REFERENCES `content_asset` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_content_asset_link_subject`
ON `content_asset_link` (`content_id`, `subject_type`, `subject_key`);
CREATE INDEX IF NOT EXISTS `idx_content_asset_link_asset`
ON `content_asset_link` (`asset_id`);

CREATE TABLE IF NOT EXISTS `content_video_variant` (
  `asset_id` INTEGER PRIMARY KEY,
  `video_id` TEXT NOT NULL,
  `variant_key` TEXT NOT NULL,
  `spec` TEXT,
  `quality` TEXT,
  `width` INTEGER,
  `height` INTEGER,
  `fps` INTEGER,
  `bitrate` INTEGER,
  `size` INTEGER NOT NULL DEFAULT 0,
  `codec` TEXT,
  `format` TEXT,
  `stream_type` TEXT,
  `has_video` INTEGER NOT NULL DEFAULT 1,
  `has_audio` INTEGER NOT NULL DEFAULT 1,
  `is_default` INTEGER NOT NULL DEFAULT 0,
  `url` TEXT,
  `url_expires_at` INTEGER,
  `metadata` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER,
  UNIQUE (`video_id`, `variant_key`),
  FOREIGN KEY (`asset_id`) REFERENCES `content_asset` (`id`),
  FOREIGN KEY (`video_id`) REFERENCES `content_video` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_content_video_variant_video`
ON `content_video_variant` (`video_id`);

CREATE INDEX IF NOT EXISTS `idx_content_video_variant_deleted_at`
ON `content_video_variant` (`deleted_at`);

CREATE TABLE IF NOT EXISTS `content_video_subtitle_track` (
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

CREATE INDEX IF NOT EXISTS `idx_content_video_subtitle_track_video`
ON `content_video_subtitle_track` (`video_id`);

CREATE INDEX IF NOT EXISTS `idx_content_video_subtitle_track_language`
ON `content_video_subtitle_track` (`language_code`);

CREATE INDEX IF NOT EXISTS `idx_content_video_subtitle_track_deleted_at`
ON `content_video_subtitle_track` (`deleted_at`);

CREATE TABLE IF NOT EXISTS `content_video_subtitle_source` (
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

CREATE INDEX IF NOT EXISTS `idx_content_video_subtitle_source_track`
ON `content_video_subtitle_source` (`track_id`);

CREATE INDEX IF NOT EXISTS `idx_content_video_subtitle_source_deleted_at`
ON `content_video_subtitle_source` (`deleted_at`);

CREATE TABLE IF NOT EXISTS `download_resource_asset` (
  `resource_id` INTEGER NOT NULL,
  `asset_id` INTEGER NOT NULL,
  `relation` TEXT NOT NULL,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (`resource_id`, `asset_id`, `relation`),
  FOREIGN KEY (`resource_id`) REFERENCES `download_resource` (`id`),
  FOREIGN KEY (`asset_id`) REFERENCES `content_asset` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_download_resource_asset_resource`
ON `download_resource_asset` (`resource_id`);

CREATE INDEX IF NOT EXISTS `idx_download_resource_asset_asset`
ON `download_resource_asset` (`asset_id`);

CREATE INDEX IF NOT EXISTS `idx_download_resource_content`
ON `download_resource` (`content_id`);

-- Every legacy video gets a default variant, even when its source did not
-- expose a resolution/specification.
INSERT OR IGNORE INTO `content_asset` (
  `content_id`, `kind`, `role`, `asset_key`, `mime_type`, `size`, `metadata`,
  `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `content_video`.`id`,
  'video',
  'video_variant',
  'default',
  CASE
    WHEN TRIM(COALESCE(`content_video`.`format`, '')) = '' THEN ''
    ELSE 'video/' || LOWER(`content_video`.`format`)
  END,
  COALESCE(`content_video`.`size`, 0),
  '',
  COALESCE(`content`.`created_at`, 0),
  COALESCE(`content`.`updated_at`, 0),
  `content_video`.`deleted_at`
FROM `content_video`
LEFT JOIN `content` ON `content`.`id` = `content_video`.`id`;

INSERT OR IGNORE INTO `content_video_variant` (
  `asset_id`, `video_id`, `variant_key`, `spec`, `quality`, `width`, `height`,
  `fps`, `bitrate`, `size`, `codec`, `format`, `stream_type`, `has_video`,
  `has_audio`, `is_default`, `url`, `metadata`, `created_at`, `updated_at`,
  `deleted_at`
)
SELECT
  `content_asset`.`id`,
  `content_video`.`id`,
  'default',
  '',
  '',
  NULLIF(`content_video`.`width`, 0),
  NULLIF(`content_video`.`height`, 0),
  NULLIF(`content_video`.`fps`, 0),
  NULLIF(`content_video`.`bitrate`, 0),
  COALESCE(`content_video`.`size`, 0),
  `content_video`.`codec`,
  `content_video`.`format`,
  'progressive',
  1,
  1,
  1,
  `content_video`.`url`,
  '',
  COALESCE(`content`.`created_at`, 0),
  COALESCE(`content`.`updated_at`, 0),
  `content_video`.`deleted_at`
FROM `content_video`
JOIN `content_asset`
  ON `content_asset`.`content_id` = `content_video`.`id`
 AND `content_asset`.`role` = 'video_variant'
 AND `content_asset`.`asset_key` = 'default'
LEFT JOIN `content` ON `content`.`id` = `content_video`.`id`;

-- Preserve the legacy single subtitle URL as an undetermined-language track.
INSERT OR IGNORE INTO `content_asset` (
  `content_id`, `kind`, `role`, `asset_key`, `language_code`, `mime_type`, `size`, `metadata`,
  `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `content_video`.`id`,
  'text',
  'subtitle',
  'legacy:und:default',
  'und',
  '',
  0,
  '',
  COALESCE(`content`.`created_at`, 0),
  COALESCE(`content`.`updated_at`, 0),
  `content_video`.`deleted_at`
FROM `content_video`
LEFT JOIN `content` ON `content`.`id` = `content_video`.`id`
WHERE TRIM(COALESCE(`content_video`.`subtitle_url`, '')) <> '';

-- Backfill assets for existing resources. Chapter identity is recovered from
-- source URL/index metadata, while video specification is recovered from Extra.
INSERT OR IGNORE INTO `content_asset` (
  `content_id`, `kind`, `role`, `asset_key`, `mime_type`, `size`, `metadata`,
  `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `download_resource`.`content_id`,
  CASE
    WHEN LOWER(`download_resource`.`kind`) = 'video'
      OR LOWER(`download_resource`.`kind`) LIKE 'video/%' THEN 'video'
    WHEN LOWER(`download_resource`.`kind`) = 'audio'
      OR LOWER(`download_resource`.`kind`) LIKE 'audio/%' THEN 'audio'
    WHEN LOWER(`download_resource`.`kind`) LIKE '%subtitle%'
      OR LOWER(`download_resource`.`name`) LIKE '%.srt'
      OR LOWER(`download_resource`.`name`) LIKE '%.vtt'
      OR LOWER(`download_resource`.`name`) LIKE '%.ass' THEN 'text'
    WHEN LOWER(`download_resource`.`kind`) = 'image'
      OR LOWER(`download_resource`.`kind`) LIKE 'image/%' THEN 'image'
    WHEN LOWER(`download_resource`.`kind`) = 'text'
      OR LOWER(`download_resource`.`kind`) LIKE 'text/%'
      OR LOWER(`download_resource`.`kind`) = 'html' THEN 'text'
    WHEN LOWER(`download_resource`.`name`) LIKE '%.pdf' THEN 'document'
    ELSE 'binary'
  END,
  CASE
    WHEN LOWER(`download_resource`.`kind`) = 'video'
      OR LOWER(`download_resource`.`kind`) LIKE 'video/%' THEN 'video_variant'
    WHEN LOWER(`download_resource`.`kind`) = 'audio'
      OR LOWER(`download_resource`.`kind`) LIKE 'audio/%' THEN 'audio_variant'
    WHEN LOWER(`download_resource`.`kind`) LIKE '%subtitle%'
      OR LOWER(`download_resource`.`name`) LIKE '%.srt'
      OR LOWER(`download_resource`.`name`) LIKE '%.vtt'
      OR LOWER(`download_resource`.`name`) LIKE '%.ass' THEN 'subtitle'
    WHEN LOWER(`download_resource`.`unique_id`) LIKE '%_cover' THEN 'cover'
    WHEN JSON_VALID(COALESCE(`download_resource`.`extra`, ''))
      AND CAST(COALESCE(JSON_EXTRACT(`download_resource`.`extra`, '$.chapter_index'), 0) AS INTEGER) > 0
      THEN 'novel_chapter'
    WHEN LOWER(`download_resource`.`kind`) = 'image'
      OR LOWER(`download_resource`.`kind`) LIKE 'image/%' THEN 'primary'
    WHEN LOWER(COALESCE(`content`.`type`, '')) IN ('article', 'blog')
      AND (LOWER(`download_resource`.`kind`) = 'html'
        OR LOWER(`download_resource`.`kind`) LIKE 'text/%') THEN 'article_body'
    WHEN LOWER(COALESCE(`content`.`type`, '')) = 'novel'
      AND (LOWER(`download_resource`.`kind`) = 'html'
        OR LOWER(`download_resource`.`kind`) = 'text'
        OR LOWER(`download_resource`.`kind`) LIKE 'text/%'
        OR LOWER(`download_resource`.`name`) LIKE '%.pdf'
        OR LOWER(`download_resource`.`name`) LIKE '%.epub') THEN 'novel_book'
    ELSE 'attachment'
  END,
  CASE
    WHEN (LOWER(`download_resource`.`kind`) = 'video'
      OR LOWER(`download_resource`.`kind`) LIKE 'video/%')
      AND JSON_VALID(COALESCE(`download_resource`.`extra`, ''))
      AND TRIM(COALESCE(JSON_EXTRACT(`download_resource`.`extra`, '$.spec'), '')) <> ''
      AND LOWER(TRIM(JSON_EXTRACT(`download_resource`.`extra`, '$.spec'))) <> 'original'
      THEN TRIM(JSON_EXTRACT(`download_resource`.`extra`, '$.spec'))
    WHEN LOWER(`download_resource`.`kind`) = 'video'
      OR LOWER(`download_resource`.`kind`) LIKE 'video/%' THEN 'default'
    WHEN JSON_VALID(COALESCE(`download_resource`.`extra`, ''))
      AND CAST(COALESCE(JSON_EXTRACT(`download_resource`.`extra`, '$.chapter_index'), 0) AS INTEGER) > 0
      THEN COALESCE(
        (
          SELECT `content_novel_chapter`.`chapter_key`
          FROM `content_novel_chapter`
          WHERE `content_novel_chapter`.`novel_id` = `download_resource`.`content_id`
            AND `content_novel_chapter`.`idx` = CAST(JSON_EXTRACT(`download_resource`.`extra`, '$.chapter_index') AS INTEGER)
          ORDER BY `content_novel_chapter`.`id`
          LIMIT 1
        ),
        CASE
          WHEN TRIM(COALESCE(JSON_EXTRACT(`download_resource`.`extra`, '$.source_url'), '')) <> ''
            THEN 'url:' || TRIM(JSON_EXTRACT(`download_resource`.`extra`, '$.source_url'))
          ELSE 'idx:' || CAST(JSON_EXTRACT(`download_resource`.`extra`, '$.chapter_index') AS TEXT)
        END
      ) || ':' || CASE
        WHEN LOWER(`download_resource`.`name`) LIKE '%.html' THEN 'html'
        WHEN LOWER(`download_resource`.`name`) LIKE '%.txt' THEN 'txt'
        WHEN LOWER(`download_resource`.`name`) LIKE '%.md' THEN 'md'
        ELSE 'default'
      END
    WHEN LOWER(`download_resource`.`kind`) = 'image'
      OR LOWER(`download_resource`.`kind`) LIKE 'image/%' THEN COALESCE(
        (
          SELECT `content_image`.`image_key` || ':original'
          FROM `download_endpoint`
          JOIN `content_image`
            ON `content_image`.`album_id` = `download_resource`.`content_id`
           AND `content_image`.`url` = `download_endpoint`.`url`
          WHERE `download_endpoint`.`resource_id` = `download_resource`.`id`
          ORDER BY `content_image`.`id`, `download_endpoint`.`id`
          LIMIT 1
        ),
        CASE
          WHEN TRIM(COALESCE(`download_resource`.`unique_id`, '')) <> ''
            THEN `download_resource`.`unique_id`
          ELSE 'resource:' || `download_resource`.`id`
        END
      )
    WHEN TRIM(COALESCE(`download_resource`.`unique_id`, '')) <> ''
      THEN `download_resource`.`unique_id`
    ELSE 'resource:' || `download_resource`.`id`
  END,
  CASE
    WHEN INSTR(`download_resource`.`kind`, '/') > 0 THEN LOWER(`download_resource`.`kind`)
    ELSE ''
  END,
  COALESCE(`download_resource`.`size`, 0),
  '',
  COALESCE(`download_resource`.`created_at`, 0),
  COALESCE(`download_resource`.`updated_at`, 0),
  `download_resource`.`deleted_at`
FROM `download_resource`
LEFT JOIN `content` ON `content`.`id` = `download_resource`.`content_id`
WHERE `download_resource`.`content_id` IS NOT NULL
  AND TRIM(`download_resource`.`content_id`) <> '';

-- Legacy non-default video resources become variants. Their exact dimensions
-- remain unknown until the source adapter refreshes the catalog.
INSERT OR IGNORE INTO `content_video_variant` (
  `asset_id`, `video_id`, `variant_key`, `spec`, `quality`, `size`, `format`,
  `stream_type`, `has_video`, `has_audio`, `is_default`, `url`, `metadata`,
  `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `content_asset`.`id`,
  `content_asset`.`content_id`,
  `content_asset`.`asset_key`,
  CASE WHEN `content_asset`.`asset_key` = 'default'
    THEN '' ELSE `content_asset`.`asset_key` END,
  '',
  `content_asset`.`size`,
  '',
  'progressive',
  1,
  1,
  CASE WHEN `content_asset`.`asset_key` = 'default' THEN 1 ELSE 0 END,
  '',
  '',
  `content_asset`.`created_at`,
  `content_asset`.`updated_at`,
  `content_asset`.`deleted_at`
FROM `content_asset`
JOIN `content_video` ON `content_video`.`id` = `content_asset`.`content_id`
WHERE `content_asset`.`role` = 'video_variant';

INSERT OR IGNORE INTO `content_video_subtitle_track` (
  `video_id`, `track_key`, `language_code`, `language_name`, `label`, `kind`,
  `is_default`, `is_forced`, `is_auto_generated`, `is_hearing_impaired`,
  `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `content_video`.`id`,
  'legacy:und',
  'und',
  '',
  '',
  'subtitle',
  1,
  0,
  0,
  0,
  COALESCE(`content`.`created_at`, 0),
  COALESCE(`content`.`updated_at`, 0),
  `content_video`.`deleted_at`
FROM `content_video`
LEFT JOIN `content` ON `content`.`id` = `content_video`.`id`
WHERE TRIM(COALESCE(`content_video`.`subtitle_url`, '')) <> ''
   OR EXISTS (
     SELECT 1
     FROM `content_asset`
     WHERE `content_asset`.`content_id` = `content_video`.`id`
       AND `content_asset`.`role` = 'subtitle'
   );

INSERT OR IGNORE INTO `content_video_subtitle_source` (
  `asset_id`, `track_id`, `source_key`, `format`, `url`, `encoding`, `metadata`,
  `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `content_asset`.`id`,
  `content_video_subtitle_track`.`id`,
  CASE
    WHEN `content_asset`.`asset_key` = 'legacy:und:default' THEN 'default'
    ELSE `content_asset`.`asset_key`
  END,
  CASE
    WHEN LOWER(`content_asset`.`asset_key`) LIKE '%.vtt' THEN 'vtt'
    WHEN LOWER(`content_asset`.`asset_key`) LIKE '%.srt' THEN 'srt'
    WHEN LOWER(`content_asset`.`asset_key`) LIKE '%.ass' THEN 'ass'
    ELSE ''
  END,
  CASE
    WHEN `content_asset`.`asset_key` = 'legacy:und:default'
      THEN COALESCE(`content_video`.`subtitle_url`, '')
    ELSE ''
  END,
  '',
  '',
  `content_asset`.`created_at`,
  `content_asset`.`updated_at`,
  `content_asset`.`deleted_at`
FROM `content_asset`
JOIN `content_video_subtitle_track`
  ON `content_video_subtitle_track`.`video_id` = `content_asset`.`content_id`
 AND `content_video_subtitle_track`.`track_key` = 'legacy:und'
JOIN `content_video`
  ON `content_video`.`id` = `content_asset`.`content_id`
WHERE `content_asset`.`role` = 'subtitle';

INSERT OR IGNORE INTO `download_resource_asset` (
  `resource_id`, `asset_id`, `relation`, `created_at`
)
SELECT
  `download_resource`.`id`,
  `content_asset`.`id`,
  'source',
  COALESCE(`download_resource`.`created_at`, 0)
FROM `download_resource`
JOIN `content_asset`
  ON `content_asset`.`content_id` = `download_resource`.`content_id`
 AND `content_asset`.`role` = CASE
    WHEN LOWER(`download_resource`.`kind`) = 'video'
      OR LOWER(`download_resource`.`kind`) LIKE 'video/%' THEN 'video_variant'
    WHEN LOWER(`download_resource`.`kind`) = 'audio'
      OR LOWER(`download_resource`.`kind`) LIKE 'audio/%' THEN 'audio_variant'
    WHEN LOWER(`download_resource`.`kind`) LIKE '%subtitle%'
      OR LOWER(`download_resource`.`name`) LIKE '%.srt'
      OR LOWER(`download_resource`.`name`) LIKE '%.vtt'
      OR LOWER(`download_resource`.`name`) LIKE '%.ass' THEN 'subtitle'
    WHEN LOWER(`download_resource`.`unique_id`) LIKE '%_cover' THEN 'cover'
    WHEN JSON_VALID(COALESCE(`download_resource`.`extra`, ''))
      AND CAST(COALESCE(JSON_EXTRACT(`download_resource`.`extra`, '$.chapter_index'), 0) AS INTEGER) > 0
      THEN 'novel_chapter'
    WHEN LOWER(`download_resource`.`kind`) = 'image'
      OR LOWER(`download_resource`.`kind`) LIKE 'image/%' THEN 'primary'
    WHEN LOWER(COALESCE(`content`.`type`, '')) IN ('article', 'blog')
      AND (LOWER(`download_resource`.`kind`) = 'html'
        OR LOWER(`download_resource`.`kind`) LIKE 'text/%') THEN 'article_body'
    WHEN LOWER(COALESCE(`content`.`type`, '')) = 'novel'
      AND (LOWER(`download_resource`.`kind`) = 'html'
        OR LOWER(`download_resource`.`kind`) = 'text'
        OR LOWER(`download_resource`.`kind`) LIKE 'text/%'
        OR LOWER(`download_resource`.`name`) LIKE '%.pdf'
        OR LOWER(`download_resource`.`name`) LIKE '%.epub') THEN 'novel_book'
    ELSE 'attachment'
  END
 AND `content_asset`.`asset_key` = CASE
    WHEN (LOWER(`download_resource`.`kind`) = 'video'
      OR LOWER(`download_resource`.`kind`) LIKE 'video/%')
      AND JSON_VALID(COALESCE(`download_resource`.`extra`, ''))
      AND TRIM(COALESCE(JSON_EXTRACT(`download_resource`.`extra`, '$.spec'), '')) <> ''
      AND LOWER(TRIM(JSON_EXTRACT(`download_resource`.`extra`, '$.spec'))) <> 'original'
      THEN TRIM(JSON_EXTRACT(`download_resource`.`extra`, '$.spec'))
    WHEN LOWER(`download_resource`.`kind`) = 'video'
      OR LOWER(`download_resource`.`kind`) LIKE 'video/%' THEN 'default'
    WHEN JSON_VALID(COALESCE(`download_resource`.`extra`, ''))
      AND CAST(COALESCE(JSON_EXTRACT(`download_resource`.`extra`, '$.chapter_index'), 0) AS INTEGER) > 0
      THEN COALESCE(
        (
          SELECT `content_novel_chapter`.`chapter_key`
          FROM `content_novel_chapter`
          WHERE `content_novel_chapter`.`novel_id` = `download_resource`.`content_id`
            AND `content_novel_chapter`.`idx` = CAST(JSON_EXTRACT(`download_resource`.`extra`, '$.chapter_index') AS INTEGER)
          ORDER BY `content_novel_chapter`.`id`
          LIMIT 1
        ),
        CASE
          WHEN TRIM(COALESCE(JSON_EXTRACT(`download_resource`.`extra`, '$.source_url'), '')) <> ''
            THEN 'url:' || TRIM(JSON_EXTRACT(`download_resource`.`extra`, '$.source_url'))
          ELSE 'idx:' || CAST(JSON_EXTRACT(`download_resource`.`extra`, '$.chapter_index') AS TEXT)
        END
      ) || ':' || CASE
        WHEN LOWER(`download_resource`.`name`) LIKE '%.html' THEN 'html'
        WHEN LOWER(`download_resource`.`name`) LIKE '%.txt' THEN 'txt'
        WHEN LOWER(`download_resource`.`name`) LIKE '%.md' THEN 'md'
        ELSE 'default'
      END
    WHEN LOWER(`download_resource`.`kind`) = 'image'
      OR LOWER(`download_resource`.`kind`) LIKE 'image/%' THEN COALESCE(
        (
          SELECT `content_image`.`image_key` || ':original'
          FROM `download_endpoint`
          JOIN `content_image`
            ON `content_image`.`album_id` = `download_resource`.`content_id`
           AND `content_image`.`url` = `download_endpoint`.`url`
          WHERE `download_endpoint`.`resource_id` = `download_resource`.`id`
          ORDER BY `content_image`.`id`, `download_endpoint`.`id`
          LIMIT 1
        ),
        CASE
          WHEN TRIM(COALESCE(`download_resource`.`unique_id`, '')) <> ''
            THEN `download_resource`.`unique_id`
          ELSE 'resource:' || `download_resource`.`id`
        END
      )
    WHEN TRIM(COALESCE(`download_resource`.`unique_id`, '')) <> ''
      THEN `download_resource`.`unique_id`
    ELSE 'resource:' || `download_resource`.`id`
  END
LEFT JOIN `content` ON `content`.`id` = `download_resource`.`content_id`
WHERE `download_resource`.`content_id` IS NOT NULL
  AND TRIM(`download_resource`.`content_id`) <> '';

INSERT OR IGNORE INTO `content_asset_link` (
  `content_id`, `subject_type`, `subject_key`, `asset_id`, `relation`, `created_at`
)
SELECT
  `content_novel_chapter`.`novel_id`,
  'novel_chapter',
  `content_novel_chapter`.`chapter_key`,
  `content_asset`.`id`,
  'representation',
  `content_asset`.`created_at`
FROM `content_novel_chapter`
JOIN `content_asset`
  ON `content_asset`.`content_id` = `content_novel_chapter`.`novel_id`
 AND `content_asset`.`role` = 'novel_chapter'
 AND `content_asset`.`asset_key` LIKE `content_novel_chapter`.`chapter_key` || ':%';

-- Every archived album image receives a stable subject asset. Exact legacy
-- endpoint URL matches are connected to their historical resources.
INSERT OR IGNORE INTO `content_asset` (
  `content_id`, `kind`, `role`, `asset_key`, `mime_type`, `size`, `sort_order`,
  `metadata`, `created_at`, `updated_at`, `deleted_at`
)
SELECT
  `content_image`.`album_id`,
  'image',
  'primary',
  `content_image`.`image_key` || ':original',
  CASE
    WHEN TRIM(COALESCE(`content_image`.`ext`, '')) = '' THEN ''
    ELSE 'image/' || LOWER(`content_image`.`ext`)
  END,
  COALESCE(`content_image`.`size`, 0),
  COALESCE(`content_image`.`sort_order`, 0),
  '',
  COALESCE(`content`.`created_at`, 0),
  COALESCE(`content`.`updated_at`, 0),
  `content_image`.`deleted_at`
FROM `content_image`
LEFT JOIN `content` ON `content`.`id` = `content_image`.`album_id`;

INSERT OR IGNORE INTO `content_asset_link` (
  `content_id`, `subject_type`, `subject_key`, `asset_id`, `relation`, `created_at`
)
SELECT
  `content_image`.`album_id`,
  'album_image',
  `content_image`.`image_key`,
  `content_asset`.`id`,
  'representation',
  `content_asset`.`created_at`
FROM `content_image`
JOIN `content_asset`
  ON `content_asset`.`content_id` = `content_image`.`album_id`
 AND `content_asset`.`role` = 'primary'
 AND `content_asset`.`asset_key` = `content_image`.`image_key` || ':original';

INSERT OR IGNORE INTO `download_resource_asset` (
  `resource_id`, `asset_id`, `relation`, `created_at`
)
SELECT DISTINCT
  `download_resource`.`id`,
  `content_asset`.`id`,
  'source',
  COALESCE(`download_resource`.`created_at`, 0)
FROM `content_image`
JOIN `content_asset`
  ON `content_asset`.`content_id` = `content_image`.`album_id`
 AND `content_asset`.`role` = 'primary'
 AND `content_asset`.`asset_key` = `content_image`.`image_key` || ':original'
JOIN `download_resource`
  ON `download_resource`.`content_id` = `content_image`.`album_id`
 AND (
   LOWER(`download_resource`.`kind`) = 'image'
   OR LOWER(`download_resource`.`kind`) LIKE 'image/%'
 )
JOIN `download_endpoint`
  ON `download_endpoint`.`resource_id` = `download_resource`.`id`
 AND `download_endpoint`.`url` = `content_image`.`url`;

-- Distinguish main-story chapters from extras such as side stories and
-- acknowledgements. Existing chapters remain main-story chapters by default.
ALTER TABLE `content_novel_chapter`
ADD COLUMN `is_extra` INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS `idx_novel_chapter_novel_extra`
ON `content_novel_chapter` (`novel_id`, `is_extra`);

-- External AI conversation archives. These tables are intentionally separate
-- from chat_session/chat_message, which belong to the application's own chat
-- and generic chat-import subsystem.
CREATE TABLE IF NOT EXISTS `content_conversation` (
  `id` TEXT PRIMARY KEY,
  `source_type` TEXT,
  `source_format` TEXT,
  `format_version` TEXT,
  `default_model_provider` TEXT,
  `default_model_name` TEXT,
  `current_branch_key` TEXT,
  `message_count` INTEGER NOT NULL DEFAULT 0,
  `branch_count` INTEGER NOT NULL DEFAULT 0,
  `is_shared` INTEGER NOT NULL DEFAULT 0,
  `metadata` TEXT,
  FOREIGN KEY (`id`) REFERENCES `content` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_content_conversation_source`
ON `content_conversation` (`source_type`);

CREATE TABLE IF NOT EXISTS `content_conversation_branch` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `conversation_id` TEXT NOT NULL,
  `branch_key` TEXT NOT NULL,
  `title` TEXT,
  `root_message_key` TEXT,
  `leaf_message_key` TEXT,
  `is_current` INTEGER NOT NULL DEFAULT 0,
  `sort_order` INTEGER NOT NULL DEFAULT 0,
  `metadata` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER,
  UNIQUE (`conversation_id`, `branch_key`),
  FOREIGN KEY (`conversation_id`) REFERENCES `content_conversation` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_content_conversation_branch_conversation`
ON `content_conversation_branch` (`conversation_id`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_branch_deleted_at`
ON `content_conversation_branch` (`deleted_at`);

CREATE TABLE IF NOT EXISTS `content_conversation_message` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `conversation_id` TEXT NOT NULL,
  `message_key` TEXT NOT NULL,
  `parent_message_key` TEXT,
  `role` TEXT NOT NULL,
  `author_name` TEXT,
  `model_provider` TEXT,
  `model_name` TEXT,
  `status` TEXT,
  `content_text` TEXT,
  `content_hash` TEXT,
  `sequence` INTEGER NOT NULL DEFAULT 0,
  `sent_at` INTEGER,
  `edited_at` INTEGER,
  `metadata` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER,
  UNIQUE (`conversation_id`, `message_key`),
  FOREIGN KEY (`conversation_id`) REFERENCES `content_conversation` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_conversation`
ON `content_conversation_message` (`conversation_id`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_parent`
ON `content_conversation_message` (`conversation_id`, `parent_message_key`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_role`
ON `content_conversation_message` (`role`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_sequence`
ON `content_conversation_message` (`conversation_id`, `sequence`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_deleted_at`
ON `content_conversation_message` (`deleted_at`);

CREATE TABLE IF NOT EXISTS `content_conversation_message_part` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `conversation_id` TEXT NOT NULL,
  `message_id` INTEGER NOT NULL,
  `message_key` TEXT NOT NULL,
  `part_key` TEXT NOT NULL,
  `subject_key` TEXT NOT NULL,
  `sort_order` INTEGER NOT NULL DEFAULT 0,
  `type` TEXT NOT NULL,
  `text` TEXT,
  `url` TEXT,
  `mime_type` TEXT,
  `language_code` TEXT,
  `tool_call_id` TEXT,
  `tool_name` TEXT,
  `metadata` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER,
  UNIQUE (`conversation_id`, `message_key`, `part_key`),
  UNIQUE (`conversation_id`, `subject_key`),
  FOREIGN KEY (`conversation_id`) REFERENCES `content_conversation` (`id`),
  FOREIGN KEY (`message_id`) REFERENCES `content_conversation_message` (`id`)
);

CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_part_conversation`
ON `content_conversation_message_part` (`conversation_id`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_part_message`
ON `content_conversation_message_part` (`message_id`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_part_subject`
ON `content_conversation_message_part` (`conversation_id`, `subject_key`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_part_type`
ON `content_conversation_message_part` (`type`);
CREATE INDEX IF NOT EXISTS `idx_content_conversation_message_part_deleted_at`
ON `content_conversation_message_part` (`deleted_at`);
