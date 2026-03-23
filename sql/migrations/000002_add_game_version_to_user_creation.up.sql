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

-- Migrate solution.id primary key from SERIAL/INTEGER to UUIDv7 and update score FK.

-- Ensure extension objects live in a dedicated schema.
CREATE SCHEMA IF NOT EXISTS ext;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pgcrypto') THEN
        ALTER EXTENSION pgcrypto SET SCHEMA ext;
    ELSE
        CREATE EXTENSION pgcrypto WITH SCHEMA ext;
    END IF;
END;
$$;

-- UUIDv7 generator compatible with PostgreSQL 15.
CREATE OR REPLACE FUNCTION generate_uuid_v7()
RETURNS UUID AS $$
DECLARE
    v_unix_ts_ms BIGINT;
    v_random BYTEA;
    v_uuid_bytes BYTEA;
BEGIN
    -- Unix timestamp in milliseconds (48 most-significant bits in UUIDv7).
    v_unix_ts_ms := FLOOR(EXTRACT(EPOCH FROM clock_timestamp()) * 1000);
    v_random := ext.gen_random_bytes(10);
    v_uuid_bytes := E'\\x00000000000000000000000000000000'::BYTEA;

    -- Timestamp bytes (big-endian, bytes 0..5).
    v_uuid_bytes := set_byte(v_uuid_bytes, 0, ((v_unix_ts_ms >> 40) & 255)::INT);
    v_uuid_bytes := set_byte(v_uuid_bytes, 1, ((v_unix_ts_ms >> 32) & 255)::INT);
    v_uuid_bytes := set_byte(v_uuid_bytes, 2, ((v_unix_ts_ms >> 24) & 255)::INT);
    v_uuid_bytes := set_byte(v_uuid_bytes, 3, ((v_unix_ts_ms >> 16) & 255)::INT);
    v_uuid_bytes := set_byte(v_uuid_bytes, 4, ((v_unix_ts_ms >> 8) & 255)::INT);
    v_uuid_bytes := set_byte(v_uuid_bytes, 5, (v_unix_ts_ms & 255)::INT);

    -- Version 7 in high nibble of byte 6.
    v_uuid_bytes := set_byte(v_uuid_bytes, 6, ((7 << 4) | (get_byte(v_random, 0) & 15))::INT);
    v_uuid_bytes := set_byte(v_uuid_bytes, 7, get_byte(v_random, 1));

    -- RFC 4122 variant (10xx xxxx) in byte 8.
    v_uuid_bytes := set_byte(v_uuid_bytes, 8, ((2 << 6) | (get_byte(v_random, 2) & 63))::INT);

    -- Remaining random bytes.
    v_uuid_bytes := set_byte(v_uuid_bytes, 9, get_byte(v_random, 3));
    v_uuid_bytes := set_byte(v_uuid_bytes, 10, get_byte(v_random, 4));
    v_uuid_bytes := set_byte(v_uuid_bytes, 11, get_byte(v_random, 5));
    v_uuid_bytes := set_byte(v_uuid_bytes, 12, get_byte(v_random, 6));
    v_uuid_bytes := set_byte(v_uuid_bytes, 13, get_byte(v_random, 7));
    v_uuid_bytes := set_byte(v_uuid_bytes, 14, get_byte(v_random, 8));
    v_uuid_bytes := set_byte(v_uuid_bytes, 15, get_byte(v_random, 9));

    RETURN encode(v_uuid_bytes, 'hex')::UUID;
END;
$$ LANGUAGE plpgsql VOLATILE;

-- Drop dependent functions before replacing solution.id column.
DROP FUNCTION IF EXISTS get_best_scores(UUID, JSON, VARCHAR(16));
DROP FUNCTION IF EXISTS get_leaderboard_count(UUID, JSON, VARCHAR(16), VARCHAR(16));
DROP FUNCTION IF EXISTS get_leaderboard(UUID, JSON, VARCHAR(16), VARCHAR(16), INTEGER, INTEGER);
DROP FUNCTION IF EXISTS get_user_play_time_ms(UUID);

-- Add replacement UUID columns and backfill existing data.
ALTER TABLE solution ADD COLUMN id_uuid UUID;
ALTER TABLE solution ALTER COLUMN id_uuid SET DEFAULT generate_uuid_v7();
UPDATE solution
SET id_uuid = generate_uuid_v7()
WHERE id_uuid IS NULL;
ALTER TABLE solution ALTER COLUMN id_uuid SET NOT NULL;

ALTER TABLE score ADD COLUMN solution_id_uuid UUID;
UPDATE score sc
SET solution_id_uuid = s.id_uuid
FROM solution s
WHERE sc.solution_id = s.id;
ALTER TABLE score ALTER COLUMN solution_id_uuid SET NOT NULL;

-- Swap primary and foreign keys.
ALTER TABLE score DROP CONSTRAINT IF EXISTS fk_score_solution;
ALTER TABLE score DROP CONSTRAINT IF EXISTS score_pkey;
ALTER TABLE solution DROP CONSTRAINT IF EXISTS solution_pkey;
ALTER TABLE solution ALTER COLUMN id DROP DEFAULT;
ALTER SEQUENCE IF EXISTS solution_id_seq OWNED BY NONE;

ALTER TABLE solution DROP COLUMN id;
ALTER TABLE solution RENAME COLUMN id_uuid TO id;
ALTER TABLE solution ADD CONSTRAINT solution_pkey PRIMARY KEY (id);

ALTER TABLE score DROP COLUMN solution_id;
ALTER TABLE score RENAME COLUMN solution_id_uuid TO solution_id;
ALTER TABLE score ADD CONSTRAINT score_pkey PRIMARY KEY (solution_id, type_id);
ALTER TABLE score
    ADD CONSTRAINT fk_score_solution
    FOREIGN KEY (solution_id) REFERENCES solution(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_score_solution_id ON score(solution_id);

-- Remove legacy serial sequence if present.
DROP SEQUENCE IF EXISTS solution_id_seq;

-- Update stored procedure return type and internals for UUID solution IDs.
DROP FUNCTION IF EXISTS submit_solution_with_scores(UUID, VARCHAR(16), INTEGER, VARCHAR(32), VARCHAR(1024), SMALLINT, JSON);

CREATE OR REPLACE FUNCTION submit_solution_with_scores(
    p_user_id UUID,
    p_level_id VARCHAR(16),
    p_level_version INTEGER,
    p_game_version VARCHAR(32),
    p_solution VARCHAR(1024),
    p_result SMALLINT,
    p_scores JSON
) RETURNS UUID AS $$
DECLARE
    v_solution_id UUID;
    v_score_record JSON;
    v_score_type VARCHAR(16);
    v_score_value INTEGER;
    v_score_type_exists BOOLEAN;
BEGIN
    INSERT INTO solution (user_id, level_id, level_version, game_version, solution, result)
    VALUES (p_user_id, p_level_id, p_level_version, p_game_version, p_solution, p_result)
    RETURNING id INTO v_solution_id;

    FOR v_score_record IN SELECT * FROM json_array_elements(p_scores)
    LOOP
        v_score_type := v_score_record->>'type';
        v_score_value := (v_score_record->>'value')::INTEGER;

        SELECT EXISTS(SELECT 1 FROM score_type WHERE id = v_score_type) INTO v_score_type_exists;

        IF NOT v_score_type_exists THEN
            RAISE EXCEPTION 'Invalid score type: %', v_score_type;
        END IF;

        INSERT INTO score (solution_id, type_id, score)
        VALUES (v_solution_id, v_score_type, v_score_value);
    END LOOP;

    RETURN v_solution_id;
END;
$$ LANGUAGE plpgsql;

-- Recreate function to get best scores.
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
    FOR v_level_record IN SELECT * FROM json_array_elements(p_levels)
    LOOP
        v_level_id := v_level_record->>'level_id';
        v_level_version := (v_level_record->>'level_version')::INTEGER;

        IF v_level_version = -1 THEN
            v_where_conditions := array_append(v_where_conditions,
                format('(s.level_id = %L AND s.level_version = (
                    SELECT MAX(s2.level_version)
                    FROM solution s2
                    JOIN score sc2 ON s2.id = sc2.solution_id
                    WHERE s2.level_id = %L
                ))', v_level_id, v_level_id));
        ELSE
            v_where_conditions := array_append(v_where_conditions,
                format('(s.level_id = %L AND s.level_version = %s)', v_level_id, v_level_version));
        END IF;
    END LOOP;

    IF array_length(v_where_conditions, 1) = 0 THEN
        RAISE EXCEPTION 'No valid level specifications provided';
    END IF;

    v_final_where := array_to_string(v_where_conditions, ' OR ');

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
            RAISE EXCEPTION 'Friends scope not yet implemented';

        WHEN 'regional' THEN
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

    RETURN QUERY EXECUTE v_sql;
END;
$$ LANGUAGE plpgsql;

-- Recreate function to get leaderboard count.
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
    FOR v_level_json IN SELECT * FROM json_array_elements(p_levels)
    LOOP
        IF (v_level_json->>'level_version')::INTEGER = -1 THEN
            v_where_conditions := array_append(v_where_conditions, format(
                '(s.level_id = %L AND s.level_version = (SELECT MAX(level_version) FROM solution WHERE level_id = %L))',
                v_level_json->>'level_id', v_level_json->>'level_id'
            ));
        ELSE
            v_where_conditions := array_append(v_where_conditions, format(
                '(s.level_id = %L AND s.level_version = %s)',
                v_level_json->>'level_id', v_level_json->>'level_version'
            ));
        END IF;
    END LOOP;

    v_final_where := array_to_string(v_where_conditions, ' OR ');

    IF p_score_type IS NOT NULL AND p_score_type != '' THEN
        v_final_where := v_final_where || format(' AND sc.type_id = %L', p_score_type);
    END IF;

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

    EXECUTE v_sql INTO v_count;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

-- Recreate function to get paginated leaderboard.
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
    FOR v_level_json IN SELECT * FROM json_array_elements(p_levels)
    LOOP
        IF (v_level_json->>'level_version')::INTEGER = -1 THEN
            v_where_conditions := array_append(v_where_conditions, format(
                '(s.level_id = %L AND s.level_version = (SELECT MAX(level_version) FROM solution WHERE level_id = %L))',
                v_level_json->>'level_id', v_level_json->>'level_id'
            ));
        ELSE
            v_where_conditions := array_append(v_where_conditions, format(
                '(s.level_id = %L AND s.level_version = %s)',
                v_level_json->>'level_id', v_level_json->>'level_version'
            ));
        END IF;
    END LOOP;

    v_final_where := array_to_string(v_where_conditions, ' OR ');

    IF p_score_type IS NOT NULL AND p_score_type != '' THEN
        v_final_where := v_final_where || format(' AND sc.type_id = %L', p_score_type);
    END IF;

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
                        MIN(s.date_time_utc) as earliest_date
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

    RETURN QUERY EXECUTE v_sql;
END;
$$ LANGUAGE plpgsql;

-- Recreate function to calculate play time.
CREATE OR REPLACE FUNCTION get_user_play_time_ms(p_user_id UUID)
RETURNS INTEGER AS $$
DECLARE
    v_total_time INTEGER := 0;
BEGIN
    SELECT COALESCE(SUM(sc.score), 0) INTO v_total_time
    FROM solution s
    JOIN score sc ON s.id = sc.solution_id
    WHERE s.user_id = p_user_id
      AND sc.type_id = 'time_ms';

    RETURN v_total_time;
END;
$$ LANGUAGE plpgsql;
