package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// PostgresQueries implements the Querier interface for PostgreSQL
type PostgresQueries struct {
	db DBTX
}

// NewPostgres creates a new PostgreSQL implementation of Querier
func NewPostgres(db DBTX) *PostgresQueries {
	return &PostgresQueries{db: db}
}

// Ensure PostgresQueries implements Querier
var _ Querier = (*PostgresQueries)(nil)

// WithTx returns a new PostgresQueries instance that uses the provided transaction
func (q *PostgresQueries) WithTx(tx *sql.Tx) *PostgresQueries {
	return &PostgresQueries{db: tx}
}

// CreateSession creates a new conversation session
func (q *PostgresQueries) CreateSession(ctx context.Context, arg CreateSessionParams) (Session, error) {
	// Debug logging removed - working correctly now
	// Use default model/provider if not provided
	modelProvider := arg.ModelProvider
	if modelProvider == "" {
		modelProvider = "anthropic"
	}
	modelID := arg.ModelID
	if modelID == "" {
		modelID = "claude-sonnet-4-5-20250929"
	}

	query := `
		INSERT INTO ai_conversations (
			session_id,
			parent_conversation_id,
			title,
			model_provider,
			model_id,
			start_time,
			last_active,
			status,
			total_turns,
			total_tokens_input,
			total_tokens_output,
			cost_usd
		) VALUES (
			$1, $2, $3, $4, $5,
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP,
			'active',
			$6, $7, $8, $9
		) RETURNING
			session_id, parent_conversation_id, title,
			total_turns, total_tokens_input, total_tokens_output,
			cost_usd, created_at, updated_at, summary_turn_id
	`

	var sess Session
	var createdAt, updatedAt sql.NullTime
	var parentID, summaryID sql.NullString
	var costUSD sql.NullFloat64

	err := q.db.QueryRowContext(ctx, query,
		arg.ID,
		arg.ParentSessionID,
		arg.Title,
		modelProvider,
		modelID,
		arg.MessageCount,
		arg.PromptTokens,
		arg.CompletionTokens,
		arg.Cost,
	).Scan(
		&sess.ID,
		&parentID,
		&sess.Title,
		&sess.MessageCount,
		&sess.PromptTokens,
		&sess.CompletionTokens,
		&costUSD,
		&createdAt,
		&updatedAt,
		&summaryID,
	)

	if err != nil {
		return Session{}, err
	}

	sess.ParentSessionID = parentID
	sess.SummaryMessageID = summaryID
	sess.Cost = costUSD.Float64

	if createdAt.Valid {
		sess.CreatedAt = createdAt.Time.Unix()
		sess.UpdatedAt = sess.CreatedAt
	}

	return sess, nil
}

// GetSessionByID retrieves a session by ID
func (q *PostgresQueries) GetSessionByID(ctx context.Context, id string) (Session, error) {
	query := `
		SELECT
			session_id, parent_conversation_id, title,
			total_turns, total_tokens_input, total_tokens_output,
			cost_usd, created_at, updated_at, summary_turn_id
		FROM ai_conversations
		WHERE session_id = $1
		LIMIT 1
	`

	var sess Session
	var createdAt, updatedAt sql.NullTime
	var parentID, summaryID sql.NullString
	var costUSD sql.NullFloat64

	err := q.db.QueryRowContext(ctx, query, id).Scan(
		&sess.ID,
		&parentID,
		&sess.Title,
		&sess.MessageCount,
		&sess.PromptTokens,
		&sess.CompletionTokens,
		&costUSD,
		&createdAt,
		&updatedAt,
		&summaryID,
	)

	if err != nil {
		return Session{}, err
	}

	sess.ParentSessionID = parentID
	sess.SummaryMessageID = summaryID
	sess.Cost = costUSD.Float64

	if createdAt.Valid {
		sess.CreatedAt = createdAt.Time.Unix()
		sess.UpdatedAt = updatedAt.Time.Unix()
	}

	return sess, nil
}

// ListSessions lists all root sessions (no parent)
func (q *PostgresQueries) ListSessions(ctx context.Context) ([]Session, error) {
	query := `
		SELECT
			session_id, parent_conversation_id, title,
			total_turns, total_tokens_input, total_tokens_output,
			cost_usd, created_at, updated_at, summary_turn_id
		FROM ai_conversations
		WHERE parent_conversation_id IS NULL
		ORDER BY created_at DESC
	`

	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var createdAt, updatedAt sql.NullTime
		var parentID, summaryID sql.NullString
		var costUSD sql.NullFloat64

		err := rows.Scan(
			&sess.ID,
			&parentID,
			&sess.Title,
			&sess.MessageCount,
			&sess.PromptTokens,
			&sess.CompletionTokens,
			&costUSD,
			&createdAt,
			&updatedAt,
			&summaryID,
		)
		if err != nil {
			return nil, err
		}

		sess.ParentSessionID = parentID
		sess.SummaryMessageID = summaryID
		sess.Cost = costUSD.Float64

		if createdAt.Valid {
			sess.CreatedAt = createdAt.Time.Unix()
			sess.UpdatedAt = updatedAt.Time.Unix()
		}

		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// UpdateSession updates a session
func (q *PostgresQueries) UpdateSession(ctx context.Context, arg UpdateSessionParams) (Session, error) {
	query := `
		UPDATE ai_conversations
		SET
			title = $1,
			total_tokens_input = $2,
			total_tokens_output = $3,
			summary_turn_id = $4,
			cost_usd = $5,
			last_active = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE session_id = $6
		RETURNING
			session_id, parent_conversation_id, title,
			total_turns, total_tokens_input, total_tokens_output,
			cost_usd, created_at, updated_at, summary_turn_id
	`

	var sess Session
	var createdAt, updatedAt sql.NullTime
	var parentID, summaryID sql.NullString
	var costUSD sql.NullFloat64

	err := q.db.QueryRowContext(ctx, query,
		arg.Title,
		arg.PromptTokens,
		arg.CompletionTokens,
		arg.SummaryMessageID,
		arg.Cost,
		arg.ID,
	).Scan(
		&sess.ID,
		&parentID,
		&sess.Title,
		&sess.MessageCount,
		&sess.PromptTokens,
		&sess.CompletionTokens,
		&costUSD,
		&createdAt,
		&updatedAt,
		&summaryID,
	)

	if err != nil {
		return Session{}, err
	}

	sess.ParentSessionID = parentID
	sess.SummaryMessageID = summaryID
	sess.Cost = costUSD.Float64

	if createdAt.Valid {
		sess.CreatedAt = createdAt.Time.Unix()
		sess.UpdatedAt = updatedAt.Time.Unix()
	}

	return sess, nil
}

// DeleteSession deletes a session
func (q *PostgresQueries) DeleteSession(ctx context.Context, id string) error {
	query := `DELETE FROM ai_conversations WHERE session_id = $1`
	_, err := q.db.ExecContext(ctx, query, id)
	return err
}

// CreateMessage creates a new message/turn
func (q *PostgresQueries) CreateMessage(ctx context.Context, arg CreateMessageParams) (Message, error) {
	// First, ensure the conversation exists
	var convID uuid.UUID
	err := q.db.QueryRowContext(ctx,
		`SELECT id FROM ai_conversations WHERE session_id = $1`,
		arg.SessionID,
	).Scan(&convID)

	if err == sql.ErrNoRows {
		// Create session if it doesn't exist
		createSess := CreateSessionParams{
			ID:               arg.SessionID,
			Title:            "New Conversation",
			MessageCount:     0,
			PromptTokens:     0,
			CompletionTokens: 0,
			Cost:             0,
		}
		_, err = q.CreateSession(ctx, createSess)
		if err != nil {
			return Message{}, fmt.Errorf("failed to create session: %w", err)
		}
		// Get the conversation ID again
		err = q.db.QueryRowContext(ctx,
			`SELECT id FROM ai_conversations WHERE session_id = $1`,
			arg.SessionID,
		).Scan(&convID)
		if err != nil {
			return Message{}, fmt.Errorf("failed to get conversation ID: %w", err)
		}
	} else if err != nil {
		return Message{}, err
	}

	// Create message ID if not provided
	msgID := arg.ID
	if msgID == "" {
		msgID = uuid.New().String()
	}

	// Prepare parts as JSON
	var partsJSON []byte
	if arg.Parts != "" {
		// Check if it's already JSON
		if json.Valid([]byte(arg.Parts)) {
			partsJSON = []byte(arg.Parts)
		} else {
			// Create a simple text part
			parts := []map[string]string{
				{"type": "text", "content": arg.Parts},
			}
			partsJSON, _ = json.Marshal(parts)
		}
	} else {
		partsJSON = []byte("[]")
	}

	// Extract content for the content field
	content := arg.Parts
	var partsList []map[string]interface{}
	if json.Unmarshal(partsJSON, &partsList) == nil && len(partsList) > 0 {
		if textContent, ok := partsList[0]["content"].(string); ok {
			content = textContent
		}
	}

	// Determine turn_number and turn_sequence based on turn structure:
	// turn = user_side + (assistant_tool_calls*) + assistant_side
	var turnNumber int
	var turnSequence int
	var turnPartType string

	if arg.Role == "user" {
		// User message starts a new turn (user_side)
		err = q.db.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(turn_number), 0) + 1 FROM ai_conversation_turns WHERE id_conversation = $1`,
			convID,
		).Scan(&turnNumber)
		if err != nil {
			return Message{}, fmt.Errorf("failed to get next turn number: %w", err)
		}
		turnSequence = 0
		turnPartType = "user_message"
	} else {
		// Assistant/tool message continues the current turn
		err = q.db.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(turn_number), 1), COALESCE(MAX(turn_sequence), -1) + 1
			 FROM ai_conversation_turns WHERE id_conversation = $1`,
			convID,
		).Scan(&turnNumber, &turnSequence)
		if err != nil {
			return Message{}, fmt.Errorf("failed to get current turn info: %w", err)
		}

		// Determine turn_part_type based on role and content
		if arg.Role == "tool" {
			turnPartType = "tool_result"
		} else if arg.Role == "assistant" {
			// Check if this contains tool calls
			partsStr := string(arg.Parts)
			if len(partsStr) > 20 && strings.Contains(partsStr, `"type":"tool_use"`) {
				turnPartType = "tool_request"
			} else {
				// Assistant's final text response (assistant_side)
				turnPartType = "assistant_message"
			}
		} else {
			turnPartType = "user_message"
		}
	}

	query := `
		INSERT INTO ai_conversation_turns (
			id_conversation,
			turn_number,
			turn_sequence,
			turn_part_type,
			agent,
			role,
			content,
			parts,
			model,
			provider,
			timestamp,
			created_at,
			hydration_state
		) VALUES (
			$1, $2, $3, $4,
			'crush-rcm',
			$5,
			$6,
			$7,
			$8,
			$9,
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP,
			'hydrated'
		) RETURNING
			id, role, parts, model, provider,
			timestamp, created_at, finished_at
	`

	var msg Message
	var msgUUID uuid.UUID
	var timestamp, createdAt sql.NullTime
	var finishedAt sql.NullTime
	var returnedParts json.RawMessage

	err = q.db.QueryRowContext(ctx, query,
		convID,
		turnNumber,
		turnSequence,
		turnPartType,
		arg.Role,
		content,
		partsJSON,
		arg.Model,
		arg.Provider,
	).Scan(
		&msgUUID,
		&msg.Role,
		&returnedParts,
		&msg.Model,
		&msg.Provider,
		&timestamp,
		&createdAt,
		&finishedAt,
	)

	if err != nil {
		return Message{}, fmt.Errorf("failed to create message: %w", err)
	}

	msg.ID = msgUUID.String()
	msg.SessionID = arg.SessionID
	msg.Parts = string(returnedParts)

	if createdAt.Valid {
		msg.CreatedAt = createdAt.Time.Unix()
		msg.UpdatedAt = msg.CreatedAt
	}

	if finishedAt.Valid {
		msg.FinishedAt = sql.NullInt64{
			Int64: finishedAt.Time.Unix(),
			Valid: true,
		}
	}

	// Update session token counts if provided
	if arg.Model.Valid || arg.Provider.Valid {
		updateQuery := `
			UPDATE ai_conversations
			SET last_active = CURRENT_TIMESTAMP,
			    total_turns = total_turns + 1
			WHERE id = $1
		`
		q.db.ExecContext(ctx, updateQuery, convID)
	}

	return msg, nil
}

// GetMessage retrieves a message by ID
func (q *PostgresQueries) GetMessage(ctx context.Context, id string) (Message, error) {
	query := `
		SELECT
			t.id, c.session_id, t.role, t.parts, t.model, t.provider,
			t.created_at, t.finished_at
		FROM ai_conversation_turns t
		JOIN ai_conversations c ON t.id_conversation = c.id
		WHERE t.id = $1
		LIMIT 1
	`

	var msg Message
	var msgUUID uuid.UUID
	var createdAt sql.NullTime
	var finishedAt sql.NullTime
	var parts json.RawMessage

	err := q.db.QueryRowContext(ctx, query, id).Scan(
		&msgUUID,
		&msg.SessionID,
		&msg.Role,
		&parts,
		&msg.Model,
		&msg.Provider,
		&createdAt,
		&finishedAt,
	)

	if err != nil {
		return Message{}, err
	}

	msg.ID = msgUUID.String()
	msg.Parts = string(parts)

	if createdAt.Valid {
		msg.CreatedAt = createdAt.Time.Unix()
		msg.UpdatedAt = msg.CreatedAt
	}

	if finishedAt.Valid {
		msg.FinishedAt = sql.NullInt64{
			Int64: finishedAt.Time.Unix(),
			Valid: true,
		}
	}

	return msg, nil
}

// ListMessagesBySession lists all messages for a session
func (q *PostgresQueries) ListMessagesBySession(ctx context.Context, sessionID string) ([]Message, error) {
	query := `
		SELECT
			t.id, t.role, t.parts, t.model, t.provider,
			t.created_at, t.finished_at
		FROM ai_conversation_turns t
		JOIN ai_conversations c ON t.id_conversation = c.id
		WHERE c.session_id = $1
		ORDER BY t.turn_number ASC
	`

	rows, err := q.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var msgUUID uuid.UUID
		var createdAt sql.NullTime
		var finishedAt sql.NullTime
		var parts json.RawMessage

		err := rows.Scan(
			&msgUUID,
			&msg.Role,
			&parts,
			&msg.Model,
			&msg.Provider,
			&createdAt,
			&finishedAt,
		)
		if err != nil {
			return nil, err
		}

		msg.ID = msgUUID.String()
		msg.SessionID = sessionID
		msg.Parts = string(parts)

		if createdAt.Valid {
			msg.CreatedAt = createdAt.Time.Unix()
			msg.UpdatedAt = msg.CreatedAt
		}

		if finishedAt.Valid {
			msg.FinishedAt = sql.NullInt64{
				Int64: finishedAt.Time.Unix(),
				Valid: true,
			}
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// UpdateMessage updates a message
func (q *PostgresQueries) UpdateMessage(ctx context.Context, arg UpdateMessageParams) error {
	// Parse parts to get content
	content := ""
	if arg.Parts != "" {
		var partsList []map[string]interface{}
		if json.Unmarshal([]byte(arg.Parts), &partsList) == nil && len(partsList) > 0 {
			if textContent, ok := partsList[0]["content"].(string); ok {
				content = textContent
			}
		}
	}

	query := `
		UPDATE ai_conversation_turns
		SET
			content = $1,
			parts = $2,
			finished_at = $3
		WHERE id = $4
	`

	_, err := q.db.ExecContext(ctx, query,
		content,
		arg.Parts,
		time.Unix(arg.FinishedAt.Int64, 0),
		arg.ID,
	)
	return err
}

// DeleteMessage deletes a message
func (q *PostgresQueries) DeleteMessage(ctx context.Context, id string) error {
	query := `DELETE FROM ai_conversation_turns WHERE id = $1`
	_, err := q.db.ExecContext(ctx, query, id)
	return err
}

// DeleteSessionMessages deletes all messages for a session
func (q *PostgresQueries) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	query := `
		DELETE FROM ai_conversation_turns t
		USING ai_conversations c
		WHERE t.id_conversation = c.id
		AND c.session_id = $1
	`
	_, err := q.db.ExecContext(ctx, query, sessionID)
	return err
}

// File operations - these are simple stubs for now
func (q *PostgresQueries) CreateFile(ctx context.Context, arg CreateFileParams) (File, error) {
	// Files need id_conversation, not session_id
	var convID uuid.UUID
	err := q.db.QueryRowContext(ctx,
		`SELECT id FROM ai_conversations WHERE session_id = $1`,
		arg.SessionID,
	).Scan(&convID)

	if err != nil {
		return File{}, err
	}

	query := `
		INSERT INTO files (
			id_conversation,
			path,
			content,
			version
		) VALUES ($1, $2, $3, $4)
		RETURNING id, path, content, version, created_at, updated_at
	`

	var file File
	var fileUUID uuid.UUID
	var createdAt, updatedAt time.Time

	err = q.db.QueryRowContext(ctx, query,
		convID,
		arg.Path,
		arg.Content,
		arg.Version,
	).Scan(
		&fileUUID,
		&file.Path,
		&file.Content,
		&file.Version,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return File{}, err
	}

	file.ID = fileUUID.String()
	file.SessionID = arg.SessionID
	file.CreatedAt = createdAt.Unix()
	file.UpdatedAt = updatedAt.Unix()

	return file, nil
}

func (q *PostgresQueries) GetFile(ctx context.Context, id string) (File, error) {
	query := `
		SELECT f.id, c.session_id, f.path, f.content, f.version, f.created_at, f.updated_at
		FROM files f
		JOIN ai_conversations c ON f.id_conversation = c.id
		WHERE f.id = $1
	`

	var file File
	var fileUUID uuid.UUID
	var createdAt, updatedAt time.Time

	err := q.db.QueryRowContext(ctx, query, id).Scan(
		&fileUUID,
		&file.SessionID,
		&file.Path,
		&file.Content,
		&file.Version,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return File{}, err
	}

	file.ID = fileUUID.String()
	file.CreatedAt = createdAt.Unix()
	file.UpdatedAt = updatedAt.Unix()

	return file, nil
}

func (q *PostgresQueries) GetFileByPathAndSession(ctx context.Context, arg GetFileByPathAndSessionParams) (File, error) {
	query := `
		SELECT f.id, c.session_id, f.path, f.content, f.version, f.created_at, f.updated_at
		FROM files f
		JOIN ai_conversations c ON f.id_conversation = c.id
		WHERE f.path = $1 AND c.session_id = $2
		ORDER BY f.version DESC, f.created_at DESC
		LIMIT 1
	`

	var file File
	var fileUUID uuid.UUID
	var createdAt, updatedAt time.Time

	err := q.db.QueryRowContext(ctx, query, arg.Path, arg.SessionID).Scan(
		&fileUUID,
		&file.SessionID,
		&file.Path,
		&file.Content,
		&file.Version,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return File{}, err
	}

	file.ID = fileUUID.String()
	file.CreatedAt = createdAt.Unix()
	file.UpdatedAt = updatedAt.Unix()

	return file, nil
}

func (q *PostgresQueries) ListFilesByPath(ctx context.Context, path string) ([]File, error) {
	query := `
		SELECT f.id, c.session_id, f.path, f.content, f.version, f.created_at, f.updated_at
		FROM files f
		JOIN ai_conversations c ON f.id_conversation = c.id
		WHERE f.path = $1
		ORDER BY f.version DESC, f.created_at DESC
	`

	rows, err := q.db.QueryContext(ctx, query, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		var fileUUID uuid.UUID
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&fileUUID,
			&file.SessionID,
			&file.Path,
			&file.Content,
			&file.Version,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		file.ID = fileUUID.String()
		file.CreatedAt = createdAt.Unix()
		file.UpdatedAt = updatedAt.Unix()

		files = append(files, file)
	}

	return files, nil
}

func (q *PostgresQueries) ListFilesBySession(ctx context.Context, sessionID string) ([]File, error) {
	query := `
		SELECT f.id, f.path, f.content, f.version, f.created_at, f.updated_at
		FROM files f
		JOIN ai_conversations c ON f.id_conversation = c.id
		WHERE c.session_id = $1
		ORDER BY f.version ASC, f.created_at ASC
	`

	rows, err := q.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		var fileUUID uuid.UUID
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&fileUUID,
			&file.Path,
			&file.Content,
			&file.Version,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		file.ID = fileUUID.String()
		file.SessionID = sessionID
		file.CreatedAt = createdAt.Unix()
		file.UpdatedAt = updatedAt.Unix()

		files = append(files, file)
	}

	return files, nil
}

func (q *PostgresQueries) ListLatestSessionFiles(ctx context.Context, sessionID string) ([]File, error) {
	query := `
		SELECT DISTINCT ON (f.path)
			f.id, f.path, f.content, f.version, f.created_at, f.updated_at
		FROM files f
		JOIN ai_conversations c ON f.id_conversation = c.id
		WHERE c.session_id = $1
		ORDER BY f.path, f.version DESC, f.created_at DESC
	`

	rows, err := q.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		var fileUUID uuid.UUID
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&fileUUID,
			&file.Path,
			&file.Content,
			&file.Version,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		file.ID = fileUUID.String()
		file.SessionID = sessionID
		file.CreatedAt = createdAt.Unix()
		file.UpdatedAt = updatedAt.Unix()

		files = append(files, file)
	}

	return files, nil
}

func (q *PostgresQueries) ListNewFiles(ctx context.Context) ([]File, error) {
	// Return empty list for now
	return []File{}, nil
}

func (q *PostgresQueries) DeleteFile(ctx context.Context, id string) error {
	query := `DELETE FROM files WHERE id = $1`
	_, err := q.db.ExecContext(ctx, query, id)
	return err
}

func (q *PostgresQueries) DeleteSessionFiles(ctx context.Context, sessionID string) error {
	query := `
		DELETE FROM files f
		USING ai_conversations c
		WHERE f.id_conversation = c.id
		AND c.session_id = $1
	`
	_, err := q.db.ExecContext(ctx, query, sessionID)
	return err
}