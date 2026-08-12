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
