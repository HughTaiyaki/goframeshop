-- Praise, collection and comment data owned by the interaction service.
--
-- Safe to re-run: every statement is idempotent. On a fresh volume MySQL runs
-- it automatically; on an existing volume apply it with `make db-init`.

CREATE DATABASE IF NOT EXISTS `interaction`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

-- The application account only gets the database it actually uses.
GRANT SELECT, INSERT, UPDATE, DELETE ON `interaction`.* TO 'shop'@'%';

USE `interaction`;

-- object_type: 1 goods, 2 article. object_id points into another service, so
-- there is no foreign key; existence is not checked until the goods service
-- exists.

CREATE TABLE IF NOT EXISTS `praise_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `object_type` TINYINT UNSIGNED NOT NULL COMMENT '1 goods, 2 article',
  `object_id` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  -- One praise per user per object. The index, not a read-then-insert check,
  -- is what prevents duplicates under concurrent requests.
  UNIQUE KEY `uk_praise_user_object` (`user_id`, `object_type`, `object_id`),
  KEY `idx_praise_object` (`object_type`, `object_id`)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS `collection_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `object_type` TINYINT UNSIGNED NOT NULL COMMENT '1 goods, 2 article',
  `object_id` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_collection_user_object` (`user_id`, `object_type`, `object_id`),
  KEY `idx_collection_object` (`object_type`, `object_id`)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS `comment_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `parent_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0 for a top-level comment',
  `user_id` BIGINT UNSIGNED NOT NULL,
  `object_type` TINYINT UNSIGNED NOT NULL COMMENT '1 goods, 2 article',
  `object_id` BIGINT UNSIGNED NOT NULL,
  `content` VARCHAR(255) NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_comment_object` (`object_type`, `object_id`, `deleted_at`),
  KEY `idx_comment_user` (`user_id`),
  KEY `idx_comment_parent` (`parent_id`)
) ENGINE=InnoDB;
