package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetCheckpoint returns the checkpoint for a plan week, or ErrNotFound when that week has none
// (only W12 and B7 do).
func (s *Store) GetCheckpoint(ctx context.Context, week string) (Checkpoint, error) {
	var (
		c         Checkpoint
		questions string
		helpers   *string
		answers   *string
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT id, week, questions, helpers, answers, completed_at FROM checkpoints WHERE week = ?`, week,
	).Scan(&c.ID, &c.Week, &questions, &helpers, &answers, &c.CompletedAt)
	if err != nil {
		return Checkpoint{}, classify(fmt.Sprintf("get checkpoint %s", week), err)
	}

	if err := json.Unmarshal([]byte(questions), &c.Questions); err != nil {
		return Checkpoint{}, fmt.Errorf("decode checkpoint %s questions: %w", week, err)
	}

	if helpers != nil {
		if err := json.Unmarshal([]byte(*helpers), &c.Helpers); err != nil {
			return Checkpoint{}, fmt.Errorf("decode checkpoint %s helpers: %w", week, err)
		}
		if len(c.Helpers) != len(c.Questions) {
			return Checkpoint{}, fmt.Errorf(
				"decode checkpoint %s: got %d helpers for %d questions: %w",
				week, len(c.Helpers), len(c.Questions), ErrConstraint,
			)
		}
	}

	if answers != nil {
		if err := json.Unmarshal([]byte(*answers), &c.Answers); err != nil {
			return Checkpoint{}, fmt.Errorf("decode checkpoint %s answers: %w", week, err)
		}
		if len(c.Answers) != len(c.Questions) {
			return Checkpoint{}, fmt.Errorf(
				"decode checkpoint %s: got %d answers for %d questions: %w",
				week, len(c.Answers), len(c.Questions), ErrConstraint,
			)
		}
	}

	return c, nil
}

// SaveCheckpointAnswers replaces the answer array whole.
//
// answers must be the same length as questions - a mismatch is a caller bug, so it is rejected
// here as well as in the handler. The first save stamps completed_at; later saves leave it.
func (s *Store) SaveCheckpointAnswers(ctx context.Context, week string, answers []string) (Checkpoint, error) {
	existing, err := s.GetCheckpoint(ctx, week)
	if err != nil {
		return Checkpoint{}, err
	}

	if len(answers) != len(existing.Questions) {
		return Checkpoint{}, fmt.Errorf(
			"save checkpoint %s: got %d answers for %d questions: %w",
			week, len(answers), len(existing.Questions), ErrConstraint,
		)
	}

	encoded, err := json.Marshal(answers)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("encode checkpoint %s answers: %w", week, err)
	}

	completedAt := existing.CompletedAt
	if completedAt == nil {
		now := s.utcNow()
		completedAt = &now
	}

	if _, err := s.db.ExecContext(ctx,
		`UPDATE checkpoints SET answers = ?, completed_at = ? WHERE week = ?`,
		string(encoded), completedAt, week,
	); err != nil {
		return Checkpoint{}, classify(fmt.Sprintf("save checkpoint %s", week), err)
	}

	return s.GetCheckpoint(ctx, week)
}
