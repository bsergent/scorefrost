-- User Table
-- Stores user information including display names and API key authentication
CREATE TABLE IF NOT EXISTS "user" (
    id UUID PRIMARY KEY,
    friend_code VARCHAR(9) UNIQUE NOT NULL, -- Format: XXXX-XXXX (8 alphanumeric chars + 1 dash)
    display_name_pending VARCHAR(32),
    display_name VARCHAR(32),
    display_name_status SMALLINT NOT NULL DEFAULT 0, -- 0=Pending, 1=Approved, 2=Rejected
    api_key_hash VARCHAR(64) NOT NULL,
    date_time_created_utc TIMESTAMP NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    date_time_active_utc TIMESTAMP NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    game_version VARCHAR(32),
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