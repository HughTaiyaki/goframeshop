-- Back-office administrators, owned by the admin service.
--
-- Safe to re-run. On an existing volume apply it with `make db-init`.

CREATE DATABASE IF NOT EXISTS `admin`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

GRANT SELECT, INSERT, UPDATE, DELETE ON `admin`.* TO 'shop'@'%';

USE `admin`;

-- Roles and permissions from the original tutorial are not modelled yet: the
-- only distinction today is whether an administrator may create others.
CREATE TABLE IF NOT EXISTS `admin_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `password_salt` VARBINARY(32) NOT NULL,
  `is_super` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '1 may create administrators',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_admin_info_name` (`name`)
) ENGINE=InnoDB;
