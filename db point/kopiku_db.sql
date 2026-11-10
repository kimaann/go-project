-- phpMyAdmin SQL Dump
-- version 5.2.3
-- https://www.phpmyadmin.net/
--
-- Host: 127.0.0.1
-- Generation Time: Nov 10, 2026 at 09:44 PM
-- Server version: 10.11.16-MariaDB
-- PHP Version: 8.5.12

SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
START TRANSACTION;
SET time_zone = "+00:00";


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;

--
-- Database: `kopiku_db`
--

-- --------------------------------------------------------

--
-- Table structure for table `point_ledger`
--

CREATE TABLE `point_ledger` (
  `id` bigint(20) UNSIGNED NOT NULL,
  `user_id` varchar(64) NOT NULL,
  `lot_id` bigint(20) UNSIGNED DEFAULT NULL,
  `redemption_id` bigint(20) UNSIGNED DEFAULT NULL,
  `order_id` varchar(64) DEFAULT NULL,
  `entry_type` varchar(16) NOT NULL,
  `points_delta` int(11) NOT NULL,
  `balance_after` int(11) NOT NULL,
  `note` varchar(255) NOT NULL DEFAULT '',
  `created_at` timestamp NOT NULL DEFAULT current_timestamp()
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Dumping data for table `point_ledger`
--

INSERT INTO `point_ledger` (`id`, `user_id`, `lot_id`, `redemption_id`, `order_id`, `entry_type`, `points_delta`, `balance_after`, `note`, `created_at`) VALUES
(1, 'u3', 1, NULL, NULL, 'earn', 350, 350, 'Migrasi saldo poin lama', '2026-11-10 05:30:16'),
(2, 'u4', 2, NULL, NULL, 'earn', 1200, 1200, 'Migrasi saldo poin lama', '2026-11-10 05:30:16'),
(82, 'u3', 1, 18, NULL, 'redeem', -100, 250, 'Penukaran poin dari halaman poin', '2026-11-10 06:28:53'),
(83, 'u3', 46, NULL, 'ORD-323131', 'earn', 28, 278, 'Poin transaksi ORD-323131', '2026-11-10 06:30:21'),
(84, 'u3', 1, 19, NULL, 'redeem', -100, 178, 'Penukaran poin dari halaman poin', '2026-11-10 06:31:53'),
(85, 'u3', 47, NULL, 'ORD-371204', 'earn', 20, 198, 'Poin transaksi ORD-371204', '2026-11-10 06:33:11');

-- --------------------------------------------------------

--
-- Table structure for table `point_lots`
--

CREATE TABLE `point_lots` (
  `id` bigint(20) UNSIGNED NOT NULL,
  `user_id` varchar(64) NOT NULL,
  `order_id` varchar(64) DEFAULT NULL,
  `points_total` int(11) NOT NULL,
  `points_remaining` int(11) NOT NULL,
  `earned_at` datetime NOT NULL,
  `expires_at` datetime NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'active',
  `note` varchar(255) NOT NULL DEFAULT '',
  `created_at` timestamp NOT NULL DEFAULT current_timestamp(),
  `updated_at` timestamp NOT NULL DEFAULT current_timestamp() ON UPDATE current_timestamp()
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Dumping data for table `point_lots`
--

INSERT INTO `point_lots` (`id`, `user_id`, `order_id`, `points_total`, `points_remaining`, `earned_at`, `expires_at`, `status`, `note`, `created_at`, `updated_at`) VALUES
(1, 'u3', NULL, 350, 150, '2026-11-10 12:30:16', '2027-05-10 12:30:16', 'active', 'Migrasi saldo poin lama', '2026-11-10 05:30:16', '2026-11-10 06:31:53'),
(2, 'u4', NULL, 1200, 1200, '2026-11-10 12:30:16', '2027-05-10 12:30:16', 'active', 'Migrasi saldo poin lama', '2026-11-10 05:30:16', '2026-11-10 05:30:16'),
(46, 'u3', 'ORD-323131', 28, 28, '2026-11-10 13:30:21', '2027-05-10 13:30:21', 'active', 'Poin transaksi ORD-323131', '2026-11-10 06:30:21', '2026-11-10 06:30:21'),
(47, 'u3', 'ORD-371204', 20, 20, '2026-11-10 13:33:11', '2027-05-10 13:33:11', 'active', 'Poin transaksi ORD-371204', '2026-11-10 06:33:11', '2026-11-10 06:33:11');

-- --------------------------------------------------------

--
-- Table structure for table `point_redemptions`
--

CREATE TABLE `point_redemptions` (
  `id` bigint(20) UNSIGNED NOT NULL,
  `user_id` varchar(64) NOT NULL,
  `order_id` varchar(64) DEFAULT NULL,
  `points` int(11) NOT NULL,
  `rupiah_value` bigint(20) NOT NULL,
  `allocations` longtext DEFAULT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'active',
  `voucher_code` varchar(40) DEFAULT NULL,
  `note` varchar(255) NOT NULL DEFAULT '',
  `created_at` timestamp NOT NULL DEFAULT current_timestamp(),
  `reversed_at` datetime DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Dumping data for table `point_redemptions`
--

INSERT INTO `point_redemptions` (`id`, `user_id`, `order_id`, `points`, `rupiah_value`, `allocations`, `status`, `voucher_code`, `note`, `created_at`, `reversed_at`) VALUES
(18, 'u3', NULL, 100, 5000, '[{\"lot_id\":1,\"points\":100}]', 'active', 'PTS-8XATSB', 'Penukaran poin dari halaman poin', '2026-11-10 06:28:53', NULL),
(19, 'u3', NULL, 100, 5000, '[{\"lot_id\":1,\"points\":100}]', 'active', 'PTS-MYAJ57', 'Penukaran poin dari halaman poin', '2026-11-10 06:31:53', NULL);

-- --------------------------------------------------------

--
-- Table structure for table `users`
--

CREATE TABLE `users` (
  `id` varchar(36) NOT NULL,
  `name` varchar(100) NOT NULL,
  `username` varchar(50) NOT NULL,
  `phone` varchar(20) NOT NULL,
  `email` varchar(191) NOT NULL,
  `password_hash` varchar(255) NOT NULL,
  `role` enum('user','kasir','admin') NOT NULL DEFAULT 'user',
  `points` int(10) UNSIGNED NOT NULL DEFAULT 0,
  `is_active` tinyint(1) NOT NULL DEFAULT 1,
  `last_login_at` datetime DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT current_timestamp(),
  `updated_at` timestamp NOT NULL DEFAULT current_timestamp() ON UPDATE current_timestamp()
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Dumping data for table `users`
--

INSERT INTO `users` (`id`, `name`, `username`, `phone`, `email`, `password_hash`, `role`, `points`, `is_active`, `last_login_at`, `created_at`, `updated_at`) VALUES
('u1', 'Administrator', 'admin', '08111111111', 'admin@kopiku.id', '$2a$12$hqsacyy1c3YEX7/gW/t6y.vbUBSU1.UxaVvyby0hZpY5YmwBf4ZRK', 'admin', 0, 1, '2026-11-10 12:34:59', '2026-11-08 08:26:35', '2026-11-10 05:34:59'),
('u2', 'Kasir Sudirman', 'kasir', '08222222222', 'kasir@kopiku.id', '$2a$12$2C1CpFmWYs78nmi2SwCo1eZlXQ6GysdWsNJUsF.ure5qKBaBE66Ya', 'kasir', 0, 1, NULL, '2026-11-08 08:26:35', '2026-11-08 08:26:35'),
('u3', 'Budi Santoso', 'user1', '08333333333', 'budi@email.com', '$2a$12$6DMkNaccvUpweNv4RLpuiu1.ExU2WLNw9enywQoTyCg.heQwWzICO', 'user', 198, 1, '2026-11-10 13:27:30', '2026-11-08 08:26:35', '2026-11-10 06:33:11'),
('u4', 'Siti Rahayu', 'user2', '08444444444', 'siti@email.com', '$2a$12$4/dMVpEZEtWIzk/WdsUKo.45dF0gEqjV2cdyoPRecQWQb4noko4qO', 'user', 1200, 1, NULL, '2026-11-08 08:26:35', '2026-11-08 08:26:35');

--
-- Indexes for dumped tables
--

--
-- Indexes for table `point_ledger`
--
ALTER TABLE `point_ledger`
  ADD PRIMARY KEY (`id`),
  ADD KEY `idx_point_ledger_user` (`user_id`,`id`),
  ADD KEY `idx_point_ledger_order` (`order_id`,`entry_type`),
  ADD KEY `idx_point_ledger_lot` (`lot_id`);

--
-- Indexes for table `point_lots`
--
ALTER TABLE `point_lots`
  ADD PRIMARY KEY (`id`),
  ADD KEY `idx_point_lots_user_fifo` (`user_id`,`status`,`expires_at`,`id`),
  ADD KEY `idx_point_lots_order` (`order_id`);

--
-- Indexes for table `point_redemptions`
--
ALTER TABLE `point_redemptions`
  ADD PRIMARY KEY (`id`),
  ADD KEY `idx_point_redemptions_user` (`user_id`,`created_at`),
  ADD KEY `idx_point_redemptions_order` (`order_id`,`status`);

--
-- Indexes for table `users`
--
ALTER TABLE `users`
  ADD PRIMARY KEY (`id`),
  ADD UNIQUE KEY `uq_users_username` (`username`),
  ADD UNIQUE KEY `uq_users_email` (`email`),
  ADD KEY `idx_users_role_active` (`role`,`is_active`);

--
-- AUTO_INCREMENT for dumped tables
--

--
-- AUTO_INCREMENT for table `point_ledger`
--
ALTER TABLE `point_ledger`
  MODIFY `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT, AUTO_INCREMENT=86;

--
-- AUTO_INCREMENT for table `point_lots`
--
ALTER TABLE `point_lots`
  MODIFY `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT, AUTO_INCREMENT=48;

--
-- AUTO_INCREMENT for table `point_redemptions`
--
ALTER TABLE `point_redemptions`
  MODIFY `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT, AUTO_INCREMENT=20;
COMMIT;

/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
