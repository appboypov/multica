-- Release coordination source capture (release-owner).
--
-- Source facts are captured in their write transaction, so a rollback leaves
-- no receipt and a commit cannot lose one. Release facts land on the release's
-- issue_wakeup rows and use the existing receipt table, its uniqueness and its
-- expiry. Issue capture paths are unchanged: every issue branch below keeps
-- its previous body.

-- Thread roots stay inside one subject: an issue reply walks issue comments,
-- a release reply walks release comments.
CREATE OR REPLACE FUNCTION comment_thread_root_id(comment_id uuid) RETURNS uuid
LANGUAGE sql STABLE STRICT AS $$
    WITH RECURSIVE ancestors AS (
        SELECT c.id, c.parent_id, c.issue_id, c.release_id, c.workspace_id, ARRAY[c.id] AS path
        FROM comment c WHERE c.id = comment_id
        UNION ALL
        SELECT p.id, p.parent_id, p.issue_id, p.release_id, p.workspace_id, a.path || p.id
        FROM ancestors a JOIN comment p ON p.id = a.parent_id
            AND p.issue_id IS NOT DISTINCT FROM a.issue_id
            AND p.release_id IS NOT DISTINCT FROM a.release_id
            AND p.workspace_id = a.workspace_id
        WHERE NOT p.id = ANY(a.path)
    )
    SELECT COALESCE(
        (SELECT id FROM ancestors ORDER BY cardinality(path) DESC LIMIT 1),
        comment_id
    )
$$;

-- People's and agents' waits count against the same per-subject (32) and
-- per-workspace (1000) limits whether they wait on an issue or a release.
-- Platform rules stay outside the capacity.
CREATE OR REPLACE FUNCTION guard_issue_wakeup_capacity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT NEW.enabled OR NEW.system_rule IS NOT NULL THEN RETURN NEW; END IF;
 IF TG_OP='UPDATE' AND OLD.enabled AND OLD.issue_id IS NOT DISTINCT FROM NEW.issue_id
  AND OLD.release_id IS NOT DISTINCT FROM NEW.release_id AND OLD.workspace_id=NEW.workspace_id THEN RETURN NEW; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('issue-wakeup-capacity:'||NEW.workspace_id::text,0));
 IF NEW.issue_id IS NOT NULL AND (SELECT count(*) FROM (SELECT 1 FROM issue_wakeup WHERE workspace_id=NEW.workspace_id AND issue_id=NEW.issue_id AND enabled AND system_rule IS NULL AND id<>NEW.id LIMIT 32) slots)>=32 THEN
  RAISE EXCEPTION 'An issue can have at most 32 enabled wakeups' USING ERRCODE='23514',CONSTRAINT='issue_wakeup_active_limit';
 END IF;
 IF NEW.release_id IS NOT NULL AND (SELECT count(*) FROM (SELECT 1 FROM issue_wakeup WHERE workspace_id=NEW.workspace_id AND release_id=NEW.release_id AND enabled AND system_rule IS NULL AND id<>NEW.id LIMIT 32) slots)>=32 THEN
  RAISE EXCEPTION 'A release can have at most 32 enabled wakeups' USING ERRCODE='23514',CONSTRAINT='issue_wakeup_active_limit';
 END IF;
 IF (SELECT count(*) FROM (SELECT 1 FROM issue_wakeup WHERE workspace_id=NEW.workspace_id AND enabled AND system_rule IS NULL AND id<>NEW.id LIMIT 1000) slots)>=1000 THEN
  RAISE EXCEPTION 'A workspace can have at most 1000 enabled wakeups' USING ERRCODE='23514',CONSTRAINT='issue_wakeup_active_limit';
 END IF;
 RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS guard_issue_wakeup_capacity ON issue_wakeup;
CREATE TRIGGER guard_issue_wakeup_capacity BEFORE INSERT OR UPDATE OF enabled, issue_id, release_id, workspace_id ON issue_wakeup
 FOR EACH ROW EXECUTE FUNCTION guard_issue_wakeup_capacity();

-- Record one release fact on the release's matching event rules. Only an
-- owned, unshipped release captures: a release without an owner, a shipped
-- release and a removed one record nothing. The rule is read FOR KEY SHARE and
-- the repository is never locked here, so an issue status writer (issue row
-- held) cannot invert shipping's repository -> issue -> wakeup order; shipping
-- deletes the rule under that order and a capture either lands before it or
-- finds the rule gone.
--
-- p_coalesce names the pending receipt a repeated fact folds into (owner
-- changes). Member comments and issue endings pass NULL: every one is its own
-- receipt, so no comment or ending is lost before the next run reads them.
CREATE OR REPLACE FUNCTION capture_release_wakeup(p_release uuid, p_type text, p_key text, p_agent uuid, p_task uuid, p_payload jsonb, p_coalesce text)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE w issue_wakeup; owner_workspace uuid; evidence jsonb; violated_constraint text;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM issue_wakeup WHERE release_id=p_release AND enabled AND kind='event' AND p_type=ANY(event_types)) THEN RETURN; END IF;
 IF p_agent IS NULL AND current_setting('multica.actor_type',true)='agent' THEN
  p_agent := NULLIF(current_setting('multica.actor_id',true),'')::uuid;
 END IF;
 SELECT r.workspace_id INTO owner_workspace FROM release r
 WHERE r.id=p_release AND r.shipped_at IS NULL AND r.owner_type IS NOT NULL AND r.owner_id IS NOT NULL;
 IF owner_workspace IS NULL THEN RETURN; END IF;
 evidence := jsonb_build_object('event_id',p_key,'event_type',p_type,'version',1,
  'occurred_at',clock_timestamp(),'workspace_id',owner_workspace,'release_id',p_release,
  'source_task_id',p_task,'agent_id',p_agent,
  'actor_type',COALESCE(NULLIF(current_setting('multica.actor_type',true),''),CASE WHEN p_agent IS NOT NULL THEN 'agent' ELSE 'system' END),
  'actor_id',COALESCE(NULLIF(current_setting('multica.actor_id',true),''),p_agent::text)) || p_payload;
 FOR w IN SELECT * FROM issue_wakeup WHERE release_id=p_release AND workspace_id=owner_workspace AND enabled AND kind='event'
   AND p_type=ANY(event_types)
   AND (filter_actor_type IS NULL OR (filter_actor_type=evidence->>'actor_type' AND filter_actor_id::text=evidence->>'actor_id'))
   AND (filter_agent_id IS NULL OR filter_agent_id=p_agent)
   AND (filter_task_id IS NULL OR filter_task_id=p_task)
   ORDER BY id
   FOR KEY SHARE
 LOOP
  BEGIN
   INSERT INTO issue_wakeup_receipt(id,wakeup_id,revision,event_key,event_type,payload,coalesce_key)
    VALUES(gen_random_uuid(),w.id,w.revision,p_key,p_type,
     evidence || jsonb_build_object('coalesced_count',1,'first_occurred_at',evidence->'occurred_at'),p_coalesce)
   ON CONFLICT(wakeup_id,revision,coalesce_key) WHERE processed_at IS NULL AND coalesce_key IS NOT NULL
   DO UPDATE SET id=EXCLUDED.id,payload=EXCLUDED.payload || jsonb_build_object(
    'coalesced_count',COALESCE((issue_wakeup_receipt.payload->>'coalesced_count')::bigint,1)+1,
    'first_occurred_at',issue_wakeup_receipt.payload->'first_occurred_at')
   WHERE issue_wakeup_receipt.event_key<>p_key AND issue_wakeup_receipt.payload->>'event_id' IS DISTINCT FROM p_key;
  EXCEPTION WHEN unique_violation THEN
   GET STACKED DIAGNOSTICS violated_constraint = CONSTRAINT_NAME;
   -- A retained receipt already recorded this exact source fact.
   IF violated_constraint <> 'issue_wakeup_receipt_key_idx' THEN RAISE; END IF;
  END;
 END LOOP;
END $$;

-- Comments: the issue branch is unchanged. A release comment wakes its owner
-- only when a member writes it (a root or a reply); agent and system result
-- comments never do, so an owner report cannot wake the owner again.
CREATE OR REPLACE FUNCTION capture_comment_wakeup() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE c comment; event_type text; source_id uuid; source_agent uuid; payload jsonb; root_id uuid;
BEGIN
 c := CASE WHEN TG_OP='DELETE' THEN OLD ELSE NEW END;
 IF c.release_id IS NOT NULL THEN
  IF TG_OP<>'INSERT' OR c.author_type<>'member' THEN RETURN c; END IF;
  root_id := comment_thread_root_id(c.id);
  PERFORM capture_release_wakeup(c.release_id,'release.comment.created','comment:'||c.id::text,NULL,NULL,
   jsonb_build_object('comment_id',c.id,'parent_comment_id',c.parent_id,'thread_id',COALESCE(root_id,c.id),
    'author_type',c.author_type,'author_id',c.author_id,'actor_type','member','actor_id',c.author_id),NULL);
  RETURN c;
 END IF;
 IF NOT EXISTS (SELECT 1 FROM issue_wakeup WHERE issue_id=c.issue_id AND enabled AND kind='event') THEN RETURN c; END IF;
 source_id := NULLIF(current_setting('multica.source_task_id',true),'')::uuid;
 IF TG_OP='INSERT' THEN
  event_type := 'comment.created';
  source_id := COALESCE(source_id,c.source_task_id);
  source_agent := CASE WHEN c.author_type='agent' THEN c.author_id END;
 ELSIF TG_OP='DELETE' THEN
  -- Pruning an existing tombstone is storage cleanup, not another deletion.
  IF OLD.deleted_at IS NOT NULL THEN RETURN OLD; END IF;
  event_type := 'comment.deleted';
 ELSIF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
  event_type := 'comment.deleted';
 ELSIF NEW.deleted_at IS NOT NULL THEN RETURN NEW;
 ELSIF OLD.resolved_at IS NULL AND NEW.resolved_at IS NOT NULL THEN event_type := 'comment.resolved';
 ELSIF OLD.resolved_at IS NOT NULL AND NEW.resolved_at IS NULL THEN event_type := 'comment.unresolved';
 ELSIF NEW.content IS DISTINCT FROM OLD.content THEN event_type := 'comment.updated';
 ELSE RETURN NEW;
 END IF;
 IF source_id IS NOT NULL THEN SELECT agent_id INTO source_agent FROM agent_task_queue WHERE id=source_id; END IF;
 -- Keep author and actor distinct: an admin editing an agent's comment is
 -- not an event produced by that agent's original run.
 IF TG_OP<>'INSERT' AND source_id IS NULL THEN
  source_agent := CASE WHEN current_setting('multica.actor_type',true)='agent' THEN NULLIF(current_setting('multica.actor_id',true),'')::uuid END;
 END IF;
 root_id := c.id;
 IF c.parent_id IS NOT NULL THEN
  WITH RECURSIVE ancestors AS (
   SELECT id,parent_id,1 AS depth FROM comment WHERE id=c.parent_id AND issue_id=c.issue_id AND workspace_id=c.workspace_id
   UNION ALL SELECT p.id,p.parent_id,a.depth+1 FROM comment p JOIN ancestors a ON p.id=a.parent_id
    WHERE p.issue_id=c.issue_id AND p.workspace_id=c.workspace_id AND a.depth<256
  ) SELECT id INTO root_id FROM ancestors ORDER BY depth DESC LIMIT 1;
 END IF;
 payload := jsonb_build_object('comment_id',c.id,'parent_comment_id',c.parent_id,'thread_id',COALESCE(root_id,c.id),
  'author_type',c.author_type,'author_id',c.author_id);
 IF TG_OP='INSERT' THEN payload := payload || jsonb_build_object('actor_type',c.author_type,'actor_id',c.author_id); END IF;
 PERFORM capture_issue_wakeup(c.issue_id,event_type,gen_random_uuid()::text,source_agent,source_id,payload);
 RETURN c;
END $$;

-- Every real terminal transition of an issue in an owned release, by any
-- writer (single or batch update, automatic PR completion, task recovery),
-- with built-in and custom done/closed statuses. done -> cancelled is a new
-- ending; cancelled is terminal but not successful. Keys are issue id plus the
-- revision the status write produced, so one transition records once. The
-- release owner's own transitions are captured too: this is not the parent
-- stage barrier and has no self-acknowledgement suppression.
CREATE OR REPLACE FUNCTION capture_release_issue_wakeup() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE old_effective text; new_effective text; source_id uuid; source_agent uuid;
BEGIN
 new_effective := issue_effective_status(NEW.workspace_id, NEW.status);
 IF new_effective NOT IN ('done','cancelled') THEN RETURN NULL; END IF;
 old_effective := issue_effective_status(OLD.workspace_id, OLD.status);
 IF old_effective = new_effective THEN RETURN NULL; END IF;
 source_id := NULLIF(current_setting('multica.source_task_id',true),'')::uuid;
 IF source_id IS NOT NULL THEN SELECT agent_id INTO source_agent FROM agent_task_queue WHERE id=source_id; END IF;
 PERFORM capture_release_wakeup(NEW.release_id,'release.issue.ended',
  'issue:'||NEW.id::text||':'||NEW.revision::text||':'||new_effective,source_agent,source_id,
  jsonb_build_object('issue_id',NEW.id,'status',NEW.status,'previous_status',OLD.status,
   'outcome',new_effective,'previous_outcome',old_effective,'issue_revision',NEW.revision),NULL);
 RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS capture_release_issue_wakeup ON issue;
CREATE TRIGGER capture_release_issue_wakeup AFTER UPDATE OF status ON issue FOR EACH ROW
 WHEN (NEW.release_id IS NOT NULL AND OLD.status IS DISTINCT FROM NEW.status)
 EXECUTE FUNCTION capture_release_issue_wakeup();
