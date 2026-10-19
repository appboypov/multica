package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/question"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// folderSkillOriginType marks a library skill that comes from a skill folder.
const folderSkillOriginType = "local_folder"

// SkillFolderResponse is a folder on one machine whose skills are in the
// workspace's library.
type SkillFolderResponse struct {
	ID        string  `json:"id"`
	DaemonID  string  `json:"daemon_id"`
	Path      string  `json:"path"`
	CreatedBy *string `json:"created_by"`
	CreatedAt string  `json:"created_at"`
}

func skillFolderToResponse(f db.SkillFolder) SkillFolderResponse {
	return SkillFolderResponse{
		ID:        uuidToString(f.ID),
		DaemonID:  f.DaemonID,
		Path:      f.Path,
		CreatedBy: uuidToPtr(f.CreatedBy),
		CreatedAt: timestampToString(f.CreatedAt),
	}
}

// folderSkillReport is one skill of a skill folder as the daemon reads it.
type folderSkillReport struct {
	FolderID    string `json:"folder_id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	SourcePath  string `json:"source_path"`
}

type folderSkillOrigin struct {
	Type       string `json:"type"`
	FolderID   string `json:"folder_id"`
	DaemonID   string `json:"daemon_id"`
	Key        string `json:"key"`
	SourcePath string `json:"source_path"`
}

// ListSkillFolders lists the workspace's skill folders.
func (h *Handler) ListSkillFolders(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	folders, err := h.Queries.ListSkillFolders(r.Context(), workspaceUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skill folders")
		return
	}
	resp := make([]SkillFolderResponse, len(folders))
	for i, f := range folders {
		resp[i] = skillFolderToResponse(f)
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateSkillFolder adds a folder on the runtime's machine to the library.
// Only the runtime's owner may add one, since the folder's files are read
// from that machine.
func (h *Handler) CreateSkillFolder(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireRuntimeLocalSkillAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeAPI, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	folderPath := path.Clean(strings.TrimSpace(req.Path))
	if !path.IsAbs(folderPath) {
		writeError(w, http.StatusBadRequest, "path must be absolute")
		return
	}
	daemonID, ok := h.runtimeDaemonID(w, r, rt.runtimeID)
	if !ok {
		return
	}
	folder, err := h.Queries.CreateSkillFolder(r.Context(), db.CreateSkillFolderParams{
		WorkspaceID: parseUUID(rt.workspaceID),
		DaemonID:    daemonID,
		Path:        folderPath,
		CreatedBy:   parseUUID(requestUserID(r)),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "this folder is already in the library")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add skill folder")
		return
	}
	slog.Info("skill folder added", "folder_id", uuidToString(folder.ID), "daemon_id", daemonID, "path", folderPath)
	writeJSON(w, http.StatusCreated, skillFolderToResponse(folder))
}

// DeleteSkillFolder removes a folder from the library together with its
// skills, which also takes them off every agent that had them.
func (h *Handler) DeleteSkillFolder(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.requireWorkspaceRole(w, r, workspaceID, "skill folder not found", "owner", "admin", "member")
	if !ok {
		return
	}
	folderUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "id")
	if !ok {
		return
	}
	workspaceUUID := parseUUID(workspaceID)
	folder, err := h.Queries.GetSkillFolderInWorkspace(r.Context(), db.GetSkillFolderInWorkspaceParams{ID: folderUUID, WorkspaceID: workspaceUUID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "skill folder not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load skill folder")
		return
	}
	isCreator := folder.CreatedBy.Valid && uuidToString(folder.CreatedBy) == requestUserID(r)
	if !isCreator && !roleAllowed(member.Role, "owner", "admin") {
		writeError(w, http.StatusForbidden, "only the folder's creator can remove it")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	skills, err := qtx.ListSkillsByFolder(r.Context(), db.ListSkillsByFolderParams{WorkspaceID: workspaceUUID, FolderID: uuidToString(folder.ID)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list folder skills")
		return
	}
	questionIDs, err := h.deleteLibrarySkills(r.Context(), qtx, workspaceUUID, skills)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete folder skills")
		return
	}
	if err := qtx.DeleteSkillFolder(r.Context(), db.DeleteSkillFolderParams{ID: folder.ID, WorkspaceID: workspaceUUID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete skill folder")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit skill folder deletion")
		return
	}
	h.QuestionService.PublishLinkChanges(r.Context(), workspaceUUID, questionIDs)
	actorType, actorID := h.resolveActor(r, requestUserID(r), workspaceID)
	for _, sk := range skills {
		h.publish(protocol.EventSkillDeleted, workspaceID, actorType, actorID, map[string]any{"skill_id": uuidToString(sk.ID)})
	}
	slog.Info("skill folder removed", "folder_id", uuidToString(folder.ID), "skills", len(skills))
	w.WriteHeader(http.StatusNoContent)
}

// ScanSkillFolders asks the runtime's daemon to read the skill folders on its
// machine. The returned request is polled like a local-skills request; when
// the daemon reports, the library is brought in line with the folders.
func (h *Handler) ScanSkillFolders(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireRuntimeLocalSkillAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeAPI, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	if rt.status != "online" {
		writeError(w, http.StatusServiceUnavailable, "runtime is offline")
		return
	}
	daemonID, ok := h.runtimeDaemonID(w, r, rt.runtimeID)
	if !ok {
		return
	}
	folders, err := h.Queries.ListSkillFoldersByDaemon(r.Context(), db.ListSkillFoldersByDaemonParams{
		WorkspaceID: parseUUID(rt.workspaceID),
		DaemonID:    daemonID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skill folders")
		return
	}
	if len(folders) == 0 {
		writeError(w, http.StatusNotFound, "no skill folders on this machine")
		return
	}
	refs := make([]protocol.DaemonSkillFolder, len(folders))
	for i, f := range folders {
		refs[i] = protocol.DaemonSkillFolder{ID: uuidToString(f.ID), Path: f.Path}
	}
	req, err := h.LocalSkillListStore.Create(r.Context(), rt.runtimeID, refs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue skill folder scan: "+err.Error())
		return
	}
	h.requestDaemonPendingWork(rt.runtimeID, protocol.PendingWorkKindLocalSkills)
	writeJSON(w, http.StatusOK, req)
}

func (h *Handler) runtimeDaemonID(w http.ResponseWriter, r *http.Request, runtimeID string) (string, bool) {
	runtime, err := h.getAgentRuntime(r.Context(), obsmetrics.RuntimeLookupSourceRuntimeAPI, parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load runtime")
		return "", false
	}
	if !runtime.DaemonID.Valid || runtime.DaemonID.String == "" {
		writeError(w, http.StatusBadRequest, "runtime has no machine")
		return "", false
	}
	return runtime.DaemonID.String, true
}

// syncFolderSkills brings the library in line with a scan of skill folders:
// each scanned skill updates its library row or gets a new one, and each
// library row whose skill is gone is deleted. A new skill whose name another
// library skill already has stays out; it is returned so the scan shows it.
func (h *Handler) syncFolderSkills(ctx context.Context, workspaceID pgtype.UUID, requested []protocol.DaemonSkillFolder, scanned []folderSkillReport) ([]RuntimeLocalSkillSummary, error) {
	folders, err := h.Queries.ListSkillFolders(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list skill folders: %w", err)
	}
	wanted := make(map[string]struct{}, len(requested))
	for _, f := range requested {
		wanted[f.ID] = struct{}{}
	}
	scannedByFolder := make(map[string][]folderSkillReport, len(requested))
	for _, sk := range scanned {
		scannedByFolder[sk.FolderID] = append(scannedByFolder[sk.FolderID], sk)
	}

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)

	var created, updated []db.Skill
	var removed []db.Skill
	skipped := make([]RuntimeLocalSkillSummary, 0)
	for _, folder := range folders {
		folderID := uuidToString(folder.ID)
		if _, ok := wanted[folderID]; !ok {
			continue
		}
		existing, err := qtx.ListSkillsByFolder(ctx, db.ListSkillsByFolderParams{WorkspaceID: workspaceID, FolderID: folderID})
		if err != nil {
			return nil, fmt.Errorf("list folder skills: %w", err)
		}
		byKey := make(map[string]db.Skill, len(existing))
		for _, sk := range existing {
			byKey[folderSkillKey(sk.Config)] = sk
		}
		seen := make(map[string]struct{}, len(scannedByFolder[folderID]))
		for _, report := range scannedByFolder[folderID] {
			seen[report.Key] = struct{}{}
			config, err := json.Marshal(map[string]any{"origin": folderSkillOrigin{
				Type:       folderSkillOriginType,
				FolderID:   folderID,
				DaemonID:   folder.DaemonID,
				Key:        report.Key,
				SourcePath: report.SourcePath,
			}})
			if err != nil {
				return nil, fmt.Errorf("encode folder skill origin: %w", err)
			}
			name := sanitizeNullBytes(strings.TrimSpace(report.Name))
			holder, taken, err := h.lookupSkillByNameTx(ctx, qtx, workspaceID, name)
			if err != nil {
				return nil, err
			}
			if current, ok := byKey[report.Key]; ok {
				params := db.UpdateSkillParams{
					ID:          current.ID,
					Description: pgtype.Text{String: sanitizeNullBytes(report.Description), Valid: true},
					Content:     pgtype.Text{String: sanitizeNullBytes(report.Content), Valid: true},
					Config:      config,
				}
				if name != current.Name && (!taken || holder.ID == current.ID) {
					params.Name = pgtype.Text{String: name, Valid: true}
				}
				row, err := qtx.UpdateSkill(ctx, params)
				if err != nil {
					return nil, fmt.Errorf("update folder skill: %w", err)
				}
				updated = append(updated, row)
				continue
			}
			if taken {
				skipped = append(skipped, RuntimeLocalSkillSummary{
					Key:         report.Key,
					Name:        name,
					Description: report.Description,
					SourcePath:  report.SourcePath,
					Root:        "folder",
				})
				continue
			}
			row, err := qtx.CreateSkill(ctx, db.CreateSkillParams{
				WorkspaceID: workspaceID,
				Name:        name,
				Description: sanitizeNullBytes(report.Description),
				Content:     sanitizeNullBytes(report.Content),
				Config:      config,
				CreatedBy:   folder.CreatedBy,
			})
			if err != nil {
				return nil, fmt.Errorf("create folder skill: %w", err)
			}
			created = append(created, row)
		}
		for key, sk := range byKey {
			if _, ok := seen[key]; !ok {
				removed = append(removed, sk)
			}
		}
	}
	questionIDs, err := h.deleteLibrarySkills(ctx, qtx, workspaceID, removed)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit folder skill sync: %w", err)
	}

	workspace := uuidToString(workspaceID)
	h.QuestionService.PublishLinkChanges(ctx, workspaceID, questionIDs)
	for _, sk := range created {
		h.publish(protocol.EventSkillCreated, workspace, "system", "", map[string]any{"skill": skillToResponse(sk)})
	}
	for _, sk := range updated {
		h.publish(protocol.EventSkillUpdated, workspace, "system", "", map[string]any{"skill": skillToResponse(sk)})
	}
	for _, sk := range removed {
		h.publish(protocol.EventSkillDeleted, workspace, "system", "", map[string]any{"skill_id": uuidToString(sk.ID)})
	}
	slog.Info("skill folders synced", "workspace_id", workspace, "created", len(created), "updated", len(updated), "removed", len(removed), "name_taken", len(skipped))
	return skipped, nil
}

func (h *Handler) lookupSkillByNameTx(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID, name string) (db.Skill, bool, error) {
	sk, err := qtx.GetSkillByWorkspaceAndName(ctx, db.GetSkillByWorkspaceAndNameParams{WorkspaceID: workspaceID, Name: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Skill{}, false, nil
		}
		return db.Skill{}, false, fmt.Errorf("look up skill by name: %w", err)
	}
	return sk, true, nil
}

// deleteLibrarySkills deletes skill rows with their label assignments and
// question links, and returns the questions whose links changed.
func (h *Handler) deleteLibrarySkills(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID, skills []db.Skill) ([]pgtype.UUID, error) {
	if len(skills) == 0 {
		return nil, nil
	}
	ids := make([]pgtype.UUID, len(skills))
	for i, sk := range skills {
		ids[i] = sk.ID
		if err := qtx.DeleteSkillLabelAssignmentsBySkill(ctx, sk.ID); err != nil {
			return nil, fmt.Errorf("remove skill label assignments: %w", err)
		}
	}
	questionIDs, err := h.QuestionService.DropLinksTo(ctx, qtx, workspaceID, question.LinkTypeSkill, ids)
	if err != nil {
		return nil, fmt.Errorf("remove skill question links: %w", err)
	}
	for _, sk := range skills {
		if err := qtx.DeleteSkill(ctx, db.DeleteSkillParams{ID: sk.ID, WorkspaceID: workspaceID}); err != nil {
			return nil, fmt.Errorf("delete skill: %w", err)
		}
	}
	return questionIDs, nil
}

func folderSkillKey(config []byte) string {
	var parsed struct {
		Origin folderSkillOrigin `json:"origin"`
	}
	if json.Unmarshal(config, &parsed) != nil {
		return ""
	}
	return parsed.Origin.Key
}
