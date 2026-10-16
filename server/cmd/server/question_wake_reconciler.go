package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
)

const (
	questionWakeSweepInterval        = 30 * time.Second
	questionWakeSweepTimeout         = 25 * time.Second
	questionWakeSweepBatchSize int32 = 100
)

func runQuestionWakeReconciler(ctx context.Context, questions *service.QuestionService) {
	sweep := func() {
		sweepCtx, cancel := context.WithTimeout(ctx, questionWakeSweepTimeout)
		defer cancel()
		if err := questions.ReconcilePendingQuestionWakes(sweepCtx, questionWakeSweepBatchSize); err != nil {
			slog.Warn("question wake reconciler: sweep failed", "error", err)
		}
	}
	sweep()
	runPeriodicSweep(ctx, questionWakeSweepInterval, sweep)
}
