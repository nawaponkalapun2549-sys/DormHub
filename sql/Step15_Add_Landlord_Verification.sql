-- DORMHUB STEP 15: SAFE ADD-ON MIGRATION
-- Select EXISTING `DormHub` database; this DOES NOT DROP or replace any table.
-- For new users choosing Landlord, approval is required before their role changes.
USE `DormHub`;
CREATE TABLE IF NOT EXISTS `landlord_applications` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `user_id` INT NOT NULL,
  `dorm_name` VARCHAR(150) NOT NULL,
  `document_path` VARCHAR(255) NOT NULL,
  `original_filename` VARCHAR(255) NOT NULL,
  `status` ENUM('pending','approved','rejected') NOT NULL DEFAULT 'pending',
  `review_note` VARCHAR(255) NOT NULL DEFAULT '',
  `reviewed_by` INT NULL,
  `reviewed_at` DATETIME NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_landlord_application_user` (`user_id`),
  KEY `idx_landlord_application_status` (`status`, `created_at`),
  CONSTRAINT `fk_landlord_application_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_landlord_application_reviewer` FOREIGN KEY (`reviewed_by`) REFERENCES `users`(`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SHOW TABLES;
