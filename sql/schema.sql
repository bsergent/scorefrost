-- User Table
-- Stores user information including display names and API key authentication
CREATE TABLE IF NOT EXISTS "user" (
    id UUID PRIMARY KEY,
    friend_code VARCHAR(9) UNIQUE NOT NULL, -- Format: XXXX-XXXX (8 alphanumeric chars + 1 dash)
    display_name_pending VARCHAR(64),
    display_name VARCHAR(64),
    display_name_status SMALLINT NOT NULL DEFAULT 0, -- 0=Pending, 1=Approved, 2=Rejected
    api_key_hash VARCHAR(64) NOT NULL,
    date_time_created_utc TIMESTAMP NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    CONSTRAINT chk_display_name_status CHECK (display_name_status IN (0, 1, 2))
);

CREATE INDEX IF NOT EXISTS idx_user_display_name ON "user"(display_name);
CREATE INDEX IF NOT EXISTS idx_user_api_key_hash ON "user"(api_key_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_friend_code ON "user"(friend_code);

-- Score Type Table
-- Defines types of scores (e.g., time, striping, fuel, stars)
CREATE TABLE IF NOT EXISTS score_type (
    id VARCHAR(16) PRIMARY KEY,
    display_name VARCHAR(64) NOT NULL,
    higher_is_better BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_score_type_display_name ON score_type(display_name);

-- Solution Table
-- Stores player solutions for levels
CREATE TABLE IF NOT EXISTS solution (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    level_id VARCHAR(16) NOT NULL,
    level_version INTEGER NOT NULL,
    game_version VARCHAR(32) NOT NULL,
    solution VARCHAR(1024) NOT NULL,
    result SMALLINT NOT NULL, -- 0=Failure, 1=Success, 2=DidNotFinish
    date_time_utc TIMESTAMP NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    CONSTRAINT fk_solution_user FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE,
    CONSTRAINT chk_result CHECK (result IN (0, 1, 2))
);

CREATE INDEX IF NOT EXISTS idx_solution_user_id ON solution(user_id);
CREATE INDEX IF NOT EXISTS idx_solution_level_id ON solution(level_id);
CREATE INDEX IF NOT EXISTS idx_solution_level_version ON solution(level_id, level_version);
CREATE INDEX IF NOT EXISTS idx_solution_game_version ON solution(game_version);
CREATE INDEX IF NOT EXISTS idx_solution_date_time ON solution(date_time_utc);

-- Score Table
-- Individual scores for each solution by type
CREATE TABLE IF NOT EXISTS score (
    solution_id INTEGER NOT NULL,
    type_id VARCHAR(16) NOT NULL,
    score INTEGER NOT NULL,
    PRIMARY KEY (solution_id, type_id),
    CONSTRAINT fk_score_solution FOREIGN KEY (solution_id) REFERENCES solution(id) ON DELETE CASCADE,
    CONSTRAINT fk_score_type FOREIGN KEY (type_id) REFERENCES score_type(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_score_solution_id ON score(solution_id);
CREATE INDEX IF NOT EXISTS idx_score_type_id ON score(type_id);
CREATE INDEX IF NOT EXISTS idx_score_value ON score(score);

-- User Relation Table
-- Links users together as friends or blocks
CREATE TABLE IF NOT EXISTS user_relation (
    user_id_source UUID NOT NULL,
    user_id_target UUID NOT NULL,
    status SMALLINT NOT NULL DEFAULT 0, -- 0=Pending, 1=Confirmed, 2=Blocked
    PRIMARY KEY (user_id_source, user_id_target),
    CONSTRAINT fk_user_relation_source FOREIGN KEY (user_id_source) REFERENCES "user"(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_relation_target FOREIGN KEY (user_id_target) REFERENCES "user"(id) ON DELETE CASCADE,
    CONSTRAINT chk_status CHECK (status IN (0, 1, 2)),
    CONSTRAINT chk_no_self_relation CHECK (user_id_source != user_id_target)
);

CREATE INDEX IF NOT EXISTS idx_user_relation_source ON user_relation(user_id_source);
CREATE INDEX IF NOT EXISTS idx_user_relation_target ON user_relation(user_id_target);
CREATE INDEX IF NOT EXISTS idx_user_relation_status ON user_relation(status);

-- Insert anonymous users (Anonymous and Dev)
-- Note: Dev user's API key hash is set by the application on startup
INSERT INTO "user" (id, friend_code, display_name, display_name_status, api_key_hash)
VALUES 
    ('00000000-0000-0000-0000-000000000000', '0000-0000', 'Anonymous', 1, 'anonymous_no_key'),
    ('00000000-0000-0000-0000-000000000001', '0000-0001', 'Dev', 1, 'placeholder')
ON CONFLICT (id) DO NOTHING;

-- Insert basic score types
INSERT INTO score_type (id, display_name, higher_is_better)
VALUES 
    ('time_ms', 'Time (Milliseconds)', FALSE), -- Lower time is better
    ('striping', 'Striping', TRUE),            -- Higher striping is better
    ('fuel', 'Fuel', FALSE),                   -- Lower fuel usage is better
    ('stars', 'Stars', TRUE)                   -- Higher stars is better
ON CONFLICT (id) DO NOTHING;

-- Function to create a new user
-- Returns the new user's ID
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

-- Function to get all pending display names
-- Returns table with user info and pending names
CREATE OR REPLACE FUNCTION get_pending_display_names()
RETURNS TABLE (
    user_id UUID,
    friend_code VARCHAR(9),
    current_display_name VARCHAR(64),
    pending_display_name VARCHAR(64),
    display_name_status SMALLINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        id,
        u.friend_code,
        COALESCE(u.display_name, '')::VARCHAR(64) as current_display_name,
        COALESCE(u.display_name_pending, '')::VARCHAR(64) as pending_display_name,
        u.display_name_status
    FROM "user" u
    WHERE u.display_name_status = 0
      AND u.display_name_pending IS NOT NULL
      AND u.display_name_pending != ''
    ORDER BY u.date_time_created_utc ASC;
END;
$$ LANGUAGE plpgsql;

-- Function to approve a pending display name
-- Returns the final display name and status
CREATE OR REPLACE FUNCTION approve_display_name(p_user_id UUID)
RETURNS TABLE (
    final_display_name VARCHAR(64),
    status SMALLINT
) AS $$
DECLARE
    v_pending_name VARCHAR(64);
BEGIN
    -- Check if user has a pending display name
    SELECT display_name_pending INTO v_pending_name
    FROM "user"
    WHERE id = p_user_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'User not found';
    END IF;
    
    IF v_pending_name IS NULL OR TRIM(v_pending_name) = '' THEN
        RAISE EXCEPTION 'No pending display name for this user';
    END IF;
    
    -- Approve: Set display_name to pending value, clear pending, set status to approved (1)
    UPDATE "user"
    SET display_name = display_name_pending,
        display_name_pending = NULL,
        display_name_status = 1
    WHERE id = p_user_id;
    
    -- Return the approved display name and status
    RETURN QUERY
    SELECT v_pending_name, 1::SMALLINT;
END;
$$ LANGUAGE plpgsql;

-- Function to reject a pending display name
-- Returns the current display name and status
CREATE OR REPLACE FUNCTION reject_display_name(p_user_id UUID)
RETURNS TABLE (
    final_display_name VARCHAR(64),
    status SMALLINT
) AS $$
DECLARE
    v_current_name VARCHAR(64);
    v_pending_name VARCHAR(64);
BEGIN
    -- Check if user has a pending display name
    SELECT display_name, display_name_pending INTO v_current_name, v_pending_name
    FROM "user"
    WHERE id = p_user_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'User not found';
    END IF;
    
    IF v_pending_name IS NULL OR TRIM(v_pending_name) = '' THEN
        RAISE EXCEPTION 'No pending display name for this user';
    END IF;
    
    -- Reject: Clear pending, set status to rejected (2), keep current display_name
    UPDATE "user"
    SET display_name_status = 2
    WHERE id = p_user_id;
    
    -- Return the current display name and status
    RETURN QUERY
    SELECT COALESCE(v_current_name, ''), 2::SMALLINT;
END;
$$ LANGUAGE plpgsql;

-- Function to submit a solution with scores
-- Takes solution data and array of score data, returns solution_id
-- Uses implicit transaction for atomic insertion of solution and all scores
-- Validates all score types before insertion to ensure data integrity
CREATE OR REPLACE FUNCTION submit_solution_with_scores(
    p_user_id UUID,
    p_level_id VARCHAR(16),
    p_level_version INTEGER,
    p_game_version VARCHAR(32),
    p_solution VARCHAR(1024),
    p_result SMALLINT,
    p_scores JSON
) RETURNS INTEGER AS $$
DECLARE
    v_solution_id INTEGER;
    v_score_record JSON;
    v_score_type VARCHAR(16);
    v_score_value INTEGER;
    v_score_type_exists BOOLEAN;
BEGIN
    -- Insert solution and get the ID
    INSERT INTO solution (user_id, level_id, level_version, game_version, solution, result)
    VALUES (p_user_id, p_level_id, p_level_version, p_game_version, p_solution, p_result)
    RETURNING id INTO v_solution_id;
    
    -- Process each score in the JSON array
    FOR v_score_record IN SELECT * FROM json_array_elements(p_scores)
    LOOP
        -- Extract score type and value from JSON
        v_score_type := v_score_record->>'type';
        v_score_value := (v_score_record->>'value')::INTEGER;
        
        -- Validate that score type exists
        SELECT EXISTS(SELECT 1 FROM score_type WHERE id = v_score_type) INTO v_score_type_exists;
        
        IF NOT v_score_type_exists THEN
            RAISE EXCEPTION 'Invalid score type: %', v_score_type;
        END IF;
        
        -- Insert the score
        INSERT INTO score (solution_id, type_id, score)
        VALUES (v_solution_id, v_score_type, v_score_value);
    END LOOP;
    
    -- Return the solution ID
    RETURN v_solution_id;
END;
$$ LANGUAGE plpgsql;
