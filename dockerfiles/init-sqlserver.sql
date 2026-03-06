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

USE test_db;
GO

-- Create users table
IF OBJECT_ID(N'dbo.users', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.users (
        id INT IDENTITY(1,1) PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        email VARCHAR(255) NOT NULL UNIQUE,
        age INT,
        status VARCHAR(50) DEFAULT 'active',
        created_at DATETIME DEFAULT GETDATE(),
        updated_at DATETIME DEFAULT GETDATE()
    );
END;
GO

-- Create posts table
IF OBJECT_ID(N'dbo.posts', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.posts (
        id INT IDENTITY(1,1) PRIMARY KEY,
        user_id INT NOT NULL REFERENCES dbo.users(id) ON DELETE CASCADE,
        title VARCHAR(255) NOT NULL,
        content TEXT,
        published BIT DEFAULT 0,
        created_at DATETIME DEFAULT GETDATE(),
        updated_at DATETIME DEFAULT GETDATE()
    );
END;
GO

-- Create comments table
IF OBJECT_ID(N'dbo.comments', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.comments (
        id INT IDENTITY(1,1) PRIMARY KEY,
        post_id INT NOT NULL REFERENCES dbo.posts(id) ON DELETE CASCADE,
        user_id INT NOT NULL REFERENCES dbo.users(id) ON DELETE CASCADE,
        content TEXT NOT NULL,
        created_at DATETIME DEFAULT GETDATE()
    );
END;
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
