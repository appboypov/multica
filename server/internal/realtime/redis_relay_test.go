package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	redismock "github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
)

func enabledTestRetentionConfig() StreamRetentionConfig {
	cfg := DefaultStreamRetentionConfig()
	cfg.StreamTTLEnabled = true
	return cfg
}

func TestNewRedisRelayWithClientsSeparatesBlockingReadPool(t *testing.T) {
	hub := NewHub()
	writeClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	readClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		writeClient.Close()
		readClient.Close()
	})

	relay := NewRedisRelayWithClients(hub, writeClient, readClient)

	if relay.writeRDB != writeClient {
		t.Fatal("expected relay to use the write client for non-blocking Redis commands")
	}
	if relay.readRDB != readClient {
		t.Fatal("expected relay to reserve the read client for blocking XREADGROUP calls")
	}
}

func TestRedisRelayStopPreventsNewConsumers(t *testing.T) {
	hub := NewHub()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { client.Close() })

	relay := NewRedisRelay(hub, client)
	relay.Stop()
	relay.startConsumer(context.Background(), ScopeWorkspace, "workspace-1")

	relay.mu.Lock()
	consumerCount := len(relay.consumers)
	relay.mu.Unlock()
	if consumerCount != 0 {
		t.Fatalf("expected no consumers after Stop, got %d", consumerCount)
	}
	relay.Wait()
}

func TestRedisRelaySweepUsesExactTimeTrimAndRepairsTTL(t *testing.T) {
	M.Reset()
	t.Cleanup(M.Reset)
	hub := NewHub()
	attachRealtimeTestClient(hub, ScopeWorkspace, "workspace-1")
	rdb, mock := redismock.NewClientMock()
	t.Cleanup(func() { _ = rdb.Close() })
	retention := enabledTestRetentionConfig()
	retention.StreamMaxLen = 321
	retention.TrimHorizon = 20 * time.Minute
	retention.StreamTTL = 30 * time.Minute
	retention.TTLRefreshInterval = time.Minute
	retention.MaintenanceInterval = 2 * time.Minute
	relay := NewRedisRelayWithClientsAndConfig(hub, rdb, rdb, retention)
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	relay.now = func() time.Time { return now }
	relay.ttl.now = relay.now
	stream := StreamKey(ScopeWorkspace, "workspace-1")

	mock.ExpectXTrimMinID(stream, streamMinID(now, retention.TrimHorizon)).SetVal(7)
	mock.ExpectPTTL(stream).SetVal(-1)
	mock.ExpectPExpire(stream, retention.StreamTTL).SetVal(true)
	mock.ExpectZRemRangeByScore(NodesKey(ScopeWorkspace, "workspace-1"), "-inf", fmt.Sprintf("%f", float64(now.Unix()))).SetVal(1)
	mock.ExpectScan(0, "ws:scope:*:stream", legacyStreamScanCount).SetVal(nil, 0)

	relay.sweepLegacyStreams(context.Background())

	if got := M.RedisRelayStreamTrimmedTotal.Load(); got != 7 {
		t.Fatalf("trimmed total = %d, want 7", got)
	}
	if relay.retention.StreamMaxLen != 321 || relay.retention.MaintenanceInterval != 2*time.Minute {
		t.Fatalf("legacy relay did not retain shared config: %+v", relay.retention)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisRelaySweepRepairsInactiveLegacyStreamsIncrementally(t *testing.T) {
	M.Reset()
	t.Cleanup(M.Reset)
	rdb, mock := redismock.NewClientMock()
	t.Cleanup(func() { _ = rdb.Close() })
	retention := enabledTestRetentionConfig()
	relay := NewRedisRelayWithClientsAndConfig(NewHub(), rdb, rdb, retention)
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	relay.now = func() time.Time { return now }
	relay.ttl.now = relay.now
	stream := StreamKey(ScopeWorkspace, "inactive-workspace")

	mock.ExpectScan(0, "ws:scope:*:stream", legacyStreamScanCount).SetVal([]string{stream}, 42)
	mock.ExpectXTrimMinID(stream, streamMinID(now, retention.TrimHorizon)).SetVal(3)
	mock.ExpectPTTL(stream).SetVal(-1)
	mock.ExpectPExpire(stream, retention.StreamTTL).SetVal(true)

	relay.sweepLegacyStreams(context.Background())

	if relay.legacyScanCursor != 42 {
		t.Fatalf("scan cursor = %d, want 42", relay.legacyScanCursor)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisRelayCountsTTLRepairFailure(t *testing.T) {
	M.Reset()
	t.Cleanup(M.Reset)
	rdb, mock := redismock.NewClientMock()
	t.Cleanup(func() { _ = rdb.Close() })
	retention := enabledTestRetentionConfig()
	relay := NewRedisRelayWithClientsAndConfig(NewHub(), rdb, rdb, retention)
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	relay.now = func() time.Time { return now }
	relay.ttl.now = relay.now
	stream := StreamKey(ScopeWorkspace, "workspace-without-ttl")

	mock.ExpectScan(0, "ws:scope:*:stream", legacyStreamScanCount).SetVal([]string{stream}, 0)
	mock.ExpectXTrimMinID(stream, streamMinID(now, retention.TrimHorizon)).SetVal(0)
	mock.ExpectPTTL(stream).SetVal(-1)
	mock.ExpectPExpire(stream, retention.StreamTTL).SetErr(errors.New("PEXPIRE denied"))

	relay.sweepLegacyStreams(context.Background())

	if got := M.RedisRelayStreamsWithoutTTL.Load(); got != 1 {
		t.Fatalf("streams without TTL = %d, want 1", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisRelayCompleteScanPrunesMissingTTLForDeletedStreams(t *testing.T) {
	M.Reset()
	t.Cleanup(M.Reset)
	rdb, mock := redismock.NewClientMock()
	t.Cleanup(func() { _ = rdb.Close() })
	retention := enabledTestRetentionConfig()
	relay := NewRedisRelayWithClientsAndConfig(NewHub(), rdb, rdb, retention)
	relay.streamsWithoutTTL[StreamKey(ScopeWorkspace, "deleted")] = struct{}{}
	M.SetRedisStreamsWithoutTTL("legacy", 1)

	mock.ExpectScan(0, "ws:scope:*:stream", legacyStreamScanCount).SetVal(nil, 0)
	relay.sweepLegacyStreams(context.Background())

	if got := M.RedisRelayStreamsWithoutTTL.Load(); got != 0 {
		t.Fatalf("streams without TTL after full scan = %d, want 0", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisRelayEnsureConsumerGroupIgnoresBusyGroup(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	t.Cleanup(func() { _ = rdb.Close() })
	relay := NewRedisRelay(NewHub(), rdb)

	mock.ExpectXGroupCreateMkStream("stream", "group", "$").SetErr(errors.New("BUSYGROUP Consumer Group name already exists"))
	if err := relay.ensureConsumerGroup(context.Background(), "stream", "group", "$"); err != nil {
		t.Fatalf("BUSYGROUP should be ignored: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDualWriteBroadcasterFansOutLocallyBeforePublishing(t *testing.T) {
	hub := NewHub()
	client := attachRealtimeTestClient(hub, ScopeWorkspace, "workspace-1")
	publisher := &localFirstPublisher{t: t, client: client}
	broadcaster := newDualWriteBroadcaster(hub, publisher)
	message := []byte(`{"type":"issue:updated"}`)

	broadcaster.BroadcastToScope(ScopeWorkspace, "workspace-1", message)

	if !publisher.called {
		t.Fatal("expected relay publish to be invoked")
	}
	if publisher.scopeType != ScopeWorkspace || publisher.scopeID != "workspace-1" {
		t.Fatalf("unexpected relay scope: %s/%s", publisher.scopeType, publisher.scopeID)
	}
	if string(publisher.frame) != string(message) {
		t.Fatalf("expected relay payload to remain unchanged, got %s", publisher.frame)
	}

	var localFrame map[string]any
	if err := json.Unmarshal(publisher.localFrame, &localFrame); err != nil {
		t.Fatalf("local frame is not JSON: %v", err)
	}
	if localFrame["event_id"] != publisher.eventID {
		t.Fatalf("expected local frame event_id %q, got %v", publisher.eventID, localFrame["event_id"])
	}

	hub.BroadcastToScopeDedup(ScopeWorkspace, "workspace-1", injectEventID(message, publisher.eventID), publisher.eventID)
	select {
	case duplicate := <-client.send:
		t.Fatalf("expected redis loopback to dedup, got duplicate %s", duplicate)
	case <-time.After(20 * time.Millisecond):
	}
}

func attachRealtimeTestClient(hub *Hub, scopeType, scopeID string) *Client {
	client := &Client{
		send:          make(chan []byte, 2),
		workspaceID:   "workspace-1",
		userID:        "user-1",
		subscriptions: map[scopeKey]bool{},
	}
	key := sk(scopeType, scopeID)
	client.subscriptions[key] = true

	hub.mu.Lock()
	hub.clients[client] = true
	hub.rooms[key] = map[*Client]bool{client: true}
	hub.mu.Unlock()

	return client
}

type localFirstPublisher struct {
	t      *testing.T
	client *Client

	called     bool
	scopeType  string
	scopeID    string
	exclude    string
	frame      []byte
	eventID    string
	localFrame []byte
}

func (p *localFirstPublisher) PublishWithID(scopeType, scopeID, exclude string, frame []byte, id string) error {
	p.called = true
	p.scopeType = scopeType
	p.scopeID = scopeID
	p.exclude = exclude
	p.frame = append([]byte(nil), frame...)
	p.eventID = id

	select {
	case p.localFrame = <-p.client.send:
	default:
		p.t.Fatal("expected local fanout to happen before relay publish")
	}
	return nil
}

// readinessRedis keeps the consumer blocked until cancellation, allowing the
// registration/group-creation ordering to be exercised without a Redis server.
type readinessRedis struct {
	redis.UniversalClient
	createGroup func(context.Context, string, string, string) error
}

func (r *readinessRedis) XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if err := r.createGroup(ctx, stream, group, start); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal("OK")
	}
	return cmd
}

func (r *readinessRedis) ZAdd(ctx context.Context, _ string, _ ...redis.Z) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(1)
	return cmd
}

func (r *readinessRedis) XReadGroup(ctx context.Context, _ *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	<-ctx.Done()
	cmd := redis.NewXStreamSliceCmd(ctx)
	cmd.SetErr(ctx.Err())
	return cmd
}

func (r *readinessRedis) XGroupDelConsumer(ctx context.Context, _, _, _ string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(0)
	return cmd
}

func TestRedisRelayRegistrationChecksRoleAfterGroupReadiness(t *testing.T) {
	for _, demote := range []bool{false, true} {
		t.Run(fmt.Sprintf("demote=%v", demote), func(t *testing.T) {
			hub := NewHub()
			ready := make(map[string]bool)
			allowed := true
			rdb := &readinessRedis{
				createGroup: func(_ context.Context, stream, _, start string) error {
					if start != "$" {
						t.Errorf("new group replay cursor = %q, want existing live-only policy", start)
					}
					ready[stream] = true
					if demote && stream == StreamKey(ScopeWorkspace, "workspace") {
						// The demotion was published before group creation and
						// is intentionally not available through live-only replay.
						allowed = false
					}
					return nil
				},
			}
			relay := NewRedisRelay(hub, rdb)
			ctx, cancel := context.WithCancel(context.Background())
			defer func() {
				cancel()
				relay.Stop()
				relay.Wait()
			}()
			hub.SetSubscriptionCallbacks(
				func(scopeType, scopeID string) { relay.subscribeScope(ctx, scopeType, scopeID) },
				relay.stopConsumer,
			)
			checks := 0
			client := &Client{
				hub: hub, send: make(chan []byte, 1), userID: "user", workspaceID: "workspace",
				membershipChecker: membershipCheckFunc(func(context.Context, string, string) bool {
					checks++
					if !ready[StreamKey(ScopeWorkspace, "workspace")] || !ready[StreamKey(ScopeUser, "user")] {
						t.Error("final authorization ran before all subscription groups were ready")
					}
					return allowed
				}),
			}
			t.Cleanup(func() { hub.removeClient(client) })
			if got := hub.registerClient(client); got != !demote {
				t.Fatalf("registration allowed = %v, want %v", got, !demote)
			}
			if checks != 1 {
				t.Fatalf("final authorization checks = %d, want 1", checks)
			}
			if demote && (totalClients(hub) != 0 || len(hub.LocalScopes()) != 0) {
				t.Fatal("demoted registration retained live subscriptions")
			}
		})
	}
}

func TestRedisRelaySubscriptionReadinessFailureRefusesRegistration(t *testing.T) {
	for _, createErr := range []error{errors.New("redis unavailable"), context.Canceled, context.DeadlineExceeded} {
		t.Run(createErr.Error(), func(t *testing.T) {
			hub := NewHub()
			rdb := &readinessRedis{
				createGroup: func(context.Context, string, string, string) error { return createErr },
			}
			relay := NewRedisRelay(hub, rdb)
			hub.SetSubscriptionCallbacks(
				func(scopeType, scopeID string) { relay.subscribeScope(context.Background(), scopeType, scopeID) },
				relay.stopConsumer,
			)
			client := &Client{
				hub: hub, send: make(chan []byte, 1), userID: "user", workspaceID: "workspace",
				membershipChecker: &mockMembershipChecker{},
			}
			if hub.registerClient(client) {
				t.Fatal("registration succeeded without a ready relay group")
			}
			if raw, ok := <-client.send; ok {
				t.Fatalf("failed registration received a frame: %s", raw)
			}
			if totalClients(hub) != 0 || len(hub.LocalScopes()) != 0 || len(relay.consumers) != 0 {
				t.Fatal("failed subscription preparation retained a client, room, or consumer")
			}
			relay.Wait()
		})
	}
}
