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
