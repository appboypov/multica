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
	SampleEvery int `json:"sample_every"`
}

type CheckTrust struct {
	ProjectID       string `json:"project_id"`
	Kind            string `json:"kind"`
	MemberID        string `json:"member_id"`
	Passed          int    `json:"passed"`
	Feedback        int    `json:"feedback"`
	AutomaticPassed int    `json:"automatic_passed"`
	Sampling        bool   `json:"sampling"`
	SampleOpens     int    `json:"sample_opens"`
	CanManage       bool   `json:"can_manage"`
}

func validCheckKind(kind string) bool {
	switch kind {
	case "data_model", "data_flow", "design", "prototype", "feature", "improvement", "agent_tool", "release":
		return true
	}
	return false
}

func checkSample(sampling bool, opens, every int) bool { return sampling && (opens+1)%every == 0 }

func checkSettings(ctx context.Context, tx pgx.Tx, ws pgtype.UUID) (CheckSettings, error) {
	settings := CheckSettings{SampleEvery: 5}
	err := tx.QueryRow(ctx, "SELECT sample_every FROM check_settings WHERE workspace_id=$1", ws).Scan(&settings.SampleEvery)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return settings, err
}

func lockCheckTrust(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID, kind string) (CheckTrust, error) {
	row := CheckTrust{ProjectID: util.UUIDToString(project), Kind: kind}
	var member pgtype.UUID
	err := tx.QueryRow(ctx, `SELECT member_id, passed, feedback, automatic_passed, sampling, sample_opens
 FROM project_check_trust WHERE workspace_id=$1 AND project_id=$2 AND kind=$3 FOR UPDATE`, ws, project, kind).Scan(&member, &row.Passed, &row.Feedback, &row.AutomaticPassed, &row.Sampling, &row.SampleOpens)
	row.MemberID = util.UUIDToString(member)
	return row, err
}

func saveCheckTrust(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID, state CheckTrust) error {
	_, err := tx.Exec(ctx, `UPDATE project_check_trust SET passed=$4, feedback=$5, automatic_passed=$6,
 sampling=$7, sample_opens=$8 WHERE workspace_id=$1 AND project_id=$2 AND kind=$3`, ws, project, state.Kind, state.Passed, state.Feedback, state.AutomaticPassed, state.Sampling, state.SampleOpens)
	return err
}

// OpenCheck opens a check for an agent in a run on its issue or for a member,
// and serializes sampling.
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
	var task db.AgentTaskQueue
	if actor.Type == "member" {
		err = s.requireMember(ctx, q, ws, actor)
	} else {
		task, err = s.requireAgentTask(ctx, q, ws, actor)
	}
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
	if actor.Type == "agent" && task.IssueID != issue.ID {
		return nil, questionRefusal(422, "check issue must be the opening run's issue")
	}
	if !in.ProjectID.Valid || issue.ProjectID != in.ProjectID {
		return nil, questionRefusal(422, "issue must belong to the check project")
	}
	if _, err := q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: in.ProjectID, WorkspaceID: ws}); err != nil {
		return nil, questionRefusal(404, "project not found")
	}
	_, err = tx.Exec(ctx, `INSERT INTO project_check_trust (workspace_id, project_id, kind, member_id) VALUES ($1,$2,$3,$4)
 ON CONFLICT (workspace_id, project_id, kind) DO UPDATE SET member_id=EXCLUDED.member_id`, ws, in.ProjectID, in.Kind, in.MemberID)
	if err != nil {
		return nil, err
	}
	state, err := lockCheckTrust(ctx, tx, ws, in.ProjectID, in.Kind)
	if err != nil {
		return nil, err
	}
	settings, err := checkSettings(ctx, tx, ws)
	if err != nil {
		return nil, err
	}
	sampled := checkSample(state.Sampling, state.SampleOpens, settings.SampleEvery)
	details := CheckDetails{ProjectID: state.ProjectID, Kind: in.Kind, Link: in.Link, Sampled: sampled, Automatic: state.Sampling && !sampled}
	if state.Sampling {
		state.SampleOpens++
	}
	if details.Automatic {
		details.Answer = "pass"
		state.AutomaticPassed++
	}
	data, err := json.Marshal(details)
	if err != nil {
		return nil, err
	}
	row, err := q.OpenTodo(ctx, db.OpenTodoParams{ID: dbid.NewV7(), WorkspaceID: ws, MemberID: in.MemberID, Kind: "check", SourceKey: util.UUIDToString(dbid.NewV7()), IssueID: issue.ID, TaskID: actor.TaskID, ActorType: pgtype.Text{String: actor.Type, Valid: true}, ActorID: actor.ID, Title: in.Title, Details: data})
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
	return readCheck(ctx, tx, s.Queries.WithTx(tx), ws, id, actor)
}

// readCheck reads on the caller's transaction, so a check path that holds a
// connection never waits for a second one.
func readCheck(ctx context.Context, tx pgx.Tx, q *db.Queries, ws, id pgtype.UUID, actor QuestionActor) (map[string]any, error) {
	var member, opener pgtype.UUID
	var openerType pgtype.Text
	err := tx.QueryRow(ctx, "SELECT member_id, actor_type, actor_id FROM todo WHERE id=$1 AND workspace_id=$2 AND kind IN ('check','instruction_change')", id, ws).Scan(&member, &openerType, &opener)
	if err != nil {
		return nil, questionLoadError(err)
	}
	recipient := actor.Type == "member" && actor.ID == member
	opening := actor.Type == openerType.String && actor.ID == opener
	if !recipient && !opening {
		return nil, questionRefusal(404, "check not found")
	}
	row, err := q.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: id, WorkspaceID: ws, MemberID: member})
	if err != nil {
		return nil, err
	}
	return todo.Response(row), nil
}

// AnswerCheck commits a check result and its opener delivery together.
func (s *QuestionService) AnswerCheck(ctx context.Context, ws, id pgtype.UUID, actor QuestionActor, answer, note string) (map[string]any, error) {
	if answer == "keep" || answer == "undo" {
		return s.AnswerInstructionNote(ctx, ws, id, actor, answer)
	}
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
	if row.Kind != "check" {
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
		return readCheck(ctx, tx, q, ws, id, actor)
	}
	if answer != "pass" && answer != "feedback" {
		return nil, questionRefusal(422, "answer must be pass or feedback")
	}
	if answer == "feedback" && strings.TrimSpace(note) == "" {
		return nil, questionRefusal(422, "feedback needs a note")
	}
	if answer == "pass" {
		state.Passed++
	} else {
		state.Feedback++
	}
	details.Answer, details.Note = answer, note
	data, err := json.Marshal(details)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "UPDATE todo SET details=$3, done_at=now(), done_by='member' WHERE id=$1 AND workspace_id=$2", id, ws, data); err != nil {
		return nil, err
	}
	closed := []db.Todo{{ID: id}}
	if err := saveCheckTrust(ctx, tx, ws, project, state); err != nil {
		return nil, err
	}
	var task db.AgentTaskQueue
	var notified []db.ListRunningSupplementTasksForIssuesRow
	// A member who opened the check reads its answer; only an opening agent gets it delivered.
	if row.ActorType.String == "agent" {
		content := fmt.Sprintf("Check %s (%s): %s\n%s\n%s", util.UUIDToString(id), details.Kind, answer, note, details.Link)
		if answer == "feedback" {
			content += fmt.Sprintf("\nFix this piece and change the instruction that caused the problem. Apply the instruction change yourself, then post its file, summary and commit link with: multica todo instruction-note %s --file <path> --link <commit-URL> --note <summary>. Brian can Keep the change or ask you to Undo it.", util.UUIDToString(id))
		}
		task, notified, err = s.deliverCheckMessage(ctx, tx, q, ws, row.IssueID, row.ActorID, actor.ID, id, content)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	publisher := todo.Service{Queries: s.Queries, Bus: s.Bus}
	if err := publisher.PublishClosed(ctx, closed); err != nil {
		return nil, err
	}
	s.publishCheckDelivery(ctx, task, notified)
	return s.GetCheck(ctx, ws, id, actor)
}

func (s *QuestionService) ListCheckTrust(ctx context.Context, ws, project pgtype.UUID) ([]CheckTrust, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT kind,member_id,passed,feedback,automatic_passed,sampling,sample_opens FROM project_check_trust WHERE workspace_id=$1 AND project_id=$2 ORDER BY kind`, ws, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []CheckTrust{}
	for rows.Next() {
		row := CheckTrust{ProjectID: util.UUIDToString(project)}
		var member pgtype.UUID
		if err := rows.Scan(&row.Kind, &member, &row.Passed, &row.Feedback, &row.AutomaticPassed, &row.Sampling, &row.SampleOpens); err != nil {
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
		member, err := s.Queries.WithTx(tx).GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: actor.ID, WorkspaceID: ws})
		if err != nil {
			return CheckSettings{}, err
		}
		if actor.Type != "member" || (member.Role != "owner" && member.Role != "admin") {
			return CheckSettings{}, questionRefusal(403, "only owners and admins change check settings")
		}
		if input.SampleEvery < 1 || input.SampleEvery > 100 {
			return CheckSettings{}, questionRefusal(422, "settings must be between 1 and 100")
		}
		_, err = tx.Exec(ctx, `INSERT INTO check_settings(workspace_id,sample_every) VALUES($1,$2) ON CONFLICT(workspace_id) DO UPDATE SET sample_every=$2`, ws, input.SampleEvery)
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
		_, err := tx.Exec(ctx, "DELETE FROM todo WHERE workspace_id=$1 AND kind IN ('check','instruction_change') AND details->>'project_id'=$2", ws, util.UUIDToString(project))
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM project_check_trust WHERE workspace_id=$1", ws); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "DELETE FROM check_settings WHERE workspace_id=$1", ws)
	return err
}
