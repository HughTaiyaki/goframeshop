-- After-sale review and per-goods refunds (task 0018).
--
-- Safe to re-run, and safe on existing data. Apply it with `make db-init`.

USE `orders`;

DROP PROCEDURE IF EXISTS `add_refund_column`;
DELIMITER //
CREATE PROCEDURE `add_refund_column`(IN col VARCHAR(64), IN ddl TEXT)
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
      WHERE table_schema = 'orders' AND table_name = 'refund_info' AND column_name = col) THEN
    SET @ddl := CONCAT('ALTER TABLE `refund_info` ADD COLUMN ', ddl);
    PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END //
DELIMITER ;

-- The goods line's subtotal, fixed when the request is made.
CALL `add_refund_column`('amount_cents', '`amount_cents` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''the goods line subtotal'' AFTER `reason`');
CALL `add_refund_column`('review_note', '`review_note` VARCHAR(255) NOT NULL DEFAULT '''' COMMENT ''why it was approved or rejected'' AFTER `status`');
CALL `add_refund_column`('reviewed_at', '`reviewed_at` DATETIME(3) NULL AFTER `review_note`');
CALL `add_refund_column`('refunded_at', '`refunded_at` DATETIME(3) NULL AFTER `reviewed_at`');
-- Failed refund calls; APPROVED rows are retried by the recovery worker.
CALL `add_refund_column`('attempts', '`attempts` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `refunded_at`');

DROP PROCEDURE `add_refund_column`;

-- Requests made before this migration get their line's subtotal.
UPDATE `refund_info` r
  JOIN `order_goods` g ON g.`order_id` = r.`order_id` AND g.`goods_id` = r.`goods_id`
  SET r.`amount_cents` = g.`subtotal_cents`
  WHERE r.`amount_cents` = 0;

-- The worker's scan for APPROVED requests still to be paid back.
SET @has := (SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = 'orders' AND table_name = 'refund_info' AND index_name = 'idx_refund_status');
SET @ddl := IF(@has = 0, 'ALTER TABLE `refund_info` ADD KEY `idx_refund_status` (`status`, `id`)', 'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- The simulator: a payment may now be refunded in several parts, so the
-- "one refund per payment" key becomes a plain index.
USE `paysim`;

SET @has := (SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = 'paysim' AND table_name = 'refund' AND index_name = 'idx_refund_payment');
SET @ddl := IF(@has = 0, 'ALTER TABLE `refund` ADD KEY `idx_refund_payment` (`payment_id`)', 'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has := (SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = 'paysim' AND table_name = 'refund' AND index_name = 'uk_refund_payment');
SET @ddl := IF(@has > 0, 'ALTER TABLE `refund` DROP INDEX `uk_refund_payment`', 'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
