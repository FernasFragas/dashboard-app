package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FernasFragas/dashboard-app/migrations"
	"modernc.org/sqlite"
)

// insertSkill adds a second skill beyond the fixture, so tests can tell them apart.
func insertSkill(t *testing.T, s *Store, id int64, code string) int64 {
	t.Helper()

	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO skills (id, code, name, description, associate_when, target_tier,
			sort_order, created_at)
		VALUES (?, ?, ?, 'd', 'w', 'Expert', ?, '2026-08-24T09:00:00Z')`,
		id, code, code, id)
	if err != nil {
		t.Fatalf("insert skill %s: %v", code, err)
	}

	return id
}

// Progress is derived, never stored (ADR-007): finishing a task must move the skill's numbers
// with no second write anywhere.
func TestListSkillsDerivesProgress(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	evals := insertSkill(t, s, 2, "evals")

	first, err := s.CreateTask(ctx, NewTask{
		Week: "W1", Title: "golden set", Project: "synapse", SkillIDs: []int64{evals},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if _, err := s.CreateTask(ctx, NewTask{
		Week: "W1", Title: "eval runner", Project: "synapse", SkillIDs: []int64{evals},
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	before := skillByCode(t, s, "evals")
	if before.TaskCount != 2 || before.TaskDone != 0 {
		t.Errorf("done/total = %d/%d, want 0/2", before.TaskDone, before.TaskCount)
	}
	if before.LastActivity != nil {
		t.Errorf("last_activity = %v, want nil before anything is done", *before.LastActivity)
	}

	if _, err := s.UpdateTask(ctx, first.ID, first.Version, TaskPatch{Status: ptr("done")}); err != nil {
		t.Fatalf("toggle done: %v", err)
	}

	after := skillByCode(t, s, "evals")
	if after.TaskDone != 1 || after.TaskCount != 2 {
		t.Errorf("done/total = %d/%d, want 1/2", after.TaskDone, after.TaskCount)
	}
	if after.LastActivity == nil {
		t.Error("last_activity is nil after a linked task was completed")
	}
}

func TestListSkillsCountsEvidence(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	writing := insertSkill(t, s, 3, "writing")

	if _, err := s.CreateLogEntry(ctx, NewLogEntry{
		CategoryID: "application", Title: "wrote something", SkillIDs: []int64{writing},
	}); err != nil {
		t.Fatalf("create log entry: %v", err)
	}

	skill := skillByCode(t, s, "writing")
	if skill.EvidenceCount != 1 {
		t.Errorf("evidence_count = %d, want 1", skill.EvidenceCount)
	}
	if skill.LastActivity == nil {
		t.Error("last_activity is nil after a log cited the skill")
	}
}

// A skill with nothing linked reports zeroes rather than being absent.
func TestListSkillsIncludesEmptySkills(t *testing.T) {
	skills, err := newTestStore(t).ListSkills(context.Background())
	if err != nil {
		t.Fatalf("list skills: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("skills = %d, want the 1 fixture", len(skills))
	}
	if skills[0].TaskCount != 0 || skills[0].EvidenceCount != 0 {
		t.Errorf("empty skill = %+v, want zeroes", skills[0])
	}
}

func TestListSkillsQueryCountDoesNotScaleWithSkillCount(t *testing.T) {
	ctx := context.Background()
	s, counter := newCountingTestStore(t)

	for i := int64(2); i <= 6; i++ {
		insertSkill(t, s, i, fmt.Sprintf("skill-%02d", i))
	}

	counter.Reset()
	if _, err := s.ListSkills(ctx); err != nil {
		t.Fatalf("list skills with 6 skills: %v", err)
	}
	first := counter.Count()

	for i := int64(7); i <= 26; i++ {
		insertSkill(t, s, i, fmt.Sprintf("skill-%02d", i))
	}

	counter.Reset()
	if _, err := s.ListSkills(ctx); err != nil {
		t.Fatalf("list skills with 26 skills: %v", err)
	}
	second := counter.Count()

	if first != 3 || second != 3 {
		t.Fatalf("queries = %d then %d, want fixed 3-query profile", first, second)
	}
}

func TestGetSkillDetail(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	serving := insertSkill(t, s, 4, "serving")

	if _, err := s.CreateTask(ctx, NewTask{
		Week: "W8", Title: "batching", Project: "gateway", SkillIDs: []int64{serving},
	}); err == nil {
		t.Fatal("expected an unknown-week rejection so the fixture stays honest")
	}

	if _, err := s.CreateTask(ctx, NewTask{
		Week: "W5", Title: "batching", Project: "gateway", SkillIDs: []int64{serving},
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	if _, err := s.CreateLogEntry(ctx, NewLogEntry{
		CategoryID: "number", Title: "bake-off numbers", SkillIDs: []int64{serving},
	}); err != nil {
		t.Fatalf("create log entry: %v", err)
	}

	detail, err := s.GetSkillDetail(ctx, serving)
	if err != nil {
		t.Fatalf("get skill detail: %v", err)
	}

	if detail.Code != "serving" {
		t.Errorf("code = %q, want serving", detail.Code)
	}
	if len(detail.Tasks) != 1 {
		t.Errorf("tasks = %d, want 1", len(detail.Tasks))
	}
	if len(detail.Evidence) != 1 {
		t.Errorf("evidence = %d, want 1", len(detail.Evidence))
	}

	if _, err := s.GetSkillDetail(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown skill error = %v, want ErrNotFound", err)
	}
}

// The links are a statement about rows, not history: deleting the task takes them with it,
// unlike the SET NULL used for goals.
func TestDeletingATaskCascadesItsSkillLinks(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	task := createTask(t, s, "W1", "temporary", "dash")

	if got := countRows(t, s, "task_skills"); got != 1 {
		t.Fatalf("task_skills rows = %d, want 1", got)
	}

	if err := s.DeleteTask(ctx, task.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}

	if got := countRows(t, s, "task_skills"); got != 0 {
		t.Errorf("task_skills rows = %d after deleting the task, want 0", got)
	}
}

func TestSetTaskSkillsRejectsAnEmptyList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	task := createTask(t, s, "W1", "needs a skill", "dash")

	if err := s.SetTaskSkills(ctx, task.ID, nil); !errors.Is(err, ErrConstraint) {
		t.Errorf("error = %v, want ErrConstraint", err)
	}

	// The existing link survives a rejected write.
	if got := countRows(t, s, "task_skills"); got != 1 {
		t.Errorf("task_skills rows = %d, want the original link intact", got)
	}
}

// Replacing links is wholesale, and duplicates in the input collapse.
func TestSetTaskSkillsReplaces(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	other := insertSkill(t, s, 5, "chaos")
	task := createTask(t, s, "W1", "moves between skills", "dash")

	if err := s.SetTaskSkills(ctx, task.ID, []int64{other, other}); err != nil {
		t.Fatalf("set skills: %v", err)
	}

	after, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	if len(after.SkillIDs) != 1 || after.SkillIDs[0] != other {
		t.Errorf("skill_ids = %v, want exactly [%d]", after.SkillIDs, other)
	}
}

// Tasks that predate M8 have no link at all; the UI needs to know which ones.
func TestUnlinkedTaskIDs(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createTask(t, s, "W1", "linked", "dash")

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO tasks (week, title, project, steps, status, sort_order, version, created_at)
		VALUES ('W1', 'a task from before M8', 'dash', '[]', 'todo', 9999, 1, '2026-08-24T09:00:00Z')`,
	); err != nil {
		t.Fatalf("insert legacy task: %v", err)
	}

	unlinked, err := s.UnlinkedTaskIDs(ctx)
	if err != nil {
		t.Fatalf("unlinked tasks: %v", err)
	}

	if len(unlinked) != 1 {
		t.Errorf("unlinked = %v, want exactly the legacy task", unlinked)
	}
}

func skillByCode(t *testing.T, s *Store, code string) Skill {
	t.Helper()

	skills, err := s.ListSkills(context.Background())
	if err != nil {
		t.Fatalf("list skills: %v", err)
	}

	for _, skill := range skills {
		if skill.Code == code {
			return skill
		}
	}

	t.Fatalf("skill %q not found", code)

	return Skill{}
}

type queryCounter struct {
	count atomic.Int64
}

func (c *queryCounter) Reset() {
	c.count.Store(0)
}

func (c *queryCounter) Count() int64 {
	return c.count.Load()
}

func newCountingTestStore(t *testing.T) (*Store, *queryCounter) {
	t.Helper()

	counter := &queryCounter{}
	path := filepath.Join(t.TempDir(), "counted.db")
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"+
			"&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		path,
	)

	base, err := sqlite.NewConnector(dsn)
	if err != nil {
		t.Fatalf("new sqlite connector: %v", err)
	}

	db := sql.OpenDB(countingConnector{Connector: base, counter: counter})
	db.SetMaxOpenConns(1)

	s := &Store{db: db, now: func() time.Time { return fixedNow }}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close counted store: %v", err)
		}
	})

	if err := s.verifyPragmas(); err != nil {
		t.Fatalf("verify pragmas: %v", err)
	}
	if err := s.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	insertFixtures(t, s)

	return s, counter
}

type countingConnector struct {
	driver.Connector
	counter *queryCounter
}

func (c countingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return countingConn{Conn: conn, counter: c.counter}, nil
}

type countingConn struct {
	driver.Conn
	counter *queryCounter
}

func (c countingConn) ExecContext(
	ctx context.Context, query string, args []driver.NamedValue,
) (driver.Result, error) {
	c.counter.count.Add(1)
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return execer.ExecContext(ctx, query, args)
}

func (c countingConn) QueryContext(
	ctx context.Context, query string, args []driver.NamedValue,
) (driver.Rows, error) {
	c.counter.count.Add(1)
	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return queryer.QueryContext(ctx, query, args)
}

func (c countingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	preparer, ok := c.Conn.(driver.ConnPrepareContext)
	if !ok {
		return c.Prepare(query)
	}
	return preparer.PrepareContext(ctx, query)
}

func (c countingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	beginner, ok := c.Conn.(driver.ConnBeginTx)
	if !ok {
		//nolint:staticcheck // Test wrapper fallback for drivers that do not expose ConnBeginTx.
		return c.Begin()
	}
	return beginner.BeginTx(ctx, opts)
}

func (c countingConn) Ping(ctx context.Context) error {
	pinger, ok := c.Conn.(driver.Pinger)
	if !ok {
		return nil
	}
	return pinger.Ping(ctx)
}
