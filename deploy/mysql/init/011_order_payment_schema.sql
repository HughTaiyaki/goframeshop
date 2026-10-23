-- Payments as the order service records them (design §5.1), plus technical
-- refunds for payments that arrive after their order was closed.
--
-- Safe to re-run, including on an `orders` database created by 008.

USE `orders`;

-- The order service's own record of each order's payment. payment_id is
-- derived from the order number ("P" + order_no), so retries and the expiry
-- worker always name the same payment at the simulator.
--   CREATED    asked the provider to create it
--   SUCCEEDED  the provider confirmed payment (callback or query)
--   CLOSED     the provider closed it unpaid
CREATE TABLE IF NOT EXISTS `payment` (
  `payment_id` VARCHAR(40) NOT NULL,
  `order_id` BIGINT UNSIGNED NOT NULL,
  `amount_cents` BIGINT UNSIGNED NOT NULL,
  `state` VARCHAR(16) NOT NULL,
  `provider_tx_id` VARCHAR(40) NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`payment_id`),
  UNIQUE KEY `uk_payment_order` (`order_id`),
  UNIQUE KEY `uk_payment_tx` (`provider_tx_id`)
) ENGINE=InnoDB;

-- A payment that succeeded although its order is closing or closed (or whose
-- stock could not be committed) is refunded automatically. refund_id is fixed
-- per payment ("late-" + payment_id), so the refund happens at most once.
CREATE TABLE IF NOT EXISTS `tech_refund` (
  `refund_id` VARCHAR(48) NOT NULL,
  `payment_id` VARCHAR(40) NOT NULL,
  `order_id` BIGINT UNSIGNED NOT NULL,
  `amount_cents` BIGINT UNSIGNED NOT NULL,
  `reason` VARCHAR(32) NOT NULL COMMENT 'LATE_PAYMENT or COMMIT_FAILED',
  `state` VARCHAR(16) NOT NULL COMMENT 'PENDING or SUCCEEDED',
  `attempts` INT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`refund_id`),
  UNIQUE KEY `uk_tech_refund_payment` (`payment_id`),
  KEY `idx_tech_refund_state` (`state`)
) ENGINE=InnoDB;

SET @has_paid := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'orders' AND table_name = 'order_info' AND column_name = 'paid_at');
SET @ddl := IF(@has_paid = 0,
  'ALTER TABLE `order_info` ADD COLUMN `paid_at` DATETIME(3) NULL AFTER `closed_at`',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
