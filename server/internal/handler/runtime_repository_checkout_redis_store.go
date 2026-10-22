package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/redis/go-redis/v9"
)

const repositoryCheckoutRedisPrefix = "mul:" + runtimePendingRedisHashTag + ":repository_checkout:"

func repositoryCheckoutRedisKey(id string) string { return repositoryCheckoutRedisPrefix + id }
func repositoryCheckoutPendingKey(runtimeID string) string {
	return repositoryCheckoutRedisPrefix + "pending:" + runtimeID
}

// Keep the execution clock in storage without adding it to the public response.
type storedRepositoryCheckout struct {
	*RuntimeRepositoryCheckoutRequest
	RunStartedAt *time.Time `json:"run_started_at,omitempty"`
}

func marshalStoredRepositoryCheckout(req *RuntimeRepositoryCheckoutRequest) ([]byte, error) {
	return json.Marshal(storedRepositoryCheckout{req, req.RunStartedAt})
}

// Compare the exact record before changing its state. Claiming, timing out and
// receiving a late daemon result cannot overwrite one another on another node.
var repositoryCheckoutTransitionScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
redis.call('SET', KEYS[1], ARGV[2], 'EX', tonumber(ARGV[3]))
redis.call('ZREM', KEYS[2], ARGV[4])
return 1
`)

type RedisRepositoryCheckoutStore struct{ rdb redis.UniversalClient }

func NewRedisRepositoryCheckoutStore(rdb redis.UniversalClient) *RedisRepositoryCheckoutStore {
	return &RedisRepositoryCheckoutStore{rdb: rdb}
}

func (s *RedisRepositoryCheckoutStore) Create(ctx context.Context, in RuntimeRepositoryCheckoutRequest) (*RuntimeRepositoryCheckoutRequest, error) {
	now := time.Now()
	req := &RuntimeRepositoryCheckoutRequest{
		ID: randomID(), RuntimeID: in.RuntimeID, DaemonID: in.DaemonID,
		WorkspaceID: in.WorkspaceID, FullName: in.FullName, DefaultBranch: in.DefaultBranch,
		Path: in.Path, Clone: in.Clone, RepositoryID: in.RepositoryID, Relocation: in.Relocation, Status: RuntimeLocalSkillPending,
		CreatedAt: now, UpdatedAt: now,
	}
	data, err := marshalStoredRepositoryCheckout(req)
	if err != nil {
		return nil, err
	}
	key, pending := repositoryCheckoutRedisKey(req.ID), repositoryCheckoutPendingKey(req.RuntimeID)
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, key, data, repositoryCheckoutStoreRetention)
	pipe.ZAdd(ctx, pending, redis.Z{Score: float64(now.UnixNano()), Member: req.ID})
	pipe.Expire(ctx, pending, repositoryCheckoutStoreRetention*2)
	if _, err := pipe.Exec(ctx); err != nil {
		_ = s.rdb.Del(ctx, key).Err()
		_ = s.rdb.ZRem(ctx, pending, req.ID).Err()
		return nil, fmt.Errorf("persist repository checkout: %w", err)
	}
	return req, nil
}

func (s *RedisRepositoryCheckoutStore) load(ctx context.Context, id string) (*RuntimeRepositoryCheckoutRequest, string, error) {
	raw, err := s.rdb.Get(ctx, repositoryCheckoutRedisKey(id)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("get repository checkout: %w", err)
	}
	var stored storedRepositoryCheckout
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, "", fmt.Errorf("decode repository checkout: %w", err)
	}
	if stored.RuntimeRepositoryCheckoutRequest == nil {
		return nil, "", errors.New("empty stored repository checkout")
	}
	stored.RuntimeRepositoryCheckoutRequest.RunStartedAt = stored.RunStartedAt
	return stored.RuntimeRepositoryCheckoutRequest, raw, nil
}

func (s *RedisRepositoryCheckoutStore) transition(ctx context.Context, req *RuntimeRepositoryCheckoutRequest, raw string) (bool, error) {
	data, err := marshalStoredRepositoryCheckout(req)
	if err != nil {
		return false, err
	}
	changed, err := repositoryCheckoutTransitionScript.Run(ctx, s.rdb, []string{
		repositoryCheckoutRedisKey(req.ID), repositoryCheckoutPendingKey(req.RuntimeID),
	}, raw, string(data), int(repositoryCheckoutStoreRetention.Seconds()), req.ID).Int()
	if err != nil {
		return false, fmt.Errorf("transition repository checkout: %w", err)
	}
	return changed == 1, nil
}

func (s *RedisRepositoryCheckoutStore) Get(ctx context.Context, id string) (*RuntimeRepositoryCheckoutRequest, error) {
	for range localSkillRedisPopMaxRetries {
		req, raw, err := s.load(ctx, id)
		if err != nil || req == nil {
			return req, err
		}
		before := req.Status
		applyRepositoryCheckoutTimeout(req, time.Now())
		if before == req.Status {
			return req, nil
		}
		changed, err := s.transition(ctx, req, raw)
		if err != nil {
			return nil, err
		}
		if changed {
			return req, nil
		}
	}
	return nil, errors.New("repository checkout changed while reading its timeout")
}

func (s *RedisRepositoryCheckoutStore) HasPending(ctx context.Context, runtimeID string) (bool, error) {
	count, err := s.rdb.ZCard(ctx, repositoryCheckoutPendingKey(runtimeID)).Result()
	return count > 0, err
}

func (s *RedisRepositoryCheckoutStore) PopPending(ctx context.Context, runtimeID string) (*RuntimeRepositoryCheckoutRequest, error) {
	pending := repositoryCheckoutPendingKey(runtimeID)
	for range localSkillRedisPopMaxRetries {
		ids, err := s.rdb.ZRange(ctx, pending, 0, 0).Result()
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, nil
		}
		req, raw, err := s.load(ctx, ids[0])
		if err != nil {
			return nil, err
		}
		if req == nil || req.Status != RuntimeLocalSkillPending {
			if err := s.rdb.ZRem(ctx, pending, ids[0]).Err(); err != nil {
				return nil, err
			}
			continue
		}
		now := time.Now()
		applyRepositoryCheckoutTimeout(req, now)
		if req.Status == RuntimeLocalSkillPending {
			req.Status, req.RunStartedAt, req.UpdatedAt = RuntimeLocalSkillRunning, &now, now
		}
		changed, err := s.transition(ctx, req, raw)
		if err != nil {
			return nil, err
		}
		if changed && req.Status == RuntimeLocalSkillRunning {
			return req, nil
		}
	}
	return nil, nil
}

func (s *RedisRepositoryCheckoutStore) Complete(ctx context.Context, id string, result protocol.RepositoryCheckoutResult) error {
	if result.Failures == nil {
		result.Failures = []protocol.RepositoryCheckoutFailure{}
	}
	if result.VersionFiles == nil {
		result.VersionFiles = []string{}
	}
	if result.InstallCommands == nil {
		result.InstallCommands = []protocol.RepositoryInstallCommand{}
	}
	return s.finish(ctx, id, &result, "")
}

func (s *RedisRepositoryCheckoutStore) Fail(ctx context.Context, id, message string) error {
	return s.finish(ctx, id, nil, message)
}

func (s *RedisRepositoryCheckoutStore) finish(ctx context.Context, id string, result *protocol.RepositoryCheckoutResult, message string) error {
	for range localSkillRedisPopMaxRetries {
		req, raw, err := s.load(ctx, id)
		if err != nil || req == nil {
			return err
		}
		if runtimeLocalSkillRequestTerminal(req.Status) {
			return nil
		}
		now := time.Now()
		applyRepositoryCheckoutTimeout(req, now)
		if !runtimeLocalSkillRequestTerminal(req.Status) {
			req.Status, req.Error, req.UpdatedAt = RuntimeLocalSkillFailed, message, now
			if result != nil {
				req.Status, req.Result = RuntimeLocalSkillCompleted, result
			}
		}
		changed, err := s.transition(ctx, req, raw)
		if err != nil {
			return err
		}
		if changed {
			return nil
		}
	}
	return errors.New("repository checkout changed while completing its result")
}
