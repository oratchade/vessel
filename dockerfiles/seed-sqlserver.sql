-- SQL Server test data seed script

USE test_db;
GO

SET IDENTITY_INSERT dbo.users ON;
INSERT INTO dbo.users (id, name, email, age, status) VALUES
(1, 'Alice Johnson', 'alice@example.com', 28, 'active'),
(2, 'Bob Smith', 'bob@example.com', 34, 'active'),
(3, 'Charlie Davis', 'charlie@example.com', 45, 'inactive'),
(4, 'Diana Wilson', 'diana@example.com', 29, 'active'),
(5, 'Eve Martinez', 'eve@example.com', 31, 'active');
SET IDENTITY_INSERT dbo.users OFF;
GO

SET IDENTITY_INSERT dbo.posts ON;
INSERT INTO dbo.posts (id, user_id, title, content, published) VALUES
(1, 1, 'First Post', 'This is the content of the first post', 1),
(2, 1, 'Second Post', 'Content of the second post about databases', 1),
(3, 2, 'Bob''s Post', 'Bob discussing SQL and databases', 0),
(4, 3, 'Inactive Post', 'This user is inactive now', 1),
(5, 4, 'Diana''s Great Post', 'Diana sharing insights on programming', 1);
SET IDENTITY_INSERT dbo.posts OFF;
GO

SET IDENTITY_INSERT dbo.comments ON;
INSERT INTO dbo.comments (id, post_id, user_id, content) VALUES
(1, 1, 2, 'Great post Alice!'),
(2, 1, 4, 'Very informative, thanks!'),
(3, 2, 3, 'I disagree with this approach'),
(4, 2, 5, 'Excellent explanation'),
(5, 3, 1, 'Bob makes good points'),
(6, 5, 2, 'Diana is always insightful'),
(7, 5, 3, 'Love the perspective here');
SET IDENTITY_INSERT dbo.comments OFF;
GO

DBCC CHECKIDENT ('dbo.users', RESEED, 5);
DBCC CHECKIDENT ('dbo.posts', RESEED, 5);
DBCC CHECKIDENT ('dbo.comments', RESEED, 7);
GO

SELECT * FROM dbo.users;
GO
