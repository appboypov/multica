package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/todo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type deciderEffects struct {
	in        deciderInput
	updated   *db.Issue
	before    *db.Issue
	opened    []db.Todo
	closed    []db.Todo
	queued    []db.AgentTaskQueue
	cancelled []db.AgentTaskQueue
	comments  []db.CreateCommentRow
	question  *protocol.QuestionEventPayload
}

type deciderReview struct {
	Verdict        string    `json:"verdict"`
	Choice         string    `json:"choice"`
	Note           string    `json:"note"`
	ProposedChoice string    `json:"proposed_choice"`
	Decider        string    `json:"decider"`
	JevConfidence  float64   `json:"jev_confidence"`
	MemberID       string    `json:"member_id"`
	AnsweredAt     time.Time `json:"answered_at"`
}

func (s *DeciderService) openConfirmation(ctx context.Context, tx pgx.Tx, key string, in deciderInput, answer DeciderAnswer, confidence float64, mode string, effects *deciderEffects) error {
	q := s.Queries.WithTx(tx)
	member := util.MustParseUUID(in.Settings.MemberID)
	proposed := in.Rule.choiceLabel(answer.Choice)
	answerType := question.TypeSingleChoice
	if in.Rule.Decision == "squad" && (len(in.Rule.Squads) < 2 || len(in.Rule.Squads) > 20) {
		answerType = question.TypeText
	}
	place, err := deciderPlace(ctx, tx, in.Issue)
	if err != nil {
		return err
	}
	prompt := deciderPrompt(in.Rule, in.Issue.Title, place, proposed, mode)
	options := make([]string, 0, len(in.Rule.Squads))
	for option := range in.Rule.options() {
		options = append(options, in.Rule.choiceLabel(option))
	}
	sort.Strings(options)
	var settings, suggestion []byte
	if answerType == question.TypeText {
		prompt += "\nSubmit a squad name from the list below.\n" + strings.Join(options, "\n")
		settings, _ = json.Marshal(question.TextAnswerSettings{Length: "short"})
		suggestion, _ = json.Marshal(question.TextAnswerValue{Text: proposed})
	} else {
		settings, _ = json.Marshal(question.SingleChoiceAnswerSettings{Options: options})
		suggestion, _ = json.Marshal(question.SingleChoiceAnswerValue{Option: &proposed})
	}
	id := dbid.NewV7()
	row, err := q.CreateQuestion(ctx, db.CreateQuestionParams{ID: id, WorkspaceID: in.Issue.WorkspaceID, AskerUserID: member, Prompt: prompt, ContextLine: pgtype.Text{String: "Decider", Valid: true}, AnswerType: answerType, Settings: settings, SuggestedAnswer: suggestion, AskID: id, AnswerPolicy: question.PolicyEach})
	if err != nil {
		return err
	}
	if err = q.CreateQuestionLink(ctx, db.CreateQuestionLinkParams{QuestionID: id, WorkspaceID: in.Issue.WorkspaceID, LinkType: question.LinkTypeIssue, LinkID: in.Issue.ID}); err != nil {
		return err
	}
	if err = q.CreateQuestionAnswerers(ctx, db.CreateQuestionAnswerersParams{QuestionID: id, WorkspaceID: in.Issue.WorkspaceID, UserIds: []pgtype.UUID{member}}); err != nil {
		return err
	}
	questions := QuestionService{}
	eventID, err := questions.writeEvent(ctx, q, row, QuestionActor{Type: "member", ID: member}, "asked", map[string]any{"routine_decision_id": key, "review_mode": mode})
	if err != nil {
		return err
	}
	details, _ := json.Marshal(map[string]any{"routine_decision_id": key, "review_mode": mode, "jev_confidence": confidence})
	opened, err := q.OpenTodo(ctx, db.OpenTodoParams{ID: dbid.NewV7(), WorkspaceID: in.Issue.WorkspaceID, MemberID: member, Kind: todo.Question, SourceKey: util.UUIDToString(id), IssueID: in.Issue.ID, QuestionID: id, Title: "Confirm routine choice: " + proposed, Body: pgtype.Text{String: prompt, Valid: true}, Details: details})
	if err != nil {
		return err
	}
	effects.opened = append(effects.opened, opened)
	effects.question = &protocol.QuestionEventPayload{QuestionID: util.UUIDToString(id), AskID: util.UUIDToString(id), QuestionIDs: []string{util.UUIDToString(id)}, IssueID: util.UUIDToString(in.Issue.ID), IssueIDs: []string{util.UUIDToString(in.Issue.ID)}, Reason: "asked", EventID: eventID}
	_, err = tx.Exec(ctx, `UPDATE routine_decision SET question_id=$2,review_mode=$3 WHERE id=$1`, key, id, mode)
	return err
}

// deciderPrompt asks Brian in one plain sentence, the way a colleague would.
func deciderPrompt(rule DeciderRule, title, place, proposed, mode string) string {
	work := fmt.Sprintf("%q", title)
	if place != "" {
		work += " in " + place
	}
	if rule.Decision == "squad" {
		if mode == "after" {
			return fmt.Sprintf("The %s squad took %s. Pick another squad if that is wrong.", proposed, work)
		}
		return fmt.Sprintf("Should the %s squad take %s? Nothing starts until you answer.", proposed, work)
	}
	if mode == "after" {
		return fmt.Sprintf("%s: %s\nThe decider chose %s. Pick the other choice if that is wrong.", work, rule.Question, proposed)
	}
	return fmt.Sprintf("%s: %s\nThe decider says %s. Nothing starts until you answer.", work, rule.Question, proposed)
}

// deciderPlace names the issue's project, with its repository when the
// project's name does not already say it.
func deciderPlace(ctx context.Context, tx pgx.Tx, issue db.Issue) (string, error) {
	if !issue.ProjectID.Valid {
		return "", nil
	}
	var project, repos string
	if err := tx.QueryRow(ctx, `SELECT p.title, coalesce(string_agg(r.full_name, ', ' ORDER BY r.full_name), '') FROM project p LEFT JOIN repository r ON r.project_id=p.id WHERE p.id=$1 GROUP BY p.title`, issue.ProjectID).Scan(&project, &repos); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if repos != "" && !strings.Contains(strings.ToLower(repos), strings.ToLower(project)) {
		project += " (" + repos + ")"
	}
	return project, nil
}

// applyChoice changes only the chosen outcome, preserving the issue's other fields.
func (s *DeciderService) applyChoice(ctx context.Context, tx pgx.Tx, key string, in deciderInput, current db.Issue, choice, reason string, workTask pgtype.UUID, effects *deciderEffects) error {
	q := s.Queries.WithTx(tx)
	source := "decider:bug:" + util.UUIDToString(in.Issue.ID)
	if in.Release != nil {
		source = "decider:release:" + util.UUIDToString(in.Release.ID)
	}
	if choice == "fix_now" || choice == "not_yet" {
		closed, err := q.CloseTodos(ctx, db.CloseTodosParams{WorkspaceID: in.Issue.WorkspaceID, Kind: todo.Review, SourceKey: source})
		if err != nil {
			return err
		}
		effects.closed = append(effects.closed, closed...)
	}
	if workTask.Valid && (choice == "fix_later" || in.Rule.Decision == "squad") {
		cancelled, err := q.CancelAgentTaskByUser(ctx, db.CancelAgentTaskByUserParams{ID: workTask, CancelledByType: pgtype.Text{String: "member", Valid: true}, CancelledByID: util.MustParseUUID(in.Settings.MemberID), CancelledByName: pgtype.Text{String: "Brian", Valid: true}})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			if err = SettleTerminalTaskState(ctx, q, cancelled); err != nil {
				return err
			}
			effects.cancelled = append(effects.cancelled, cancelled)
		}
		workTask = pgtype.UUID{}
	}
	if in.Rule.Decision == "squad" {
		squadID, err := util.ParseUUID(choice)
		if err != nil {
			return err
		}
		squad, err := deciderSquad(ctx, q, in.Issue.WorkspaceID, func(sq db.Squad) bool { return sq.ID == squadID })
		if err != nil {
			return err
		}
		if workTask, err = s.assignSquad(ctx, q, current, current.Status, squad, "This issue is your squad's work. "+reason, effects); err != nil {
			return err
		}
	}
	if in.Rule.Decision == "bug" {
		if choice == "fix_now" {
			var props map[string]json.RawMessage
			_ = json.Unmarshal(current.Properties, &props)
			var kind string
			_ = json.Unmarshal(props[in.Rule.KindProperty], &kind)
			squadID, err := util.ParseUUID(in.Rule.SquadByKind[kind])
			if err != nil {
				return errors.New("issue kind has no configured squad")
			}
			squad, err := deciderSquad(ctx, q, in.Issue.WorkspaceID, func(sq db.Squad) bool { return sq.ID == squadID })
			if err != nil {
				return err
			}
			if workTask, err = s.assignSquad(ctx, q, current, "todo", squad, "Fix this post-merge bug. "+reason, effects); err != nil {
				return err
			}
		} else {
			effects.before = &current
			updated, err := q.UpdateIssue(ctx, db.UpdateIssueParams{ID: current.ID, ExpectedRevision: pgtype.Int8{Int64: current.Revision, Valid: true}, Status: pgtype.Text{String: "backlog", Valid: true}, AssigneeType: in.Issue.AssigneeType, AssigneeID: in.Issue.AssigneeID, StartDate: current.StartDate, DueDate: current.DueDate, ParentIssueID: current.ParentIssueID, ProjectID: current.ProjectID, Stage: current.Stage, AgentQuestions: current.AgentQuestions, DuplicateOfIssueID: current.DuplicateOfIssueID})
			if err != nil {
				return err
			}
			effects.updated = &updated
		}
	}
	if choice == "fix_later" || choice == "ready" {
		title := "Plan bug: " + current.Title
		details := map[string]any{"decision_id": key}
		if choice == "ready" {
			title = "Release check: " + in.Release.Version + " " + in.Release.Title
			details["release_id"] = util.UUIDToString(in.Release.ID)
		}
		encoded, _ := json.Marshal(details)
		opened, err := q.OpenTodo(ctx, db.OpenTodoParams{ID: dbid.NewV7(), WorkspaceID: in.Issue.WorkspaceID, MemberID: util.MustParseUUID(in.Settings.MemberID), Kind: todo.Review, SourceKey: source, IssueID: in.Issue.ID, Title: title, Body: pgtype.Text{String: reason, Valid: true}, Details: encoded})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			effects.opened = append(effects.opened, opened)
		}
	}
	_, err := tx.Exec(ctx, `UPDATE routine_decision SET applied_choice=$2,work_task_id=$3,outcome='applied' WHERE id=$1`, key, choice, workTask)
	return err
}

// deciderSquad finds the active squad that match accepts, with an active leader.
func deciderSquad(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, match func(db.Squad) bool) (db.Squad, error) {
	squads, err := q.ListSquads(ctx, workspaceID)
	if err != nil {
		return db.Squad{}, err
	}
	for _, squad := range squads {
		if !match(squad) {
			continue
		}
		leader, err := q.GetAgent(ctx, squad.LeaderID)
		if err != nil {
			return db.Squad{}, err
		}
		if leader.WorkspaceID != workspaceID || leader.ArchivedAt.Valid {
			return db.Squad{}, errors.New("the chosen squad's leader must be active in this workspace")
		}
		return squad, nil
	}
	return db.Squad{}, errors.New("the chosen squad is not active in this workspace")
}

// assignSquad gives the issue to the squad with status and, outside backlog,
// starts the squad leader's run, as assigning a squad by hand does.
func (s *DeciderService) assignSquad(ctx context.Context, q *db.Queries, current db.Issue, status string, squad db.Squad, note string, effects *deciderEffects) (pgtype.UUID, error) {
	effects.before = &current
	updated, err := q.UpdateIssue(ctx, db.UpdateIssueParams{ID: current.ID, ExpectedRevision: pgtype.Int8{Int64: current.Revision, Valid: true}, Status: pgtype.Text{String: status, Valid: true}, AssigneeType: pgtype.Text{String: "squad", Valid: true}, AssigneeID: squad.ID, StartDate: current.StartDate, DueDate: current.DueDate, ParentIssueID: current.ParentIssueID, ProjectID: current.ProjectID, Stage: current.Stage, AgentQuestions: current.AgentQuestions, DuplicateOfIssueID: current.DuplicateOfIssueID})
	if err != nil {
		return pgtype.UUID{}, err
	}
	effects.updated = &updated
	if status == "backlog" {
		return pgtype.UUID{}, nil
	}
	queued, err := s.Tasks.enqueueMentionTaskWithQueries(ctx, q, updated, squad.LeaderID, pgtype.UUID{}, nil, true, squad.ID, false, note, pgtype.UUID{}, pgtype.UUID{}, OriginNamed, nil, false)
	if err != nil {
		return pgtype.UUID{}, err
	}
	effects.queued = append(effects.queued, queued)
	return queued.ID, nil
}

// reviewConfirmation runs inside the question answer transaction.
func (s *DeciderService) reviewConfirmation(ctx context.Context, tx pgx.Tx, row db.Question, actor QuestionActor, mutation string) (*deciderEffects, error) {
	var key, maker, mode, outcome string
	var raw, proposal, history []byte
	var confidence float64
	var applied pgtype.Text
	var task pgtype.UUID
	err := tx.QueryRow(ctx, `SELECT id,input,proposal,decider,review_mode,outcome,jev_confidence,applied_choice,work_task_id,reviews FROM routine_decision WHERE question_id=$1 AND workspace_id=$2 FOR UPDATE`, row.ID, row.WorkspaceID).Scan(&key, &raw, &proposal, &maker, &mode, &outcome, &confidence, &applied, &task, &history)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var in deciderInput
	var answer DeciderAnswer
	var reviews []deciderReview
	if err = json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(proposal, &answer); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(history, &reviews); err != nil {
		return nil, err
	}
	if actor.Type != "member" || util.UUIDToString(actor.ID) != in.Settings.MemberID {
		return nil, questionRefusal(403, "only Brian can answer a routine decision")
	}
	if mutation != "answered" && mutation != "answer_edited" {
		return nil, questionRefusal(422, "answer this routine choice to confirm or correct it")
	}
	q := s.Queries.WithTx(tx)
	answers, err := q.ListQuestionAnswersByQuestions(ctx, db.ListQuestionAnswersByQuestionsParams{WorkspaceID: row.WorkspaceID, QuestionIds: []pgtype.UUID{row.ID}})
	if err != nil {
		return nil, err
	}
	var chosen string
	var note string
	for _, a := range answers {
		if a.AnswererUserID == actor.ID {
			var label string
			if row.AnswerType == question.TypeText {
				var value question.TextAnswerValue
				if err := json.Unmarshal(a.Value, &value); err != nil {
					return nil, questionRefusal(422, "select a routine choice to confirm or correct it")
				}
				label = value.Text
			} else {
				var value question.SingleChoiceAnswerValue
				if json.Unmarshal(a.Value, &value) != nil || value.Option == nil {
					return nil, questionRefusal(422, "select a routine choice to confirm or correct it")
				}
				label = *value.Option
			}
			for option := range in.Rule.options() {
				if in.Rule.choiceLabel(option) == label {
					chosen = option
					break
				}
			}
			if chosen == "" {
				return nil, questionRefusal(422, "select a routine choice to confirm or correct it")
			}
			note = a.Note.String
		}
	}
	if chosen == "" {
		return nil, questionRefusal(422, "a routine choice is required")
	}
	if len(reviews) > 0 {
		last := reviews[len(reviews)-1]
		if last.Choice == chosen && last.Note == note {
			return nil, nil
		}
	}
	effects := &deciderEffects{in: in}
	current := in.Issue
	stale := outcome == "stale"
	if mode == "before" && !applied.Valid && !stale {
		fresh, ok, err := s.input(ctx, tx, in.Issue, in.Settings, in.Rule)
		if err != nil {
			return nil, err
		}
		ws, err := q.GetWorkspace(ctx, row.WorkspaceID)
		if err != nil {
			return nil, err
		}
		fresh.Settings, err = ParseDeciderSettings(ws.Settings)
		if err != nil {
			return nil, err
		}
		stale = !ok || fresh.key() != key
		current = fresh.Issue
	} else if !stale && (!applied.Valid || applied.String != chosen) {
		if in.Rule.Decision == "release" && chosen == "ready" {
			fresh, ok, err := s.input(ctx, tx, in.Issue, in.Settings, in.Rule)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, questionRefusal(422, "release checks need an unshipped release with all issues done")
			}
			current = fresh.Issue
		} else {
			if _, err = tx.Exec(ctx, `SELECT id FROM issue WHERE id=$1 AND workspace_id=$2 FOR UPDATE`, in.Issue.ID, row.WorkspaceID); err != nil {
				return nil, err
			}
			current, err = q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: in.Issue.ID, WorkspaceID: row.WorkspaceID})
			if err != nil {
				return nil, err
			}
		}
	}
	verdict := "agreed"
	if chosen != answer.Choice {
		verdict = "corrected"
	}
	review := deciderReview{Verdict: verdict, Choice: chosen, Note: note, ProposedChoice: answer.Choice, Decider: maker, JevConfidence: confidence, MemberID: util.UUIDToString(actor.ID), AnsweredAt: time.Now().UTC()}
	reviews = append(reviews, review)
	body := fmt.Sprintf("Routine decision %s: Brian %s %s by %s (Jev confidence %.3f). Proposal reason: %s. Selected: %s. Note: %s", in.Rule.ID, verdict, in.Rule.choiceLabel(answer.Choice), maker, confidence, in.Rule.Reasons[answer.Reason], in.Rule.choiceLabel(chosen), note)
	if stale {
		body += ". Not applied: the input changed."
		if _, err = tx.Exec(ctx, `UPDATE routine_decision SET outcome='stale' WHERE id=$1`, key); err != nil {
			return nil, err
		}
	} else if !applied.Valid || applied.String != chosen {
		reason := in.Rule.Reasons[answer.Reason]
		if chosen != answer.Choice {
			reason = "Brian selected " + in.Rule.choiceLabel(chosen)
		}
		if note != "" {
			reason += ". " + note
		}
		if err = s.applyChoice(ctx, tx, key, in, current, chosen, reason, task, effects); err != nil {
			return nil, err
		}
	}
	encoded, _ := json.Marshal(reviews)
	if _, err = tx.Exec(ctx, `UPDATE routine_decision SET reviews=$2 WHERE id=$1`, key, encoded); err != nil {
		return nil, err
	}
	comment, err := q.CreateComment(ctx, db.CreateCommentParams{ID: dbid.NewV7(), IssueID: in.Issue.ID, WorkspaceID: row.WorkspaceID, AuthorType: "system", AuthorID: pgtype.UUID{Valid: true}, Content: body, Type: "comment"})
	if err != nil {
		return nil, err
	}
	effects.comments = append(effects.comments, comment)
	closed, err := q.CloseTodos(ctx, db.CloseTodosParams{WorkspaceID: row.WorkspaceID, Kind: todo.Question, SourceKey: util.UUIDToString(row.ID)})
	if err != nil {
		return nil, err
	}
	effects.closed = append(effects.closed, closed...)
	return effects, nil
}

func (s *DeciderService) publishDecision(ctx context.Context, effects *deciderEffects) {
	if effects == nil {
		return
	}
	workspaceID := util.UUIDToString(effects.in.Issue.WorkspaceID)
	if s.Bus != nil {
		for _, comment := range effects.comments {
			c := comment.Comment()
			fields := commentEventFields(c)
			fields["revision"] = c.Revision
			s.Bus.Publish(events.Event{Type: protocol.EventCommentCreated, WorkspaceID: workspaceID, ActorType: "system", Payload: map[string]any{"comment": fields, "issue_revision": comment.IssueRevision, "issue_title": effects.in.Issue.Title, "issue_status": effects.in.Issue.Status}})
		}
		if effects.updated != nil {
			ws, err := s.Queries.GetWorkspace(ctx, effects.in.Issue.WorkspaceID)
			if err == nil {
				latest, err := s.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: effects.updated.ID, WorkspaceID: effects.updated.WorkspaceID})
				if err == nil {
					s.Bus.Publish(events.Event{Type: protocol.EventIssueUpdated, WorkspaceID: workspaceID, ActorType: "system", Payload: map[string]any{"issue": IssueToMapResolved(ctx, s.Queries, latest, ws.IssuePrefix), "status_changed": effects.before.Status != latest.Status, "assignee_changed": effects.before.AssigneeID != latest.AssigneeID, "decider_applied": true}})
				} else {
					slog.Error("routine decision issue publication failed", "workspace_id", workspaceID, "error", err)
				}
			} else {
				slog.Error("routine decision workspace publication failed", "workspace_id", workspaceID, "error", err)
			}
		}
		if effects.question != nil {
			s.Bus.Publish(events.Event{Type: protocol.EventQuestionCreated, WorkspaceID: workspaceID, ActorType: "system", Payload: *effects.question})
		}
	}
	todos := todo.Service{Queries: s.Queries, Bus: s.Bus}
	for _, row := range effects.opened {
		if err := todos.PublishOpened(ctx, row); err != nil {
			slog.Error("routine decision todo publication failed", "workspace_id", workspaceID, "error", err)
		}
	}
	if err := todos.PublishClosed(ctx, effects.closed); err != nil {
		slog.Error("routine decision closed todo publication failed", "workspace_id", workspaceID, "error", err)
	}
	if s.Tasks != nil {
		s.Tasks.BroadcastCancelledTasks(ctx, workspaceID, effects.cancelled)
		for _, task := range effects.queued {
			s.Tasks.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
			s.Tasks.NotifyTaskEnqueued(ctx, task)
		}
	}
}
