package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) ListReleaseComments(w http.ResponseWriter, r *http.Request) {
	release, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	h.listSubjectComments(w, r, db.Issue{}, release)
}

func (h *Handler) CreateReleaseComment(w http.ResponseWriter, r *http.Request) {
	r = h.withWakeupActor(r)
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var in CreateCommentRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCommentContentBytes+32768)).Decode(&in) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	in.Content = sanitizeNullBytes(in.Content)
	if in.Content == "" || len(in.Content) > maxCommentContentBytes {
		writeError(w, 400, "content must contain 1–65536 bytes")
		return
	}
	if in.Type == "" {
		in.Type = "comment"
	}
	if !isClientAuthorableCommentType(in.Type) {
		writeError(w, 400, "invalid comment type")
		return
	}
	var parentID pgtype.UUID
	if in.ParentID != nil {
		parentID, ok = parseUUIDOrBadRequest(w, *in.ParentID, "parent_id")
		if !ok {
			return
		}
	}
	attachmentIDs, ok := parseUUIDSliceOrBadRequest(w, in.AttachmentIDs, "attachment_ids")
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, userID, uuidToString(rel.WorkspaceID))
	sourceID := pgtype.UUID{}
	if actorType == "agent" {
		sourceID = h.commentSourceTaskID(r)
	}
	ctx := r.Context()
	tx, err := h.beginWakeupWrite(ctx)
	if err != nil {
		writeError(w, 500, "failed to prepare comment")
		return
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	if _, err = q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
		writeMilestoneReadError(w, err, "repository")
		return
	}
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
		writeMilestoneReadError(w, err, "repository")
		return
	}
	if _, err = q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID}); err != nil {
		writeMilestoneReadError(w, err, "release")
		return
	}
	if parentID.Valid {
		parent, err := q.GetReleaseComment(ctx, db.GetReleaseCommentParams{ID: parentID, WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID})
		if err != nil || parent.DeletedAt.Valid {
			writeError(w, 400, "invalid parent comment")
			return
		}
		path, err := q.LockReleaseCommentAncestorPath(ctx, db.LockReleaseCommentAncestorPathParams{CommentID: parentID, WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID})
		if err != nil {
			writeError(w, 500, "failed to lock comment thread")
			return
		}
		if len(path) == 0 {
			writeError(w, 400, "invalid parent comment")
			return
		}
	}
	if sourceID.Valid {
		task, err := q.GetAgentTask(ctx, sourceID)
		if err != nil || task.AgentID != parseUUID(actorID) {
			writeError(w, 403, "invalid source task")
			return
		}
		if task.ReleaseID == rel.ID {
			// Completion and explicit reports serialize on the repository; replay
			// returns the existing report rather than adding a second result.
			report, reportErr := q.GetReleaseReportBySourceTask(ctx, db.GetReleaseReportBySourceTaskParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, SourceTaskID: sourceID})
			if reportErr == nil && (task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled") {
				writeJSON(w, 200, commentToResponse(report, nil, nil))
				return
			}
			if reportErr != nil && !errors.Is(reportErr, pgx.ErrNoRows) {
				writeError(w, 500, "failed to read source report")
				return
			}
		}
	}
	comment, err := q.CreateReleaseComment(ctx, db.CreateReleaseCommentParams{ID: dbid.NewV7(), ReleaseID: rel.ID, WorkspaceID: rel.WorkspaceID, AuthorType: actorType, AuthorID: parseUUID(actorID), Content: in.Content, Type: in.Type, ParentID: parentID, SourceTaskID: sourceID})
	if err != nil {
		writeError(w, 500, "failed to create comment")
		return
	}
	if len(attachmentIDs) > 0 {
		count, err := q.LinkAttachmentsToReleaseComment(ctx, db.LinkAttachmentsToReleaseCommentParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, CommentID: comment.ID, AttachmentIds: attachmentIDs, UploaderType: actorType, UploaderID: parseUUID(actorID)})
		if err != nil {
			writeError(w, 500, "failed to attach files")
			return
		}
		if count != int64(len(attachmentIDs)) {
			writeError(w, 403, "invalid comment attachments")
			return
		}
	}
	if err = tx.Commit(ctx); err != nil {
		writeError(w, 500, "failed to commit comment")
		return
	}
	h.publishReleaseComment(r, comment)
	if actorType == "member" {
		h.nudgeReleaseCoordination(ctx, rel.ID)
	}
	grouped := h.groupAttachments(r, []pgtype.UUID{comment.ID})
	writeJSON(w, 201, commentToResponse(comment, nil, grouped[uuidToString(comment.ID)]))
}

func (h *Handler) publishReleaseComment(r *http.Request, comment db.Comment) {
	ws := uuidToString(comment.WorkspaceID)
	actorType, actorID := h.resolveActor(r, requestUserID(r), ws)
	h.publish(protocol.EventReleaseCommentCreated, ws, actorType, actorID, protocol.ReleaseCommentEventPayload{ReleaseID: uuidToString(comment.ReleaseID), CommentID: uuidToString(comment.ID)})
}

func (h *Handler) authorizeReleaseCommentAuthor(w http.ResponseWriter, r *http.Request, comment db.Comment) bool {
	if _, ok := h.workspaceMember(w, r, uuidToString(comment.WorkspaceID)); !ok {
		return false
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(comment.WorkspaceID))
	if actorType != comment.AuthorType || actorID != uuidToString(comment.AuthorID) {
		writeError(w, 403, "only comment author can change this comment")
		return false
	}
	if _, err := h.Queries.GetReleaseInWorkspace(r.Context(), db.GetReleaseInWorkspaceParams{ID: comment.ReleaseID, WorkspaceID: comment.WorkspaceID}); err != nil {
		writeError(w, 404, "release not found")
		return false
	}
	return true
}

func (h *Handler) updateReleaseComment(w http.ResponseWriter, r *http.Request, existing db.Comment) {
	if !h.authorizeReleaseCommentAuthor(w, r, existing) {
		return
	}
	var in struct {
		Content          string    `json:"content"`
		ContentBase      *string   `json:"content_base"`
		ExpectedRevision *int64    `json:"expected_revision"`
		AttachmentIDs    *[]string `json:"attachment_ids"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	in.Content = sanitizeNullBytes(in.Content)
	if in.Content == "" || len(in.Content) > maxCommentContentBytes {
		writeError(w, 400, "content must contain 1–65536 bytes")
		return
	}
	params := db.UpdateCommentParams{ID: existing.ID, Content: in.Content, SourceTaskID: existing.SourceTaskID}
	if in.ContentBase != nil {
		params.ContentBase = strToText(sanitizeNullBytes(*in.ContentBase))
	}
	if in.ExpectedRevision != nil {
		params.ExpectedRevision = pgtype.Int8{Int64: *in.ExpectedRevision, Valid: true}
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, 500, "failed to prepare comment edit")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	updated, err := q.UpdateComment(r.Context(), params)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "comment changed; refresh and retry")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to update comment")
		return
	}
	comment := updated.Comment()
	if in.AttachmentIDs != nil {
		ids, ok := parseUUIDSliceOrBadRequest(w, *in.AttachmentIDs, "attachment_ids")
		if !ok {
			return
		}
		actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(existing.WorkspaceID))
		count, err := q.ReplaceReleaseCommentAttachments(r.Context(), db.ReplaceReleaseCommentAttachmentsParams{WorkspaceID: existing.WorkspaceID, ReleaseID: existing.ReleaseID, CommentID: existing.ID, AttachmentIds: ids, UploaderType: actorType, UploaderID: parseUUID(actorID)})
		if err != nil {
			writeError(w, 500, "failed to attach files")
			return
		}
		if count != int64(len(ids)) {
			writeError(w, 403, "invalid comment attachments")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit comment")
		return
	}
	h.publishReleaseComment(r, comment)
	grouped := h.groupAttachments(r, []pgtype.UUID{comment.ID})
	writeJSON(w, 200, commentToResponse(comment, nil, grouped[uuidToString(comment.ID)]))
}

func (h *Handler) deleteReleaseComment(w http.ResponseWriter, r *http.Request, comment db.Comment) {
	if !h.authorizeReleaseCommentAuthor(w, r, comment) {
		return
	}
	deleted, err := h.deleteComment(r.Context(), comment.ID, comment.WorkspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "comment not found")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to delete comment")
		return
	}
	h.deleteS3Objects(r.Context(), deleted.AttachmentURLs)
	h.publishReleaseComment(r, comment)
	w.WriteHeader(204)
}
