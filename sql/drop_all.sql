-- =====================================================
-- DROP ALL TABLES - FOR DEVELOPMENT/TESTING ONLY
-- =====================================================
-- WARNING: This will delete ALL data permanently!
-- Use this script only in development environments.
-- =====================================================

-- Drop tables in reverse dependency order to avoid foreign key constraint violations

-- Drop user_relation table (depends on user)
DROP TABLE IF EXISTS user_relation CASCADE;

-- Drop score table (depends on solution and score_type)
DROP TABLE IF EXISTS score CASCADE;

-- Drop solution table (depends on user)
DROP TABLE IF EXISTS solution CASCADE;

-- Drop score_type table (no dependencies)
DROP TABLE IF EXISTS score_type CASCADE;

-- Drop user table (other tables depend on this)
DROP TABLE IF EXISTS "user" CASCADE;

-- Drop functions
DROP FUNCTION IF EXISTS create_user(UUID, VARCHAR(9), VARCHAR(64), VARCHAR(64));

-- Note: After running this script, run tables.sql to recreate the tables
