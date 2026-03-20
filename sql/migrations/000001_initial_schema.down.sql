-- Rollback for initial schema migration.
-- WARNING: This drops all ScoreFrost tables and functions.

-- Drop functions first
DROP FUNCTION IF EXISTS touch_user_active_time(UUID, VARCHAR(32));
DROP FUNCTION IF EXISTS get_user_play_time_ms(UUID);
DROP FUNCTION IF EXISTS get_leaderboard(UUID, JSON, VARCHAR(16), VARCHAR(16), INTEGER, INTEGER);
DROP FUNCTION IF EXISTS get_leaderboard_count(UUID, JSON, VARCHAR(16), VARCHAR(16));
DROP FUNCTION IF EXISTS get_best_scores(UUID, JSON, VARCHAR(16));
DROP FUNCTION IF EXISTS submit_solution_with_scores(UUID, VARCHAR(16), INTEGER, VARCHAR(32), VARCHAR(1024), SMALLINT, JSON);
DROP FUNCTION IF EXISTS reject_display_name(UUID);
DROP FUNCTION IF EXISTS approve_display_name(UUID);
DROP FUNCTION IF EXISTS get_pending_display_names();
DROP FUNCTION IF EXISTS create_user(UUID, VARCHAR(9), VARCHAR(64), VARCHAR(64));

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS user_relation CASCADE;
DROP TABLE IF EXISTS score CASCADE;
DROP TABLE IF EXISTS solution CASCADE;
DROP TABLE IF EXISTS score_type CASCADE;
DROP TABLE IF EXISTS "user" CASCADE;
