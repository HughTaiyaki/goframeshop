-- Shipping and receipt (task 0017): PAID -> SHIPPED -> COMPLETED.
--
-- Safe to re-run, and safe on existing data: existing orders get empty
-- shipping fields. Apply it with `make db-init`.

USE `orders`;

-- Adds one column to order_info unless it is already there.
DROP PROCEDURE IF EXISTS `add_order_column`;
DELIMITER //
CREATE PROCEDURE `add_order_column`(IN col VARCHAR(64), IN ddl TEXT)
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
      WHERE table_schema = 'orders' AND table_name = 'order_info' AND column_name = col) THEN
    SET @ddl := CONCAT('ALTER TABLE `order_info` ADD COLUMN ', ddl);
    PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END //
DELIMITER ;

CALL `add_order_column`('carrier', '`carrier` VARCHAR(32) NOT NULL DEFAULT '''' COMMENT ''shipping company'' AFTER `paid_at`');
CALL `add_order_column`('tracking_no', '`tracking_no` VARCHAR(64) NOT NULL DEFAULT '''' COMMENT ''waybill number'' AFTER `carrier`');
CALL `add_order_column`('shipped_at', '`shipped_at` DATETIME(3) NULL AFTER `tracking_no`');
-- SHIPPED orders past this are confirmed by the recovery worker.
CALL `add_order_column`('confirm_deadline', '`confirm_deadline` DATETIME(3) NULL AFTER `shipped_at`');
CALL `add_order_column`('completed_at', '`completed_at` DATETIME(3) NULL AFTER `confirm_deadline`');
CALL `add_order_column`('complete_reason', '`complete_reason` VARCHAR(16) NOT NULL DEFAULT '''' COMMENT ''USER_CONFIRMED or AUTO_CONFIRMED'' AFTER `completed_at`');

DROP PROCEDURE `add_order_column`;

-- The worker's scan for SHIPPED orders past confirm_deadline.
SET @has := (SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = 'orders' AND table_name = 'order_info' AND index_name = 'idx_order_confirm');
SET @ddl := IF(@has = 0,
  'ALTER TABLE `order_info` ADD KEY `idx_order_confirm` (`status`, `confirm_deadline`)',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
