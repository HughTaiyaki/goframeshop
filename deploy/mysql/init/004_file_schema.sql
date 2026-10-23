-- Uploaded file metadata, owned by the file service. The bytes live in object
-- storage (Qiniu, or a local directory in development); only the reference
-- lives here, so other tables point at a file_id instead of copying URLs.
--
-- Safe to re-run. On an existing volume apply it with `make db-init`.

CREATE DATABASE IF NOT EXISTS `file`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

GRANT SELECT, INSERT, UPDATE, DELETE ON `file`.* TO 'shop'@'%';

USE `file`;

CREATE TABLE IF NOT EXISTS `file_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `uploader_id` BIGINT UNSIGNED NOT NULL COMMENT 'admin_info.id of the uploader',
  `name` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'original file name, for display only',
  `storage` VARCHAR(16) NOT NULL COMMENT 'qiniu or local',
  `object_key` VARCHAR(255) NOT NULL COMMENT 'key inside the storage backend',
  `url` VARCHAR(512) NOT NULL,
  `size` BIGINT UNSIGNED NOT NULL,
  `content_type` VARCHAR(64) NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_file_storage_key` (`storage`, `object_key`),
  KEY `idx_file_uploader` (`uploader_id`)
) ENGINE=InnoDB;
