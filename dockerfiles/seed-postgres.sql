-- PostgreSQL test data seed script

INSERT INTO users (id, name, email, age, status) VALUES
(1, 'Alice Johnson', 'alice@example.com', 28, 'active'),
(2, 'Bob Smith', 'bob@example.com', 34, 'active'),
(3, 'Charlie Davis', 'charlie@example.com', 45, 'inactive'),
(4, 'Diana Wilson', 'diana@example.com', 29, 'active'),
(5, 'Eve Martinez', 'eve@example.com', 31, 'active');

INSERT INTO posts (id, user_id, title, content, published) VALUES
(1, 1, 'First Post', 'This is the content of the first post', TRUE),
(2, 1, 'Second Post', 'Content of the second post about databases', TRUE),
(3, 2, 'Bob''s Post', 'Bob discussing SQL and databases', FALSE),
(4, 3, 'Inactive Post', 'This user is inactive now', TRUE),
(5, 4, 'Diana''s Great Post', 'Diana sharing insights on programming', TRUE);

INSERT INTO comments (id, post_id, user_id, content) VALUES
(1, 1, 2, 'Great post Alice!'),
(2, 1, 4, 'Very informative, thanks!'),
(3, 2, 3, 'I disagree with this approach'),
(4, 2, 5, 'Excellent explanation'),
(5, 3, 1, 'Bob makes good points'),
(6, 5, 2, 'Diana is always insightful'),
(7, 5, 3, 'Love the perspective here');

-- Reset sequences to match MySQL identity values
SELECT setval(pg_get_serial_sequence('users', 'id'), 6);
SELECT setval(pg_get_serial_sequence('posts', 'id'), 6);
SELECT setval(pg_get_serial_sequence('comments', 'id'), 8);
