-- SQL Server initialization script
-- Creates test schema and tables

USE master;
GO

-- Create database if it doesn't exist
IF NOT EXISTS (SELECT name FROM sys.databases WHERE name = N'test_db')
BEGIN
    CREATE DATABASE test_db;
END;
GO

-- Switch to test_db
USE test_db;
GO

-- Drop tables if they exist (in reverse order due to foreign keys)
IF OBJECT_ID('dbo.comments', 'U') IS NOT NULL DROP TABLE dbo.comments;
IF OBJECT_ID('dbo.posts', 'U') IS NOT NULL DROP TABLE dbo.posts;
IF OBJECT_ID('dbo.users', 'U') IS NOT NULL DROP TABLE dbo.users;
GO

-- Create users table
CREATE TABLE users (
    id INT PRIMARY KEY IDENTITY(1,1),
    name NVARCHAR(100) NOT NULL,
    email NVARCHAR(100) UNIQUE NOT NULL,
    age INT,
    status NVARCHAR(50) DEFAULT 'active',
    created_at DATETIME DEFAULT GETDATE()
);
GO

-- Create posts table
CREATE TABLE posts (
    id INT PRIMARY KEY IDENTITY(1,1),
    user_id INT NOT NULL,
    title NVARCHAR(255) NOT NULL,
    content NVARCHAR(MAX),
    published BIT DEFAULT 0,
    created_at DATETIME DEFAULT GETDATE(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE NO ACTION ON UPDATE NO ACTION
);
GO

-- Create comments table
CREATE TABLE comments (
    id INT PRIMARY KEY IDENTITY(1,1),
    post_id INT NOT NULL,
    user_id INT NOT NULL,
    content NVARCHAR(MAX) NOT NULL,
    created_at DATETIME DEFAULT GETDATE(),
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE NO ACTION ON UPDATE NO ACTION,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE NO ACTION ON UPDATE NO ACTION
);
GO

-- Create indexes for common queries
IF NOT EXISTS (SELECT name FROM sys.indexes WHERE name = N'idx_users_email' AND object_id = OBJECT_ID(N'dbo.users'))
BEGIN
    CREATE INDEX idx_users_email ON dbo.users(email);
END;
GO

IF NOT EXISTS (SELECT name FROM sys.indexes WHERE name = N'idx_users_status' AND object_id = OBJECT_ID(N'dbo.users'))
BEGIN
    CREATE INDEX idx_users_status ON dbo.users(status);
END;
GO

IF NOT EXISTS (SELECT name FROM sys.indexes WHERE name = N'idx_posts_user_id' AND object_id = OBJECT_ID(N'dbo.posts'))
BEGIN
    CREATE INDEX idx_posts_user_id ON dbo.posts(user_id);
END;
GO

IF NOT EXISTS (SELECT name FROM sys.indexes WHERE name = N'idx_posts_published' AND object_id = OBJECT_ID(N'dbo.posts'))
BEGIN
    CREATE INDEX idx_posts_published ON dbo.posts(published);
END;
GO

IF NOT EXISTS (SELECT name FROM sys.indexes WHERE name = N'idx_comments_post_id' AND object_id = OBJECT_ID(N'dbo.comments'))
BEGIN
    CREATE INDEX idx_comments_post_id ON dbo.comments(post_id);
END;
GO

IF NOT EXISTS (SELECT name FROM sys.indexes WHERE name = N'idx_comments_user_id' AND object_id = OBJECT_ID(N'dbo.comments'))
BEGIN
    CREATE INDEX idx_comments_user_id ON dbo.comments(user_id);
END;
GO
