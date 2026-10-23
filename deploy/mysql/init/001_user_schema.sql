CREATE DATABASE IF NOT EXISTS `user`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

USE `user`;

CREATE TABLE IF NOT EXISTS `user_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(200) NOT NULL,
  `avatar` VARCHAR(512) NOT NULL DEFAULT '',
  `password_hash` VARCHAR(255) NOT NULL,
  `password_salt` VARBINARY(32) NOT NULL,
  `sex` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0 unknown, 1 male, 2 female',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1 active, 2 blocked',
  `sign` VARCHAR(255) NOT NULL DEFAULT '',
  `secret_answer_hash` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_info_name` (`name`),
  KEY `idx_user_info_status` (`status`),
  KEY `idx_user_info_deleted_at` (`deleted_at`)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS `consignee_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `is_default` TINYINT UNSIGNED NOT NULL DEFAULT 0,
  `name` VARCHAR(64) NOT NULL,
  `phone` VARCHAR(32) NOT NULL,
  `province` VARCHAR(64) NOT NULL DEFAULT '',
  `city` VARCHAR(64) NOT NULL DEFAULT '',
  `town` VARCHAR(64) NOT NULL DEFAULT '',
  `street` VARCHAR(128) NOT NULL DEFAULT '',
  `detail` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_consignee_user_id` (`user_id`),
  KEY `idx_consignee_user_default` (`user_id`, `is_default`),
  KEY `idx_consignee_deleted_at` (`deleted_at`)
) ENGINE=InnoDB;
