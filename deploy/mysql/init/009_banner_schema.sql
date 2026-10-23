-- Home-page banners, owned by the banner service.
--
--   rotation_info  the carousel at the top of the home page
--   position_info  hand-picked goods slots ("手工位")
--
-- Images follow the goods_images pattern: file_id points at the file service
-- (the source of truth) and pic_url is copied in on write, so reading the home
-- page never calls another service.
--
-- Safe to re-run. On an existing volume apply it with `make db-init`.

CREATE DATABASE IF NOT EXISTS `banner`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

GRANT SELECT, INSERT, UPDATE, DELETE ON `banner`.* TO 'shop'@'%';

USE `banner`;

CREATE TABLE IF NOT EXISTS `rotation_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `file_id` BIGINT UNSIGNED NOT NULL,
  `pic_url` VARCHAR(512) NOT NULL COMMENT 'copied from the file service on write',
  `link` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'http(s) only; empty for no link',
  `sort` SMALLINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'ascending',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_rotation_sort` (`deleted_at`, `sort`, `id`)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS `position_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `file_id` BIGINT UNSIGNED NOT NULL,
  `pic_url` VARCHAR(512) NOT NULL COMMENT 'copied from the file service on write',
  `goods_id` BIGINT UNSIGNED NOT NULL,
  `goods_name` VARCHAR(200) NOT NULL COMMENT 'copied from the goods service on write',
  `link` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'http(s) only; empty means the goods page',
  `sort` SMALLINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'ascending',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_position_sort` (`deleted_at`, `sort`, `id`)
) ENGINE=InnoDB;
