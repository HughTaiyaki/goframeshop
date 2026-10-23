-- Hot recommendations: goods_info.sort.
--
-- sort is a recommendation weight set by an administrator. 0 (the default)
-- means "not hot"; any value above 0 puts the goods in the hot list, and a
-- larger value comes first. It is unrelated to goods_images.sort, which
-- orders the images of one goods (smaller first).
--
-- Safe to re-run, and safe on existing data: every existing goods starts as
-- not hot. Apply it with `make db-init`.

USE `goods`;

SET @has := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'goods' AND table_name = 'goods_info' AND column_name = 'sort');
SET @ddl := IF(@has = 0,
  'ALTER TABLE `goods_info` ADD COLUMN `sort` SMALLINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''hot weight: 0 not hot, larger first'' AFTER `status`',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- The hot list reads "on shelf, not deleted, sort > 0, ORDER BY sort DESC".
SET @has := (SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = 'goods' AND table_name = 'goods_info' AND index_name = 'idx_goods_hot');
SET @ddl := IF(@has = 0,
  'ALTER TABLE `goods_info` ADD KEY `idx_goods_hot` (`status`, `sort`)',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
