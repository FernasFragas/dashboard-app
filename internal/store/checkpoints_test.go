package store

import (
	"context"
	"errors"
	"testing"
)

func insertCheckpoint(t *testing.T, s *Store, week string) {
	t.Helper()

	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO checkpoints (week, questions, answers, completed_at)
		VALUES (?, ?, NULL, NULL)`,
		week, `["eval number?","chaos falsified anything?","PR merged or stale?"]`)
	if err != nil {
		t.Fatalf("insert checkpoint fixture: %v", err)
	}
}

func TestGetCheckpoint(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	insertCheckpoint(t, s, "W12")

	c, err := s.GetCheckpoint(ctx, "W12")
	if err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}

	if len(c.Questions) != 3 {
		t.Errorf("questions = %d, want 3", len(c.Questions))
	}
	if c.Answers != nil {
		t.Errorf("answers = %v, want nil before the first save", c.Answers)
	}
	if c.CompletedAt != nil {
		t.Errorf("completed_at = %v, want nil", *c.CompletedAt)
	}
}

func TestGetCheckpointMissing(t *testing.T) {
	_, err := newTestStore(t).GetCheckpoint(context.Background(), "W5")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound: only W12 and B7 have checkpoints", err)
	}
}

// The first save stamps completed_at; later saves replace the answers and leave it.
func TestSaveCheckpointAnswers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	insertCheckpoint(t, s, "W12")

	saved, err := s.SaveCheckpointAnswers(ctx, "W12", []string{"0.31", "yes", "merged"})
	if err != nil {
		t.Fatalf("save answers: %v", err)
	}

	if saved.CompletedAt == nil {
		t.Fatal("completed_at is nil after the first save")
	}
	firstStamp := *saved.CompletedAt

	if len(saved.Answers) != 3 || saved.Answers[0] != "0.31" {
		t.Errorf("answers = %v, want the saved array", saved.Answers)
	}

	// A blank answer is still an answer.
	resaved, err := s.SaveCheckpointAnswers(ctx, "W12", []string{"0.29", "", "stale"})
	if err != nil {
		t.Fatalf("resave answers: %v", err)
	}

	if resaved.CompletedAt == nil || *resaved.CompletedAt != firstStamp {
		t.Errorf("completed_at = %v, want it unchanged at %q", resaved.CompletedAt, firstStamp)
	}
	if resaved.Answers[1] != "" {
		t.Errorf("answers[1] = %q, want the empty answer preserved", resaved.Answers[1])
	}
}

// A length mismatch is a caller bug, rejected here as well as in the handler.
func TestSaveCheckpointAnswersLengthMismatch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	insertCheckpoint(t, s, "W12")

	if _, err := s.SaveCheckpointAnswers(ctx, "W12", []string{"only one"}); !errors.Is(err, ErrConstraint) {
		t.Errorf("error = %v, want ErrConstraint", err)
	}
}

func TestSaveCheckpointAnswersMissingWeek(t *testing.T) {
	_, err := newTestStore(t).SaveCheckpointAnswers(context.Background(), "W5", []string{"x"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// A checkpoint must reference a real plan week.
func TestCheckpointWeekIsAForeignKey(t *testing.T) {
	s := newTestStore(t)

	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO checkpoints (week, questions) VALUES ('W99', '[]')`)
	if !errors.Is(classify("insert checkpoint", err), ErrConstraint) {
		t.Errorf("error = %v, want a foreign-key violation", err)
	}
}
