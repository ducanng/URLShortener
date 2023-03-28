CREATE DATABASE IF NOT EXISTS `url_shortener` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci */;
USE `url_shortener`;
-- Path: internal\config\schema.sql
CREATE TABLE IF NOT EXISTS `shortened_urls` (
  `id` char(8) NOT NULL,
  `original_url` LONGTEXT NOT NULL,
  `short_url` varchar(255) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `clicks` int(11) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `users` (
  `id` char(10) NOT NULL,
  `username` varchar(255) NOT NULL,
  `password` varchar(255) NOT NULL,
  `email` varchar(255) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `shortened_urls` (id, original_url, short_url) VALUES ('1234567888', 'https://www.google.com', 'http://localhost:8080/1234567888');
INSERT INTO `shortened_urls` (id, original_url, short_url) VALUES ('abcdefgh', 'https://www.facebook.com', 'http://localhost:8080/abcdefgh');
INSERT INTO `shortened_urls` (id, original_url, short_url) VALUES ('1234abcd', 'https://www.youtube.com', 'http://localhost:8080/1234abcd');