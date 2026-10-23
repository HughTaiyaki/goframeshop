-- The payment simulator's own database. It stands in for an external payment
-- provider: the order service can only reach it through its API and never
-- reads or writes these tables (design §4: the simulator keeps its own facts).
--
-- Safe to re-run. Apply it with `make db-init`.

CREATE DATABASE IF NOT EXISTS `paysim`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

GRANT SELECT, INSERT, UPDATE, DELETE ON `paysim`.* TO 'shop'@'%';

USE `paysim`;

-- One row per payment. Pay and Close are decided on this row inside one
-- transaction, so "paid" and "closed" can never both happen.
--   CREATED    waiting for the payer
--   SUCCEEDED  paid; provider_tx_id set; the order service is notified
--   CLOSED     closed before payment (also a tombstone when Close came first)
--   REFUNDED   paid, then fully refunded
CREATE TABLE IF NOT EXISTS `payment` (
  `payment_id` VARCHAR(40) NOT NULL COMMENT 'chosen by the merchant, stable per order',
  `order_no` VARCHAR(32) NOT NULL,
  `amount_cents` BIGINT UNSIGNED NOT NULL COMMENT '0 for a tombstone written by Close',
  `currency` CHAR(3) NOT NULL DEFAULT 'CNY',
  `state` VARCHAR(16) NOT NULL,
  `expires_at` DATETIME(3) NULL COMMENT 'the payer cannot pay after this',
  `provider_tx_id` VARCHAR(40) NULL,
  -- Notification bookkeeping: the merchant is notified until it acknowledges.
  -- next_notify_at NULL means "do not notify" (acknowledged, given up, or a
  -- payment made with notifications switched off to simulate a lost one).
  `notify_attempts` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_notify_at` DATETIME(3) NULL,
  `paid_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`payment_id`),
  UNIQUE KEY `uk_payment_tx` (`provider_tx_id`),
  KEY `idx_payment_notify` (`next_notify_at`)
) ENGINE=InnoDB;

-- Refunds are idempotent by refund_id: the same id twice refunds once.
CREATE TABLE IF NOT EXISTS `refund` (
  `refund_id` VARCHAR(48) NOT NULL,
  `payment_id` VARCHAR(40) NOT NULL,
  `amount_cents` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`refund_id`),
  UNIQUE KEY `uk_refund_payment` (`payment_id`)
) ENGINE=InnoDB;
