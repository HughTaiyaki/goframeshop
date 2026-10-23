-- Orders and after-sale requests, owned by the order service.
--
-- The database is `orders`, not `order`: ORDER is a reserved word and would
-- need quoting in every statement.
--
-- Safe to re-run. On an existing volume apply it with `make db-init`.

CREATE DATABASE IF NOT EXISTS `orders`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

GRANT SELECT, INSERT, UPDATE, DELETE ON `orders`.* TO 'shop'@'%';

USE `orders`;

-- One row per order. Everything a shopper saw when ordering is copied in
-- (prices, address), so later edits to goods or addresses never change an
-- existing order.
--
-- status follows docs/GoFrame微服务商城详细设计.md §6.1:
--   RESERVING        created; stock reservation in progress
--   PENDING_PAYMENT  stock held until expires_at
--   CLOSING          cancelled or expired; stock being released
--   CLOSED           finished without payment (close_reason says why)
--   PAID ...         added with the payment simulator in a later task
CREATE TABLE IF NOT EXISTS `order_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_no` VARCHAR(32) NOT NULL COMMENT 'public order number; also the inventory order id',
  `user_id` BIGINT UNSIGNED NOT NULL,
  -- Idempotency: the client sends a request id with each submit. A retry
  -- with the same id returns the same order instead of creating a second one.
  `request_id` VARCHAR(64) NOT NULL,
  `request_hash` CHAR(64) NOT NULL COMMENT 'sha256 of the normalised request',
  `status` VARCHAR(20) NOT NULL,
  `close_reason` VARCHAR(32) NOT NULL DEFAULT '',
  `total_cents` BIGINT UNSIGNED NOT NULL COMMENT 'computed by the server from goods prices',
  `consignee_name` VARCHAR(64) NOT NULL,
  `consignee_phone` VARCHAR(32) NOT NULL,
  `consignee_address` VARCHAR(512) NOT NULL,
  `remark` VARCHAR(255) NOT NULL DEFAULT '',
  -- RESERVING must settle by reserve_deadline; PENDING_PAYMENT ends at
  -- expires_at. The recovery worker closes whatever passes its deadline.
  `reserve_deadline` DATETIME(3) NOT NULL,
  `expires_at` DATETIME(3) NULL,
  `closed_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  UNIQUE KEY `uk_order_user_request` (`user_id`, `request_id`),
  KEY `idx_order_user` (`user_id`, `id`),
  KEY `idx_order_status` (`status`, `expires_at`)
) ENGINE=InnoDB;

-- One row per goods in an order, with the price and name at the time.
CREATE TABLE IF NOT EXISTS `order_goods` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_id` BIGINT UNSIGNED NOT NULL,
  `goods_id` BIGINT UNSIGNED NOT NULL,
  `goods_name` VARCHAR(200) NOT NULL,
  `cover_url` VARCHAR(512) NOT NULL DEFAULT '',
  `unit_price_cents` BIGINT UNSIGNED NOT NULL,
  `quantity` INT UNSIGNED NOT NULL,
  `subtotal_cents` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_goods` (`order_id`, `goods_id`)
) ENGINE=InnoDB;

-- After-sale requests. Only a paid order can have one, and only for goods in
-- that order; at most one request per (order, goods).
CREATE TABLE IF NOT EXISTS `refund_info` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `refund_no` VARCHAR(32) NOT NULL,
  `order_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `goods_id` BIGINT UNSIGNED NOT NULL,
  `reason` VARCHAR(255) NOT NULL,
  `status` VARCHAR(16) NOT NULL COMMENT 'PENDING, APPROVED or REJECTED',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_refund_no` (`refund_no`),
  UNIQUE KEY `uk_refund_order_goods` (`order_id`, `goods_id`),
  KEY `idx_refund_user` (`user_id`, `id`)
) ENGINE=InnoDB;
