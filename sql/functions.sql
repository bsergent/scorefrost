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

-- Function to get best scores for given levels and scope
-- Returns best scores for each score type across the specified levels
-- If no version is specified for a level (version = -1), gets latest version with scores
CREATE OR REPLACE FUNCTION get_best_scores(
    p_user_id UUID,
    p_levels JSON,
    p_scope VARCHAR(16)
) RETURNS TABLE (
    level_id VARCHAR(16),
    level_version INTEGER,
    score_type VARCHAR(16),
    best_score INTEGER,
    user_id UUID,
    display_name VARCHAR(64),
    friend_code VARCHAR(9)
) AS $$
DECLARE
    v_level_record JSON;
    v_level_id VARCHAR(16);
    v_level_version INTEGER;
    v_sql TEXT;
    v_where_conditions TEXT[];
    v_final_where TEXT;
BEGIN
    -- Build level conditions from JSON array
    FOR v_level_record IN SELECT * FROM json_array_elements(p_levels)
    LOOP
        v_level_id := v_level_record->>'level_id';
        v_level_version := (v_level_record->>'level_version')::INTEGER;
        
        IF v_level_version = -1 THEN
            -- Get latest version for this level
            v_where_conditions := array_append(v_where_conditions, 
                format('(s.level_id = %L AND s.level_version = (
                    SELECT MAX(s2.level_version) 
                    FROM solution s2 
                    JOIN score sc2 ON s2.id = sc2.solution_id 
                    WHERE s2.level_id = %L
                ))', v_level_id, v_level_id));
        ELSE
            -- Specific version
            v_where_conditions := array_append(v_where_conditions, 
                format('(s.level_id = %L AND s.level_version = %s)', v_level_id, v_level_version));
        END IF;
    END LOOP;
    
    IF array_length(v_where_conditions, 1) = 0 THEN
        RAISE EXCEPTION 'No valid level specifications provided';
    END IF;
    
    v_final_where := array_to_string(v_where_conditions, ' OR ');
    
    -- Build query based on scope
    CASE p_scope
        WHEN 'personal' THEN
            v_sql := format('
                SELECT 
                    s.level_id,
                    s.level_version,
                    sc.type_id as score_type,
                    CASE 
                        WHEN st.higher_is_better THEN MAX(sc.score)
                        ELSE MIN(sc.score)
                    END as best_score,
                    u.id as user_id,
                    COALESCE(u.display_name, '''') as display_name,
                    u.friend_code
                FROM solution s
                JOIN score sc ON s.id = sc.solution_id
                JOIN score_type st ON sc.type_id = st.id
                JOIN "user" u ON s.user_id = u.id
                WHERE s.user_id = %L AND (%s)
                GROUP BY s.level_id, s.level_version, sc.type_id, st.higher_is_better, u.id, u.display_name, u.friend_code
                ORDER BY s.level_id, s.level_version, sc.type_id
            ', p_user_id, v_final_where);
            
        WHEN 'friends' THEN
            -- TODO: Implement when user relations are ready
            RAISE EXCEPTION 'Friends scope not yet implemented';
            
        WHEN 'regional' THEN
            -- TODO: Implement when regions are ready
            RAISE EXCEPTION 'Regional scope not yet implemented';
            
        WHEN 'global' THEN
            v_sql := format('
                WITH best_scores_cte AS (
                    SELECT 
                        s.level_id,
                        s.level_version,
                        sc.type_id,
                        CASE 
                            WHEN st.higher_is_better THEN MAX(sc.score)
                            ELSE MIN(sc.score)
                        END as best_score_value
                    FROM solution s
                    JOIN score sc ON s.id = sc.solution_id
                    JOIN score_type st ON sc.type_id = st.id
                    WHERE (%s)
                    GROUP BY s.level_id, s.level_version, sc.type_id, st.higher_is_better
                )
                SELECT DISTINCT
                    s.level_id,
                    s.level_version,
                    sc.type_id as score_type,
                    sc.score as best_score,
                    u.id as user_id,
                    COALESCE(u.display_name, '''') as display_name,
                    u.friend_code
                FROM solution s
                JOIN score sc ON s.id = sc.solution_id
                JOIN score_type st ON sc.type_id = st.id
                JOIN "user" u ON s.user_id = u.id
                JOIN best_scores_cte bsc ON (
                    s.level_id = bsc.level_id 
                    AND s.level_version = bsc.level_version 
                    AND sc.type_id = bsc.type_id 
                    AND sc.score = bsc.best_score_value
                )
                WHERE (%s)
                ORDER BY s.level_id, s.level_version, sc.type_id
            ', v_final_where, v_final_where);
            
        ELSE
            RAISE EXCEPTION 'Invalid scope: %', p_scope;
    END CASE;
    
    -- Execute the dynamic query
    RETURN QUERY EXECUTE v_sql;
END;
$$ LANGUAGE plpgsql;

-- Function to get leaderboard count for pagination
-- Returns total count of scores matching the criteria
CREATE OR REPLACE FUNCTION get_leaderboard_count(
    p_user_id UUID,
    p_levels JSON,
    p_scope VARCHAR(16),
    p_score_type VARCHAR(16)
)
RETURNS INTEGER AS $$
DECLARE
    v_level_json JSON;
    v_where_conditions TEXT[] := '{}';
    v_final_where TEXT;
    v_sql TEXT;
    v_count INTEGER;
BEGIN
    -- Build WHERE conditions for each level specification
    FOR v_level_json IN SELECT * FROM json_array_elements(p_levels)
    LOOP
        IF (v_level_json->>'level_version')::INTEGER = -1 THEN
            -- Latest version: find max version for this level
            v_where_conditions := array_append(v_where_conditions, format(
                '(s.level_id = %L AND s.level_version = (SELECT MAX(level_version) FROM solution WHERE level_id = %L))',
                v_level_json->>'level_id', v_level_json->>'level_id'
            ));
        ELSE
            -- Specific version
            v_where_conditions := array_append(v_where_conditions, format(
                '(s.level_id = %L AND s.level_version = %s)',
                v_level_json->>'level_id', v_level_json->>'level_version'
            ));
        END IF;
    END LOOP;

    -- Combine WHERE conditions
    v_final_where := array_to_string(v_where_conditions, ' OR ');
    
    -- Add score type filter if provided
    IF p_score_type IS NOT NULL AND p_score_type != '' THEN
        v_final_where := v_final_where || format(' AND sc.type_id = %L', p_score_type);
    END IF;

    -- Build query based on scope
    CASE p_scope
        WHEN 'personal' THEN
            v_sql := format('
                SELECT COUNT(DISTINCT (s.level_id, s.level_version, sc.type_id))
                FROM solution s
                JOIN score sc ON s.id = sc.solution_id
                JOIN score_type st ON sc.type_id = st.id
                WHERE s.user_id = %L AND (%s)
            ', p_user_id, v_final_where);
            
        WHEN 'global' THEN
            v_sql := format('
                WITH user_best_scores AS (
                    SELECT 
                        s.user_id,
                        s.level_id,
                        s.level_version,
                        sc.type_id,
                        CASE 
                            WHEN st.higher_is_better THEN MAX(sc.score)
                            ELSE MIN(sc.score)
                        END as best_score_value
                    FROM solution s
                    JOIN score sc ON s.id = sc.solution_id
                    JOIN score_type st ON sc.type_id = st.id
                    WHERE (%s)
                    GROUP BY s.user_id, s.level_id, s.level_version, sc.type_id, st.higher_is_better
                )
                SELECT COUNT(*)
                FROM user_best_scores
            ', v_final_where);
            
        WHEN 'friends' THEN
            RAISE EXCEPTION 'Friends scope not yet implemented';
            
        WHEN 'regional' THEN
            RAISE EXCEPTION 'Regional scope not yet implemented';
            
        ELSE
            RAISE EXCEPTION 'Invalid scope: %', p_scope;
    END CASE;
    
    -- Execute the query and return count
    EXECUTE v_sql INTO v_count;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

-- Function to get paginated leaderboard scores
-- Returns ranked scores with pagination support
CREATE OR REPLACE FUNCTION get_leaderboard(
    p_user_id UUID,
    p_levels JSON,
    p_scope VARCHAR(16),
    p_score_type VARCHAR(16),
    p_offset INTEGER,
    p_size INTEGER
)
RETURNS TABLE(
    rank INTEGER,
    level_id VARCHAR(16),
    level_version INTEGER,
    score_type VARCHAR(16),
    best_score INTEGER,
    user_id UUID,
    display_name VARCHAR(64),
    friend_code VARCHAR(9)
) AS $$
DECLARE
    v_level_json JSON;
    v_where_conditions TEXT[] := '{}';
    v_final_where TEXT;
    v_sql TEXT;
BEGIN
    -- Build WHERE conditions for each level specification
    FOR v_level_json IN SELECT * FROM json_array_elements(p_levels)
    LOOP
        IF (v_level_json->>'level_version')::INTEGER = -1 THEN
            -- Latest version: find max version for this level
            v_where_conditions := array_append(v_where_conditions, format(
                '(s.level_id = %L AND s.level_version = (SELECT MAX(level_version) FROM solution WHERE level_id = %L))',
                v_level_json->>'level_id', v_level_json->>'level_id'
            ));
        ELSE
            -- Specific version
            v_where_conditions := array_append(v_where_conditions, format(
                '(s.level_id = %L AND s.level_version = %s)',
                v_level_json->>'level_id', v_level_json->>'level_version'
            ));
        END IF;
    END LOOP;

    -- Combine WHERE conditions
    v_final_where := array_to_string(v_where_conditions, ' OR ');
    
    -- Add score type filter if provided
    IF p_score_type IS NOT NULL AND p_score_type != '' THEN
        v_final_where := v_final_where || format(' AND sc.type_id = %L', p_score_type);
    END IF;

    -- Build query based on scope
    CASE p_scope
        WHEN 'personal' THEN
            v_sql := format('
                SELECT 
                    ROW_NUMBER() OVER (
                        ORDER BY s.level_id, s.level_version, sc.type_id, 
                        CASE WHEN st.higher_is_better THEN sc.score END DESC,
                        CASE WHEN NOT st.higher_is_better THEN sc.score END ASC,
                        s.date_time_utc ASC
                    )::INTEGER as rank,
                    s.level_id,
                    s.level_version,
                    sc.type_id as score_type,
                    sc.score as best_score,
                    u.id as user_id,
                    COALESCE(u.display_name, '''') as display_name,
                    u.friend_code
                FROM solution s
                JOIN score sc ON s.id = sc.solution_id
                JOIN score_type st ON sc.type_id = st.id
                JOIN "user" u ON s.user_id = u.id
                WHERE s.user_id = %L AND (%s)
                ORDER BY rank
                OFFSET %s LIMIT %s
            ', p_user_id, v_final_where, p_offset, p_size);
            
        WHEN 'global' THEN
            v_sql := format('
                WITH user_best_scores AS (
                    SELECT 
                        s.user_id,
                        s.level_id,
                        s.level_version,
                        sc.type_id,
                        CASE 
                            WHEN st.higher_is_better THEN MAX(sc.score)
                            ELSE MIN(sc.score)
                        END as best_score_value,
                        MIN(s.date_time_utc) as earliest_date -- Tiebreaker for same scores
                    FROM solution s
                    JOIN score sc ON s.id = sc.solution_id
                    JOIN score_type st ON sc.type_id = st.id
                    WHERE (%s)
                    GROUP BY s.user_id, s.level_id, s.level_version, sc.type_id, st.higher_is_better
                ),
                ranked_scores AS (
                    SELECT 
                        ROW_NUMBER() OVER (
                            PARTITION BY ubs.level_id, ubs.level_version, ubs.type_id
                            ORDER BY 
                                CASE WHEN st.higher_is_better THEN ubs.best_score_value END DESC,
                                CASE WHEN NOT st.higher_is_better THEN ubs.best_score_value END ASC,
                                ubs.earliest_date ASC
                        )::INTEGER as level_rank,
                        ROW_NUMBER() OVER (
                            ORDER BY ubs.level_id, ubs.level_version, ubs.type_id,
                                CASE WHEN st.higher_is_better THEN ubs.best_score_value END DESC,
                                CASE WHEN NOT st.higher_is_better THEN ubs.best_score_value END ASC,
                                ubs.earliest_date ASC
                        )::INTEGER as global_rank,
                        ubs.level_id,
                        ubs.level_version,
                        ubs.type_id as score_type,
                        ubs.best_score_value as best_score,
                        ubs.user_id,
                        COALESCE(u.display_name, '''') as display_name,
                        u.friend_code
                    FROM user_best_scores ubs
                    JOIN score_type st ON ubs.type_id = st.id
                    JOIN "user" u ON ubs.user_id = u.id
                )
                SELECT 
                    global_rank as rank,
                    level_id,
                    level_version,
                    score_type,
                    best_score,
                    user_id,
                    display_name,
                    friend_code
                FROM ranked_scores
                ORDER BY global_rank
                OFFSET %s LIMIT %s
            ', v_final_where, p_offset, p_size);
            
        WHEN 'friends' THEN
            RAISE EXCEPTION 'Friends scope not yet implemented';
            
        WHEN 'regional' THEN
            RAISE EXCEPTION 'Regional scope not yet implemented';
            
        ELSE
            RAISE EXCEPTION 'Invalid scope: %', p_scope;
    END CASE;
    
    -- Execute the dynamic query
    RETURN QUERY EXECUTE v_sql;
END;
$$ LANGUAGE plpgsql;

-- Function to calculate total play time for a user by summing all their time_ms scores
-- Returns the total play time in milliseconds
CREATE OR REPLACE FUNCTION get_user_play_time_ms(p_user_id UUID)
RETURNS INTEGER AS $$
DECLARE
    v_total_time INTEGER := 0;
BEGIN
    -- Sum all time_ms scores for the user
    SELECT COALESCE(SUM(sc.score), 0) INTO v_total_time
    FROM solution s
    JOIN score sc ON s.id = sc.solution_id
    WHERE s.user_id = p_user_id 
      AND sc.type_id = 'time_ms';
    
    RETURN v_total_time;
END;
$$ LANGUAGE plpgsql;

-- Function to update user's last active time and game version
-- Called during login to track user activity
CREATE OR REPLACE FUNCTION touch_user_active_time(
    p_user_id UUID,
    p_game_version VARCHAR(32)
) RETURNS VOID AS $$
BEGIN
    UPDATE "user"
    SET date_time_active_utc = NOW() AT TIME ZONE 'UTC',
        game_version = p_game_version
    WHERE id = p_user_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'User not found: %', p_user_id;
    END IF;
END;
$$ LANGUAGE plpgsql;
