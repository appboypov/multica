package service

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
)

type CheckHistoryItem struct {
	ID string `json:"id"`
	Title string `json:"title"`
	CheckDetails
	AnsweredAt pgtype.Timestamptz `json:"answered_at"`
}

func (s *QuestionService) SetCheckSampling(ctx context.Context, ws, project pgtype.UUID, kind string, actor QuestionActor, sampling bool) (CheckTrust, error) {
	if !validCheckKind(kind) { return CheckTrust{}, questionRefusal(422, "unknown check kind") }
	if kind == "release" && sampling { return CheckTrust{}, questionRefusal(422, "release checks every piece") }
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil { return CheckTrust{}, err }
	defer tx.Rollback(ctx)
	if err := s.requireMember(ctx, s.Queries.WithTx(tx), ws, actor); err != nil { return CheckTrust{}, err }
	state, err := lockCheckTrust(ctx, tx, ws, project, kind)
	if err != nil { return CheckTrust{}, questionLoadError(err) }
	if state.MemberID != util.UUIDToString(actor.ID) { return CheckTrust{}, questionRefusal(403, "only the check recipient changes sampling") }
	if state.Sampling != sampling {
		state.Sampling = sampling
		state.SampleOpens = 0
		if err := saveCheckTrust(ctx, tx, ws, project, state); err != nil { return CheckTrust{}, err }
	}
	state.CanManage = true
	return state, tx.Commit(ctx)
}

func (s *QuestionService) CheckHistory(ctx context.Context, ws, project pgtype.UUID, kind string) ([]CheckHistoryItem, error) {
	if !validCheckKind(kind) { return nil, questionRefusal(422, "unknown check kind") }
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil { return nil, err }
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,title,details,done_at FROM todo WHERE workspace_id=$1 AND kind='check' AND details->>'project_id'=$2 AND details->>'check_kind'=$3 AND done_at IS NOT NULL ORDER BY created_at DESC,id DESC`, ws, util.UUIDToString(project), kind)
	if err != nil { return nil, err }
	defer rows.Close()
	items := []CheckHistoryItem{}
	for rows.Next() {
		var item CheckHistoryItem
		var id pgtype.UUID
		var details []byte
		if err := rows.Scan(&id, &item.Title, &details, &item.AnsweredAt); err != nil { return nil, err }
		item.ID = util.UUIDToString(id)
		if err := json.Unmarshal(details, &item.CheckDetails); err != nil { return nil, err }
		items = append(items, item)
	}
	return items, rows.Err()
}
