-- Rollback for game_version in user creation
-- Revert create_user function to previous signature without game_version parameter

-- Revert create_user signature.
DROP FUNCTION IF EXISTS create_user(UUID, VARCHAR(9), VARCHAR(64), VARCHAR(64), VARCHAR(32));

CREATE OR REPLACE FUNCTION create_user(
    p_id UUID,
    p_friend_code VARCHAR(9),
    p_display_name VARCHAR(64),
    p_api_key_hash VARCHAR(64)
) RETURNS UUID AS $$
BEGIN
    INSERT INTO "user" (id, friend_code, display_name, display_name_status, api_key_hash)
    VALUES (p_id, p_friend_code, p_display_name, 1, p_api_key_hash);

    RETURN p_id;
END;
$$ LANGUAGE plpgsql;
