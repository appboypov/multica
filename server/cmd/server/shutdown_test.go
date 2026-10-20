package main

import (
	"slices"
	"testing"
)

// The order teardown runs in is a contract; this pins it against the same type
// main() runs.
func TestShutdownSequence_RunsInTheDocumentedOrder(t *testing.T) {
	var ran []string
	record := func(name string) func() {
		return func() { ran = append(ran, name) }
	}
	shutdownSequence{
		StopAutopilot:     record("autopilot"),
		DrainMaintenance:  record("maintenance"),
		DrainHTTP:         record("http"),
		CancelWorkers:     record("cancel"),
		StopHeartbeats:    record("heartbeats"),
		JoinWebhookWorker: record("webhooks"),
		StopMetricsServer: record("metrics"),
		StopProfiling:     record("pprof"),
	}.run()

	want := []string{
		"autopilot", "maintenance", "http", "cancel", "heartbeats",
		"webhooks", "metrics", "pprof",
	}
	if !slices.Equal(ran, want) {
		t.Fatalf("shutdown order = %v, want %v", ran, want)
	}
}

// A deployment without a metrics server leaves that step nil. Skipping it must
// not skip the rest.
func TestShutdownSequence_SkipsTheStepsADeploymentDoesNotHave(t *testing.T) {
	var ran []string
	shutdownSequence{
		DrainHTTP:     func() { ran = append(ran, "http") },
		CancelWorkers: func() { ran = append(ran, "cancel") },
		StopProfiling: func() { ran = append(ran, "pprof") },
	}.run()

	if want := []string{"http", "cancel", "pprof"}; !slices.Equal(ran, want) {
		t.Fatalf("shutdown order = %v, want %v", ran, want)
	}
}
