-- Migration to include game_version in user creation via create_user stored procedure
-- This allows game version to be captured at the time of user creation

-- Drop old create_user function signature
DROP FUNCTION IF EXISTS create_user(UUID, VARCHAR(9), VARCHAR(64), VARCHAR(64));

-- Create new create_user function with game_version parameter
CREATE OR REPLACE FUNCTION create_user(
    p_id UUID,
    p_friend_code VARCHAR(9),
    p_display_name VARCHAR(64),
    p_api_key_hash VARCHAR(64),
    p_game_version VARCHAR(32)
) RETURNS UUID AS $$
BEGIN
    INSERT INTO "user" (id, friend_code, display_name, display_name_status, api_key_hash, game_version)
    VALUES (p_id, p_friend_code, p_display_name, 1, p_api_key_hash, p_game_version);

    RETURN p_id;
END;
$$ LANGUAGE plpgsql;
