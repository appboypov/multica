package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
)

func TestCheckRecipientTransferPreservesRecord(t *testing.T) {
	ctx := context.Background()
	f, in := checkFixture(t)
	answerCheck(t, f, openCheck(t, f, in), "pass")
	answerCheck(t, f, openCheck(t, f, in), "feedback")
	if _, err := f.svc.CheckSettings(ctx, f.workspace, f.answerer, &CheckSettings{2}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetCheckSampling(ctx, f.workspace, in.ProjectID, in.Kind, f.answerer, true); err != nil {
		t.Fatal(err)
	}
	openCheck(t, f, in)
	before := trustState(t, f, in)
	history, err := f.svc.CheckHistory(ctx, f.workspace, in.ProjectID, in.Kind)
	if err != nil {
		t.Fatal(err)
	}
	in.MemberID = f.anouk
	id := openCheck(t, f, in)
	want := before
	want.MemberID = util.UUIDToString(f.anouk)
	want.SampleOpens++
	if got := trustState(t, f, in); got != want {
		t.Fatalf("transferred record = %+v, want %+v", got, want)
	}
	after, err := f.svc.CheckHistory(ctx, f.workspace, in.ProjectID, in.Kind)
	if err != nil || !reflect.DeepEqual(after, history) {
		t.Fatalf("history changed during transfer: %+v, %v", after, err)
	}
	row, err := f.svc.GetCheck(ctx, f.workspace, id, f.other)
	if err != nil || row["done_at"] != nil {
		t.Fatalf("new recipient must receive the scheduled sample: %+v, %v", row, err)
	}
	if _, err := f.svc.AnswerCheck(ctx, f.workspace, id, f.other, "pass", ""); err != nil {
		t.Fatal(err)
	}
	want.Passed++
	if got := trustState(t, f, in); got != want {
		t.Fatalf("sample answer record = %+v, want %+v", got, want)
	}
}

func TestCheckRecipientTransferMovesSamplingControl(t *testing.T) {
	ctx := context.Background()
	f, in := checkFixture(t)
	openCheck(t, f, in)
	in.MemberID = f.anouk
	openCheck(t, f, in)
	_, err := f.svc.SetCheckSampling(ctx, f.workspace, in.ProjectID, in.Kind, f.answerer, true)
	wantQuestionRefusal(t, err, 403)
	state, err := f.svc.SetCheckSampling(ctx, f.workspace, in.ProjectID, in.Kind, f.other, true)
	if err != nil || !state.Sampling || !state.CanManage || state.MemberID != util.UUIDToString(f.anouk) {
		t.Fatalf("new recipient's sampling control: %+v, %v", state, err)
	}
}

func TestCheckRecipientTransferKeepsPendingCheckOwner(t *testing.T) {
	ctx := context.Background()
	f, in := checkFixture(t)
	pending := openCheck(t, f, in)
	in.MemberID = f.anouk
	current := openCheck(t, f, in)
	_, err := f.svc.AnswerCheck(ctx, f.workspace, pending, f.other, "pass", "")
	wantQuestionRefusal(t, err, 404)
	_, err = f.svc.AnswerCheck(ctx, f.workspace, current, f.answerer, "pass", "")
	wantQuestionRefusal(t, err, 404)
	answerCheck(t, f, pending, "feedback")
	state := trustState(t, f, in)
	if state.MemberID != util.UUIDToString(f.anouk) || state.Feedback != 1 {
		t.Fatalf("earlier answer must update the transferred record: %+v", state)
	}
}

func TestCheckRecipientTransferRejectsNonmember(t *testing.T) {
	ctx := context.Background()
	f, in := checkFixture(t)
	answerCheck(t, f, openCheck(t, f, in), "pass")
	before := trustState(t, f, in)
	in.MemberID = util.MustParseUUID(f.fx.User(t, "Outside", "outside-"+util.UUIDToString(f.workspace)+"@example.test"))
	_, err := f.svc.OpenCheck(ctx, f.workspace, f.asker, in)
	wantQuestionRefusal(t, err, 422)
	if after := trustState(t, f, in); after != before {
		t.Fatalf("refused transfer changed record: %+v, want %+v", after, before)
	}
}
