-- Shopping cart (task 0016).
--
-- Safe to re-run, and safe on existing data. Apply it with `make db-init`.

-- One row per (shopper, goods): adding the same goods again adds to the
-- quantity instead of creating a second row, also when two adds race.
CREATE TABLE IF NOT EXISTS `goods`.`cart_item` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `goods_id` BIGINT UNSIGNED NOT NULL,
  `quantity` INT UNSIGNED NOT NULL COMMENT '1-99, like an order line',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  -- Compared with the order time when an order.placed event removes bought
  -- goods: a row changed after the order is a new intent and is left alone.
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cart_user_goods` (`user_id`, `goods_id`),
  KEY `idx_cart_user_id` (`user_id`, `id`)
) ENGINE=InnoDB;

-- Whether the shopper checked out from the cart. Copied into order.placed,
-- so the goods service knows to take the bought goods out of the cart.
SET @has := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'orders' AND table_name = 'order_info' AND column_name = 'from_cart');
SET @ddl := IF(@has = 0,
  'ALTER TABLE `orders`.`order_info` ADD COLUMN `from_cart` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''1: checked out from the cart'' AFTER `remark`',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
