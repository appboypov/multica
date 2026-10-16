package handler

import (
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestReleaseCommentsStayOnTheirSubjectAndKeepReplies(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	_, _, other := milestoneReleaseFixture(t, testWorkspaceID)
	var root, reply CommentResponse
	testutil.Call(t, testHandler.CreateReleaseComment, milestoneRequest("POST", release, map[string]any{"content": "Coordinate this release"})).Want(201).JSON(&root)
	if root.ReleaseID == nil || *root.ReleaseID != release || root.AuthorType != "member" || root.AuthorID != testUserID {
		t.Fatalf("release root = %+v", root)
	}
	testutil.Call(t, testHandler.CreateReleaseComment, milestoneRequest("POST", release, map[string]any{"content": "Reply context", "parent_id": root.ID})).Want(201).JSON(&reply)
	issue := dbfx.Issue(t, "Unrelated issue")
	issueComment := dbfx.Comment(t, issue, "Issue root")
	for _, parent := range []string{root.ID, issueComment} {
		target := other
		if parent == issueComment {
			target = release
		}
		testutil.Call(t, testHandler.CreateReleaseComment, milestoneRequest("POST", target, map[string]any{"content": "Wrong parent", "parent_id": parent})).Want(400)
	}
	var comments []CommentResponse
	testutil.Call(t, testHandler.ListReleaseComments, milestoneRequest("GET", release, nil)).Want(200).JSON(&comments)
	if len(comments) != 2 || comments[0].ID != root.ID || comments[1].ID != reply.ID || comments[1].ParentID == nil || *comments[1].ParentID != root.ID {
		t.Fatalf("chronological thread = %+v", comments)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE release_id=$1", release); n != 0 {
		t.Fatalf("unowned release enqueued %d tasks", n)
	}
}

func TestReleaseCommentNewestWindowCompletesAnOldRoot(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	oldCap, oldBudget := commentHardCap, commentThreadContextBudget
	commentHardCap, commentThreadContextBudget = 2, 4
	t.Cleanup(func() { commentHardCap, commentThreadContextBudget = oldCap, oldBudget })
	root := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": release, "author_type": "member", "author_id": testUserID, "content": "Old root", "created_at": time.Now().Add(-time.Hour)})
	first := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": release, "author_type": "member", "author_id": testUserID, "content": "First reply", "parent_id": root, "created_at": time.Now().Add(-time.Minute)})
	last := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": release, "author_type": "member", "author_id": testUserID, "content": "Last reply", "parent_id": root})
	var out []CommentResponse
	testutil.Call(t, testHandler.ListReleaseComments, milestoneRequest("GET", release, nil)).Want(200).JSON(&out)
	if len(out) != 3 || out[0].ID != root || out[1].ID != first || out[2].ID != last {
		t.Fatalf("capped thread lost context: %+v", out)
	}
}

func TestReleaseCommentRejectsIssueAttachmentsAndOtherAuthors(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	issue := dbfx.Issue(t, "Attachment owner")
	attachment := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": issue, "uploader_type": "member", "uploader_id": testUserID, "filename": "scope.txt", "url": "https://example.test/scope.txt", "content_type": "text/plain", "size_bytes": 1})
	testutil.Call(t, testHandler.CreateReleaseComment, milestoneRequest("POST", release, map[string]any{"content": "Cannot borrow this file", "attachment_ids": []string{attachment}})).Want(403)
	if n := dbfx.Count(t, "SELECT count(*) FROM comment WHERE release_id=$1", release); n != 0 {
		t.Fatalf("refused attachment retained %d comments", n)
	}
	agent := dbfx.Agent(t, "Report author", "")
	comment := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": release, "author_type": "agent", "author_id": agent, "content": "Owner's actual report"})
	edit := withURLParam(newRequest("PATCH", "/api/comments/"+comment, map[string]string{"content": "Not my report"}), "commentId", comment)
	testutil.Call(t, testHandler.UpdateComment, edit).Want(403)
}
