-- Outbox / Inbox tables (design §8), one set in each database that publishes
-- or consumes events:
--   orders  publishes order.payment_pending, consumes order.expiry.due
--   goods   publishes goods.changed
--   banner  consumes goods.changed
-- Each service only touches the tables in its own database.
--
-- Also: the banner slots learn whether their goods can still be sold.
--
-- Safe to re-run. Apply it with `make db-init`.

USE `orders`;

-- Events this service has to publish. Written in the SAME transaction as the
-- business change, so "the order became payable" and "tell the others" are
-- committed together or not at all. The relay publishes PENDING rows and
-- marks them SENT once RabbitMQ confirms; a crash in between re-sends the
-- same event_id, which consumers recognise.
CREATE TABLE IF NOT EXISTS `outbox_event` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'also the aggregate version: rows of one aggregate are written under its row lock',
  `event_id` CHAR(36) NOT NULL,
  `type` VARCHAR(64) NOT NULL COMMENT 'also the routing key',
  `aggregate_id` VARCHAR(64) NOT NULL,
  `payload` JSON NOT NULL,
  `deliver_at` DATETIME(3) NULL COMMENT 'delayed event: consumers get it at this time',
  `state` VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING or SENT',
  `attempts` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_attempt_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `owner` VARCHAR(64) NULL COMMENT 'relay instance holding the lease',
  `lease_until` DATETIME(3) NULL,
  `last_error` VARCHAR(255) NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `sent_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_outbox_event` (`event_id`),
  KEY `idx_outbox_due` (`state`, `next_attempt_at`)
) ENGINE=InnoDB;

-- Events this service has consumed. The row is inserted in the SAME
-- transaction as the business change it caused, so a redelivered event finds
-- its row and is only acknowledged, never applied twice.
CREATE TABLE IF NOT EXISTS `inbox_event` (
  `consumer` VARCHAR(64) NOT NULL,
  `event_id` CHAR(36) NOT NULL,
  `payload_hash` CHAR(64) NOT NULL,
  `completed_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`consumer`, `event_id`)
) ENGINE=InnoDB;

-- Messages that can never be processed (malformed, or an event id reused
-- with a different payload). Kept for a person to look at, then acknowledged
-- so they do not block the queue.
CREATE TABLE IF NOT EXISTS `quarantine_event` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `consumer` VARCHAR(64) NOT NULL,
  `event_id` VARCHAR(64) NOT NULL DEFAULT '',
  `reason` VARCHAR(255) NOT NULL,
  `body` MEDIUMBLOB NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;

USE `goods`;

-- Events this service has to publish. Written in the SAME transaction as the
-- business change, so "the order became payable" and "tell the others" are
-- committed together or not at all. The relay publishes PENDING rows and
-- marks them SENT once RabbitMQ confirms; a crash in between re-sends the
-- same event_id, which consumers recognise.
CREATE TABLE IF NOT EXISTS `outbox_event` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'also the aggregate version: rows of one aggregate are written under its row lock',
  `event_id` CHAR(36) NOT NULL,
  `type` VARCHAR(64) NOT NULL COMMENT 'also the routing key',
  `aggregate_id` VARCHAR(64) NOT NULL,
  `payload` JSON NOT NULL,
  `deliver_at` DATETIME(3) NULL COMMENT 'delayed event: consumers get it at this time',
  `state` VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING or SENT',
  `attempts` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_attempt_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `owner` VARCHAR(64) NULL COMMENT 'relay instance holding the lease',
  `lease_until` DATETIME(3) NULL,
  `last_error` VARCHAR(255) NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `sent_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_outbox_event` (`event_id`),
  KEY `idx_outbox_due` (`state`, `next_attempt_at`)
) ENGINE=InnoDB;

-- Events this service has consumed. The row is inserted in the SAME
-- transaction as the business change it caused, so a redelivered event finds
-- its row and is only acknowledged, never applied twice.
CREATE TABLE IF NOT EXISTS `inbox_event` (
  `consumer` VARCHAR(64) NOT NULL,
  `event_id` CHAR(36) NOT NULL,
  `payload_hash` CHAR(64) NOT NULL,
  `completed_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`consumer`, `event_id`)
) ENGINE=InnoDB;

-- Messages that can never be processed (malformed, or an event id reused
-- with a different payload). Kept for a person to look at, then acknowledged
-- so they do not block the queue.
CREATE TABLE IF NOT EXISTS `quarantine_event` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `consumer` VARCHAR(64) NOT NULL,
  `event_id` VARCHAR(64) NOT NULL DEFAULT '',
  `reason` VARCHAR(255) NOT NULL,
  `body` MEDIUMBLOB NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;

USE `banner`;

-- Events this service has to publish. Written in the SAME transaction as the
-- business change, so "the order became payable" and "tell the others" are
-- committed together or not at all. The relay publishes PENDING rows and
-- marks them SENT once RabbitMQ confirms; a crash in between re-sends the
-- same event_id, which consumers recognise.
CREATE TABLE IF NOT EXISTS `outbox_event` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'also the aggregate version: rows of one aggregate are written under its row lock',
  `event_id` CHAR(36) NOT NULL,
  `type` VARCHAR(64) NOT NULL COMMENT 'also the routing key',
  `aggregate_id` VARCHAR(64) NOT NULL,
  `payload` JSON NOT NULL,
  `deliver_at` DATETIME(3) NULL COMMENT 'delayed event: consumers get it at this time',
  `state` VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING or SENT',
  `attempts` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_attempt_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `owner` VARCHAR(64) NULL COMMENT 'relay instance holding the lease',
  `lease_until` DATETIME(3) NULL,
  `last_error` VARCHAR(255) NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `sent_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_outbox_event` (`event_id`),
  KEY `idx_outbox_due` (`state`, `next_attempt_at`)
) ENGINE=InnoDB;

-- Events this service has consumed. The row is inserted in the SAME
-- transaction as the business change it caused, so a redelivered event finds
-- its row and is only acknowledged, never applied twice.
CREATE TABLE IF NOT EXISTS `inbox_event` (
  `consumer` VARCHAR(64) NOT NULL,
  `event_id` CHAR(36) NOT NULL,
  `payload_hash` CHAR(64) NOT NULL,
  `completed_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`consumer`, `event_id`)
) ENGINE=InnoDB;

-- Messages that can never be processed (malformed, or an event id reused
-- with a different payload). Kept for a person to look at, then acknowledged
-- so they do not block the queue.
CREATE TABLE IF NOT EXISTS `quarantine_event` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `consumer` VARCHAR(64) NOT NULL,
  `event_id` VARCHAR(64) NOT NULL DEFAULT '',
  `reason` VARCHAR(255) NOT NULL,
  `body` MEDIUMBLOB NOT NULL,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;

USE `banner`;

-- What the latest goods.changed said about the slot's goods. goods_version
-- only moves forward, so an older event arriving late cannot undo a newer one.
SET @has := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = 'banner' AND table_name = 'position_info' AND column_name = 'goods_on_shelf');
SET @ddl := IF(@has = 0,
  'ALTER TABLE `position_info` ADD COLUMN `goods_on_shelf` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT ''0 once the goods is off shelf or deleted'' AFTER `goods_name`, ADD COLUMN `goods_version` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''version of the last goods.changed applied'' AFTER `goods_on_shelf`, ADD KEY `idx_position_goods` (`goods_id`)',
  'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
