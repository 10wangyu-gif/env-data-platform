-- 初始化数据库脚本

-- 设置字符集
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- 创建数据库（如果不存在）
CREATE DATABASE IF NOT EXISTS `env_data_platform`
DEFAULT CHARACTER SET utf8mb4
COLLATE utf8mb4_unicode_ci;

-- 使用数据库
USE `env_data_platform`;

-- 设置时区
SET time_zone = '+08:00';

-- 优化配置
SET sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';

-- 创建用户权限（仅在开发环境）
-- GRANT ALL PRIVILEGES ON env_data_platform.* TO 'env_user'@'%' IDENTIFIED BY 'env_password';
-- FLUSH PRIVILEGES;

SET FOREIGN_KEY_CHECKS = 1;