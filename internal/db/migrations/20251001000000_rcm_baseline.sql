-- +goose Up
-- +goose StatementBegin
-- RCM Baseline Schema
-- This is the foundational schema for Retrospective Context Management (RCM)
--
-- Turn Structure: turn = user_side + (assistant_tool_calls*) + assistant_side
--
-- Key Concepts:
-- - turn_number: Identifies a complete user-assistant exchange (atomic unit)
-- - turn_sequence: Order within turn (0=user, 1+=assistant actions)
-- - turn_part_type: Type of message (user_message, tool_request, tool_result, assistant_message)
-- - hydration_state: Whether turn is visible in context (hydrated) or stored (dehydrated)

-- This migration assumes ai_conversations table already exists in the database
-- (shared schema with token-saver-mcp)

-- Verify ai_conversations exists
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 'ai_conversations'
    ) THEN
        RAISE EXCEPTION 'ai_conversations table does not exist. Please ensure rcm_context schema is applied first.';
    END IF;
END $$;

-- Create trigger function for conversation stats
CREATE OR REPLACE FUNCTION update_conversation_stats() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    UPDATE ai_conversations
    SET
        total_turns = total_turns + 1,
        total_tokens_input = total_tokens_input + COALESCE(NEW.tokens_input, 0),
        total_tokens_output = total_tokens_output + COALESCE(NEW.tokens_output, 0),
        last_active = NEW.timestamp,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.conversation_id;
    RETURN NEW;
END;
$$;

-- Create ai_conversation_turns with RCM atomic turn structure
CREATE TABLE IF NOT EXISTS ai_conversation_turns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,

    -- Turn structure fields (grouped for clarity)
    turn_number INTEGER NOT NULL,
    turn_sequence INTEGER NOT NULL DEFAULT 0,
    turn_part_type VARCHAR(20) DEFAULT 'user_message',
    hydration_state VARCHAR(20) DEFAULT 'hydrated',

    -- Message metadata
    agent VARCHAR(50),
    role VARCHAR(50) NOT NULL,

    -- Content
    content TEXT NOT NULL,
    parts JSONB,

    -- Token tracking
    tokens_input INTEGER,
    tokens_output INTEGER,
    tokens_total INTEGER,

    -- Model info
    model VARCHAR(100),
    provider VARCHAR(100),

    -- Performance
    duration_ms INTEGER,
    temperature DECIMAL(3,2),

    -- Timestamps
    timestamp TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP WITH TIME ZONE,

    -- Additional data
    metadata JSONB
);

-- Add comments explaining the RCM turn structure
COMMENT ON COLUMN ai_conversation_turns.turn_number IS 'Turn identifier - all parts with same turn_number form one complete turn';
COMMENT ON COLUMN ai_conversation_turns.turn_sequence IS 'Sequence within turn (0=user_side, 1+=assistant actions, N=assistant_side)';
COMMENT ON COLUMN ai_conversation_turns.turn_part_type IS 'Type of turn part: user_message, tool_request, tool_result, assistant_message';
COMMENT ON COLUMN ai_conversation_turns.hydration_state IS 'Turn state: hydrated (visible in context) or dehydrated (stored but not in context)';

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_turn_structure ON ai_conversation_turns (conversation_id, turn_number, turn_sequence);
CREATE INDEX IF NOT EXISTS idx_hydration ON ai_conversation_turns (conversation_id, hydration_state);
CREATE INDEX IF NOT EXISTS idx_conversation_turns ON ai_conversation_turns (conversation_id);

-- Create trigger to update conversation stats
CREATE TRIGGER update_conversation_on_turn
AFTER INSERT ON ai_conversation_turns
FOR EACH ROW
EXECUTE FUNCTION update_conversation_stats();

-- Create files table for AI edit tracking
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(path, conversation_id, version)
);

CREATE INDEX IF NOT EXISTS idx_files_conversation_id ON files (conversation_id);
CREATE INDEX IF NOT EXISTS idx_files_path ON files (path);

-- Create trigger function for file updates
CREATE OR REPLACE FUNCTION update_files_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_files_updated_at
BEFORE UPDATE ON files
FOR EACH ROW
EXECUTE FUNCTION update_files_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_conversation_on_turn ON ai_conversation_turns;
DROP TRIGGER IF EXISTS update_files_updated_at ON files;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS ai_conversation_turns;
DROP FUNCTION IF EXISTS update_conversation_stats();
DROP FUNCTION IF EXISTS update_files_updated_at();
-- Note: We don't drop ai_conversations as it's shared with token-saver-mcp
-- +goose StatementEnd
