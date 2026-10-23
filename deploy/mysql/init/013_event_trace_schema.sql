-- Trace context for events (design §9.1): the W3C traceparent of the request
-- that wrote the event, so the relay and the consumer continue the same trace
-- (or, for a delayed event, link back to it).
--
-- Safe to re-run. Apply it with `make db-init`.

SET @has := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'orders' AND table_name = 'outbox_event' AND column_name = 'traceparent');
SET @ddl := IF(@has = 0,
  'ALTER TABLE `orders`.`outbox_event` ADD COLUMN `traceparent` VARCHAR(64) NULL COMMENT ''W3C trace context of the writer'' AFTER `deliver_at`',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'goods' AND table_name = 'outbox_event' AND column_name = 'traceparent');
SET @ddl := IF(@has = 0,
  'ALTER TABLE `goods`.`outbox_event` ADD COLUMN `traceparent` VARCHAR(64) NULL COMMENT ''W3C trace context of the writer'' AFTER `deliver_at`',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'banner' AND table_name = 'outbox_event' AND column_name = 'traceparent');
SET @ddl := IF(@has = 0,
  'ALTER TABLE `banner`.`outbox_event` ADD COLUMN `traceparent` VARCHAR(64) NULL COMMENT ''W3C trace context of the writer'' AFTER `deliver_at`',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
