-- Goods, goods images and categories, owned by the goods service.
--
-- Safe to re-run. On an existing volume apply it with `make db-init`.

CREATE DATABASE IF NOT EXISTS `goods`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

GRANT SELECT, INSERT, UPDATE, DELETE ON `goods`.* TO 'shop'@'%';

USE `goods`;

-- A three-level tree. level is derived from the parent on write (parent level
-- + 1), never taken from the client, so it cannot disagree with parent_id.
CREATE TABLE IF NOT EXISTS `category_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `parent_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0 for a top-level category',
  `level` TINYINT UNSIGNED NOT NULL COMMENT '1, 2 or 3',
  `name` VARCHAR(30) NOT NULL,
  `icon_file_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'file service id, 0 for none',
  `icon_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'copied from the file service on write',
  `sort` SMALLINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'ascending',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_category_parent` (`parent_id`, `deleted_at`)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS `goods_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(200) NOT NULL,
  -- Integer cents: floating point cannot represent most decimal prices.
  `price_cents` BIGINT UNSIGNED NOT NULL,
  -- level1 is required; level2/level3 are 0 or a chain under level1.
  `level1_category_id` BIGINT UNSIGNED NOT NULL,
  `level2_category_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `level3_category_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `brand` VARCHAR(30) NOT NULL DEFAULT '',
  `stock` INT UNSIGNED NOT NULL DEFAULT 0,
  -- Written by the order flow later, never by an administrator form.
  `sale` INT UNSIGNED NOT NULL DEFAULT 0,
  `tags` VARCHAR(255) NOT NULL DEFAULT '',
  `detail_info` TEXT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_goods_level1` (`level1_category_id`, `deleted_at`),
  KEY `idx_goods_level2` (`level2_category_id`),
  KEY `idx_goods_level3` (`level3_category_id`)
) ENGINE=InnoDB;

-- Images reference the file service by file_id. url is copied in when the
-- image is attached, so reading a product never needs a call to another
-- service; the file service stays the source of truth for the file itself.
CREATE TABLE IF NOT EXISTS `goods_images` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `goods_id` BIGINT UNSIGNED NOT NULL,
  `file_id` BIGINT UNSIGNED NOT NULL,
  `url` VARCHAR(512) NOT NULL,
  `sort` SMALLINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'ascending',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_goods_images_file` (`goods_id`, `file_id`),
  KEY `idx_goods_images_goods` (`goods_id`, `sort`)
) ENGINE=InnoDB;
