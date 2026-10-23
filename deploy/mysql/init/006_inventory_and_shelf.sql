-- Inventory reservation and goods shelf status.
--
-- A migration on top of 005: safe to re-run, and safe on a database that 005
-- created earlier with data in it. Apply it with `make db-init`.
--
-- Stock moves out of goods_info into its own ledger, so that "how many can be
-- sold" is changed only by reservation transactions, never by a product form:
--
--   available  can be reserved now
--   reserved   held for an order that has not paid yet
--   sold       committed by a paid order
--
-- Reserve: available -> reserved. Commit: reserved -> sold.
-- Release: reserved -> available. Sold stock is never returned by Release.

USE `goods`;

CREATE TABLE IF NOT EXISTS `inventory` (
  `goods_id` BIGINT UNSIGNED NOT NULL,
  `available` INT UNSIGNED NOT NULL DEFAULT 0,
  `reserved` INT UNSIGNED NOT NULL DEFAULT 0,
  `sold` INT UNSIGNED NOT NULL DEFAULT 0,
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`goods_id`)
) ENGINE=InnoDB;

-- One row per (order, goods). A Release that arrives before its Reserve
-- writes a RELEASED row (a tombstone), so the late Reserve finds it and
-- refuses instead of creating a reservation for a closed order.
CREATE TABLE IF NOT EXISTS `reservation` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_id` VARCHAR(64) NOT NULL,
  `goods_id` BIGINT UNSIGNED NOT NULL,
  `quantity` INT UNSIGNED NOT NULL,
  `state` VARCHAR(16) NOT NULL COMMENT 'RESERVED, COMMITTED or RELEASED',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_reservation_order_goods` (`order_id`, `goods_id`)
) ENGINE=InnoDB;

-- The idempotency record of every Reserve / Commit / Release call, committed
-- in the same transaction as the stock change it describes. A retry with the
-- same operation_id returns the stored outcome; a retry with the same id but
-- different parameters (request_hash) is refused.
CREATE TABLE IF NOT EXISTS `inventory_operation` (
  `operation_id` VARCHAR(64) NOT NULL,
  `kind` VARCHAR(16) NOT NULL COMMENT 'RESERVE, COMMIT or RELEASE',
  `order_id` VARCHAR(64) NOT NULL,
  `request_hash` CHAR(64) NOT NULL COMMENT 'sha256 of the canonical request',
  `outcome` VARCHAR(32) NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`operation_id`),
  KEY `idx_inventory_operation_order` (`order_id`)
) ENGINE=InnoDB;

-- Conditional DDL: MySQL has no "ADD/DROP COLUMN IF [NOT] EXISTS", so each
-- step checks information_schema and runs a no-op when already applied.

-- 1. shelf status. Existing goods stay visible (on shelf); goods created from
--    now on start off shelf, because the service writes status explicitly.
SET @has_status := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'goods' AND table_name = 'goods_info' AND column_name = 'status');
SET @ddl := IF(@has_status = 0,
  'ALTER TABLE `goods_info` ADD COLUMN `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT ''1 on shelf, 2 off shelf'' AFTER `name`',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. move stock/sale into the ledger, once.
SET @has_stock := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'goods' AND table_name = 'goods_info' AND column_name = 'stock');
SET @dml := IF(@has_stock = 1,
  'INSERT IGNORE INTO `inventory` (`goods_id`, `available`, `sold`) SELECT `id`, `stock`, `sale` FROM `goods_info`',
  'DO 0');
PREPARE stmt FROM @dml; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := IF(@has_stock = 1,
  'ALTER TABLE `goods_info` DROP COLUMN `stock`, DROP COLUMN `sale`',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 3. the public catalogue filters on status.
SET @has_idx := (SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = 'goods' AND table_name = 'goods_info' AND index_name = 'idx_goods_status');
SET @ddl := IF(@has_idx = 0,
  'ALTER TABLE `goods_info` ADD KEY `idx_goods_status` (`status`, `deleted_at`)',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
