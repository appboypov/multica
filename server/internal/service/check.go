package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/todo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// CheckDetails is the durable result stored with its inbox item.
type CheckDetails struct {
	ProjectID string `json:"project_id"`
	Kind      string `json:"check_kind"`
	Link      string `json:"link"`
	Sampled   bool   `json:"sampled"`
	Automatic bool   `json:"automatic"`
	Answer    string `json:"answer"`
	Note      string `json:"note"`
	Passed    int    `json:"passed"`
	Feedback  int    `json:"feedback"`
	Streak    int    `json:"streak"`
}

type CheckInput struct {
	IssueID   pgtype.UUID
	ProjectID pgtype.UUID
	MemberID  pgtype.UUID
	Kind      string
	Link      string
	Title     string
}

type CheckSettings struct {
	StreakLength int `json:"streak_length"`
	SampleEvery  int `json:"sample_every"`
}

type CheckTrust struct {
	ProjectID       string      `json:"project_id"`
	Kind            string      `json:"kind"`
	MemberID        string      `json:"member_id"`
	Passed          int         `json:"passed"`
	Feedback        int         `json:"feedback"`
	AutomaticPassed int         `json:"automatic_passed"`
	Streak          int         `json:"streak"`
	Automatic       bool        `json:"automatic"`
	AutomaticOpens  int         `json:"automatic_opens"`
	ProposalID      pgtype.UUID `json:"proposal_id"`
}

func validCheckKind(kind string) bool {
	switch kind {
	case "data_model", "data_flow", "design", "prototype", "feature", "improvement", "agent_tool", "release":
		return true
	}
	return false
}

func checkSample(automatic bool, opens, every int) bool { return automatic && (opens+1)%every == 0 }

func checkSettings(ctx context.Context, tx pgx.Tx, ws pgtype.UUID) (CheckSettings, error) {
	settings := CheckSettings{StreakLength: 5, SampleEvery: 5}
	err := tx.QueryRow(ctx, "SELECT streak_length, sample_every FROM check_settings WHERE workspace_id=$1", ws).Scan(&settings.StreakLength, &settings.SampleEvery)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return settings, err
}

func lockCheckTrust(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID, kind string) (CheckTrust, error) {
	row := CheckTrust{ProjectID: util.UUIDToString(project), Kind: kind}
	var member pgtype.UUID
	err := tx.QueryRow(ctx, `SELECT member_id, passed, feedback, automatic_passed, streak, automatic, automatic_opens, proposal_id
 FROM project_check_trust WHERE workspace_id=$1 AND project_id=$2 AND kind=$3 FOR UPDATE`, ws, project, kind).Scan(&member, &row.Passed, &row.Feedback, &row.AutomaticPassed, &row.Streak, &row.Automatic, &row.AutomaticOpens, &row.ProposalID)
	row.MemberID = util.UUIDToString(member)
	return row, err
}

func saveCheckTrust(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID, state CheckTrust) error {
	_, err := tx.Exec(ctx, `UPDATE project_check_trust SET passed=$4, feedback=$5, automatic_passed=$6, streak=$7,
 automatic=$8, automatic_opens=$9, proposal_id=$10 WHERE workspace_id=$1 AND project_id=$2 AND kind=$3`, ws, project, state.Kind, state.Passed, state.Feedback, state.AutomaticPassed, state.Streak, state.Automatic, state.AutomaticOpens, state.ProposalID)
	return err
}

// OpenCheck uses the asking task as the opener identity and serializes sampling.
func (s *QuestionService) OpenCheck(ctx context.Context, ws pgtype.UUID, actor QuestionActor, in CheckInput) (map[string]any, error) {
	if !validCheckKind(in.Kind) {
		return nil, questionRefusal(422, "unknown check kind")
	}
	link, err := url.Parse(in.Link)
	if err != nil || link.Host == "" || (link.Scheme != "https" && link.Scheme != "http") {
		return nil, questionRefusal(422, "link must be an HTTP or HTTPS URL")
	}
	if strings.TrimSpace(in.Title) == "" || len(in.Title) > 500 {
		return nil, questionRefusal(422, "title must have 1 to 500 characters")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	task, err := s.requireAgentTask(ctx, q, ws, actor)
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM workspace WHERE id=$1 FOR KEY SHARE", ws).Scan(new(pgtype.UUID)); err != nil {
		return nil, questionLoadError(err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM project WHERE id=$1 AND workspace_id=$2 FOR KEY SHARE", in.ProjectID, ws).Scan(new(pgtype.UUID)); err != nil {
		return nil, questionLoadError(err)
	}
	if err := s.validateAnswerer(ctx, q, ws, in.MemberID); err != nil {
		return nil, err
	}
	issue, err := q.GetIssue(ctx, in.IssueID)
	if err != nil || issue.WorkspaceID != ws {
		return nil, questionRefusal(404, "issue not found")
	}
	if task.IssueID != issue.ID {
		return nil, questionRefusal(422, "check issue must be the opening run's issue")
	}
	if !in.ProjectID.Valid || issue.ProjectID != in.ProjectID {
		return nil, questionRefusal(422, "issue must belong to the check project")
	}
	if _, err := q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: in.ProjectID, WorkspaceID: ws}); err != nil {
		return nil, questionRefusal(404, "project not found")
	}
	_, err = tx.Exec(ctx, `INSERT INTO project_check_trust (workspace_id, project_id, kind, member_id) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`, ws, in.ProjectID, in.Kind, in.MemberID)
	if err != nil {
		return nil, err
	}
	state, err := lockCheckTrust(ctx, tx, ws, in.ProjectID, in.Kind)
	if err != nil {
		return nil, err
	}
	if state.MemberID != util.UUIDToString(in.MemberID) {
		return nil, questionRefusal(422, "this project and kind has another check recipient")
	}
	settings, err := checkSettings(ctx, tx, ws)
	if err != nil {
		return nil, err
	}
	sampled := checkSample(state.Automatic, state.AutomaticOpens, settings.SampleEvery)
	details := CheckDetails{ProjectID: state.ProjectID, Kind: in.Kind, Link: in.Link, Sampled: sampled, Automatic: state.Automatic && !sampled, Passed: state.Passed, Feedback: state.Feedback, Streak: state.Streak}
	if state.Automatic {
		state.AutomaticOpens++
	}
	if details.Automatic {
		details.Answer = "pass"
		state.AutomaticPassed++
	}
	data, err := json.Marshal(details)
	if err != nil {
		return nil, err
	}
	row, err := q.OpenTodo(ctx, db.OpenTodoParams{ID: dbid.NewV7(), WorkspaceID: ws, MemberID: in.MemberID, Kind: "check", SourceKey: util.UUIDToString(dbid.NewV7()), IssueID: issue.ID, TaskID: actor.TaskID, ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: actor.ID, Title: in.Title, Details: data})
	if err != nil {
		return nil, err
	}
	if details.Automatic {
		if _, err := q.CloseTodos(ctx, db.CloseTodosParams{WorkspaceID: ws, MemberID: in.MemberID, Kind: "check", SourceKey: row.SourceKey}); err != nil {
			return nil, err
		}
	}
	if err := saveCheckTrust(ctx, tx, ws, in.ProjectID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if !details.Automatic {
		if err := (todo.Service{Queries: s.Queries, Bus: s.Bus}).PublishOpened(ctx, row); err != nil {
			return nil, err
		}
	}
	return s.GetCheck(ctx, ws, row.ID, actor)
}

func (s *QuestionService) GetCheck(ctx context.Context, ws, id pgtype.UUID, actor QuestionActor) (map[string]any, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var member, opener pgtype.UUID
	err = tx.QueryRow(ctx, "SELECT member_id, actor_id FROM todo WHERE id=$1 AND workspace_id=$2 AND kind IN ('check','trust_proposal')", id, ws).Scan(&member, &opener)
	if err != nil {
		return nil, questionLoadError(err)
	}
	if (actor.Type == "member" && actor.ID != member) || (actor.Type == "agent" && actor.ID != opener) {
		return nil, questionRefusal(404, "check not found")
	}
	row, err := s.Queries.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: id, WorkspaceID: ws, MemberID: member})
	if err != nil {
		return nil, err
	}
	return todo.Response(row), nil
}

// AnswerCheck commits the result, record, proposal and wake together.
func (s *QuestionService) AnswerCheck(ctx context.Context, ws, id pgtype.UUID, actor QuestionActor, answer, note string) (map[string]any, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if err := s.requireMember(ctx, q, ws, actor); err != nil {
		return nil, err
	}
	row, err := q.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: id, WorkspaceID: ws, MemberID: actor.ID})
	if err != nil {
		return nil, questionLoadError(err)
	}
	if row.Kind != "check" && row.Kind != "trust_proposal" {
		return nil, questionRefusal(422, "this to-do has no check answer")
	}
	var details CheckDetails
	if err := json.Unmarshal(row.Details, &details); err != nil {
		return nil, err
	}
	project, err := util.ParseUUID(details.ProjectID)
	if err != nil {
		return nil, err
	}
	state, err := lockCheckTrust(ctx, tx, ws, project, details.Kind)
	if err != nil {
		return nil, err
	}
	var done pgtype.Timestamptz
	if err := tx.QueryRow(ctx, "SELECT done_at FROM todo WHERE id=$1 AND workspace_id=$2 FOR UPDATE", id, ws).Scan(&done); err != nil {
		return nil, err
	}
	if done.Valid {
		return s.GetCheck(ctx, ws, id, actor)
	}
	if row.Kind == "trust_proposal" {
		if answer != "yes" && answer != "no" {
			return nil, questionRefusal(422, "answer must be yes or no")
		}
		if state.ProposalID != id || details.Kind == "release" {
			return nil, questionRefusal(422, "proposal is no longer open")
		}
		state.Automatic = answer == "yes"
		state.AutomaticOpens = 0
		state.ProposalID = pgtype.UUID{}
		if answer == "no" {
			state.Streak = 0
		}
	} else {
		if answer != "pass" && answer != "feedback" {
			return nil, questionRefusal(422, "answer must be pass or feedback")
		}
		if answer == "feedback" && strings.TrimSpace(note) == "" {
			return nil, questionRefusal(422, "feedback needs a note")
		}
		if answer == "pass" {
			state.Passed++
			state.Streak++
		} else {
			state.Feedback++
			state.Streak = 0
			state.Automatic = false
			state.AutomaticOpens = 0
		}
	}
	details.Answer, details.Note = answer, note
	data, err := json.Marshal(details)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "UPDATE todo SET details=$3, done_at=now(), done_by='member' WHERE id=$1 AND workspace_id=$2", id, ws, data); err != nil {
		return nil, err
	}
	var opened *db.Todo
	closed := []db.Todo{{ID: id}}
	if row.Kind == "check" {
		settings, err := checkSettings(ctx, tx, ws)
		if err != nil {
			return nil, err
		}
		if answer == "feedback" && state.ProposalID.Valid {
			if _, err := tx.Exec(ctx, "UPDATE todo SET done_at=now(),done_by='handled' WHERE id=$1 AND workspace_id=$2 AND done_at IS NULL", state.ProposalID, ws); err != nil {
				return nil, err
			}
			closed = append(closed, db.Todo{ID: state.ProposalID})
			state.ProposalID = pgtype.UUID{}
		}
		if !state.Automatic && !state.ProposalID.Valid && state.Streak >= settings.StreakLength && state.Kind != "release" {
			state.ProposalID = dbid.NewV7()
			snapshot, err := json.Marshal(CheckDetails{ProjectID: state.ProjectID, Kind: state.Kind, Passed: state.Passed, Feedback: state.Feedback, Streak: state.Streak})
			if err != nil {
				return nil, err
			}
			proposal, err := q.OpenTodo(ctx, db.OpenTodoParams{ID: state.ProposalID, WorkspaceID: ws, MemberID: actor.ID, Kind: "trust_proposal", SourceKey: util.UUIDToString(state.ProposalID), ActorType: row.ActorType, ActorID: row.ActorID, Title: "Make " + state.Kind + " checks automatic?", Details: snapshot})
			if err != nil {
				return nil, err
			}
			opened = &proposal
		}
	}
	if err := saveCheckTrust(ctx, tx, ws, project, state); err != nil {
		return nil, err
	}
	var task db.AgentTaskQueue
	var notified []db.ListRunningSupplementTasksForIssuesRow
	if row.Kind == "check" && s.Tasks != nil {
		issue, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: row.IssueID, WorkspaceID: ws})
		if err != nil {
			return nil, err
		}
		content := fmt.Sprintf("Check %s (%s): %s\n%s\n%s", util.UUIDToString(id), details.Kind, answer, note, details.Link)
		active, err := q.ListRunningSupplementTasksForIssues(ctx, db.ListRunningSupplementTasksForIssuesParams{WorkspaceID: ws, IssueIds: []pgtype.UUID{issue.ID}})
		if err != nil {
			return nil, err
		}
		delivered := false
		for _, run := range active {
			if run.AgentID != row.ActorID {
				continue
			}
			_, err := q.CreateRunInputSupplement(ctx, db.CreateRunInputSupplementParams{TaskID: run.ID, IssueID: issue.ID, WorkspaceID: ws, AuthorID: actor.ID, ItemKind: "answer", ItemID: id, Content: pgtype.Text{String: content, Valid: true}})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return nil, err
			}
			notified = append(notified, run)
			delivered = true
		}
		if !delivered {
			var pending pgtype.UUID
			err = tx.QueryRow(ctx, `SELECT id FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND status IN ('queued','dispatched') AND comment_thread_id IS NULL ORDER BY created_at LIMIT 1 FOR UPDATE`, issue.ID, row.ActorID).Scan(&pending)
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE agent_task_queue SET handoff_note=concat_ws(E'\n\n',NULLIF(handoff_note,''),$2::text) WHERE id=$1`, pending, content)
			} else if errors.Is(err, pgx.ErrNoRows) {
				task, err = s.Tasks.enqueueMentionTaskWithQueries(ctx, q, issue, row.ActorID, pgtype.UUID{}, nil, false, pgtype.UUID{}, false, content, actor.ID, pgtype.UUID{}, OriginNamed, nil)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	publisher := todo.Service{Queries: s.Queries, Bus: s.Bus}
	if err := publisher.PublishClosed(ctx, closed); err != nil {
		return nil, err
	}
	if opened != nil {
		if err := publisher.PublishOpened(ctx, *opened); err != nil {
			return nil, err
		}
	}
	if task.ID.Valid {
		s.Tasks.BroadcastTaskQueued(ctx, task)
		s.Tasks.NotifyTaskEnqueued(ctx, task)
	}
	if s.RunInputs != nil && s.RunInputs.Notifier != nil {
		for _, run := range notified {
			s.RunInputs.Notifier.NotifyTaskSupplementAvailable(util.UUIDToString(run.RuntimeID), util.UUIDToString(run.ID))
		}
	}
	return s.GetCheck(ctx, ws, id, actor)
}

func (s *QuestionService) ListCheckTrust(ctx context.Context, ws, project pgtype.UUID) ([]CheckTrust, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT kind,member_id,passed,feedback,automatic_passed,streak,automatic,automatic_opens,proposal_id FROM project_check_trust WHERE workspace_id=$1 AND project_id=$2 ORDER BY kind`, ws, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []CheckTrust{}
	for rows.Next() {
		row := CheckTrust{ProjectID: util.UUIDToString(project)}
		var member pgtype.UUID
		if err := rows.Scan(&row.Kind, &member, &row.Passed, &row.Feedback, &row.AutomaticPassed, &row.Streak, &row.Automatic, &row.AutomaticOpens, &row.ProposalID); err != nil {
			return nil, err
		}
		row.MemberID = util.UUIDToString(member)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *QuestionService) CheckSettings(ctx context.Context, ws pgtype.UUID, actor QuestionActor, input *CheckSettings) (CheckSettings, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return CheckSettings{}, err
	}
	defer tx.Rollback(ctx)
	if input != nil {
		if err := tx.QueryRow(ctx, "SELECT id FROM workspace WHERE id=$1 FOR KEY SHARE", ws).Scan(new(pgtype.UUID)); err != nil {
			return CheckSettings{}, questionLoadError(err)
		}
		member, err := s.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: actor.ID, WorkspaceID: ws})
		if err != nil {
			return CheckSettings{}, err
		}
		if actor.Type != "member" || (member.Role != "owner" && member.Role != "admin") {
			return CheckSettings{}, questionRefusal(403, "only owners and admins change check settings")
		}
		if input.StreakLength < 1 || input.StreakLength > 100 || input.SampleEvery < 1 || input.SampleEvery > 100 {
			return CheckSettings{}, questionRefusal(422, "settings must be between 1 and 100")
		}
		_, err = tx.Exec(ctx, `INSERT INTO check_settings(workspace_id,streak_length,sample_every) VALUES($1,$2,$3) ON CONFLICT(workspace_id) DO UPDATE SET streak_length=$2,sample_every=$3`, ws, input.StreakLength, input.SampleEvery)
		if err != nil {
			return CheckSettings{}, err
		}
	}
	result, err := checkSettings(ctx, tx, ws)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

// DeleteCheckData removes the record and inbox items in the parent's transaction.
func DeleteCheckData(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID) error {
	if project.Valid {
		if _, err := tx.Exec(ctx, "DELETE FROM project_check_trust WHERE workspace_id=$1 AND project_id=$2", ws, project); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "DELETE FROM todo WHERE workspace_id=$1 AND kind IN ('check','trust_proposal') AND details->>'project_id'=$2", ws, util.UUIDToString(project))
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM project_check_trust WHERE workspace_id=$1", ws); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "DELETE FROM check_settings WHERE workspace_id=$1", ws)
	return err
}
