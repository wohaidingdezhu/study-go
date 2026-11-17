CREATE TABLE agent_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- 唯一约束让多个 API 进程共享同一份去重规则。
    idempotency_key text NOT NULL UNIQUE,
    release_id text NOT NULL,
    input text NOT NULL,
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    step integer NOT NULL DEFAULT 0 CHECK (step >= 0),
    -- JSONB 保存可恢复检查点。关系字段单独保存，用于查询与并发控制。
    state jsonb NOT NULL DEFAULT '{}',
    lease_epoch bigint NOT NULL DEFAULT 0,
    lease_until timestamptz,
    CHECK ((status = 'running') = (lease_until IS NOT NULL))
);

-- 部分索引只维护 worker 可能再次领取的任务。
CREATE INDEX agent_runs_claimable ON agent_runs (id)
WHERE status IN ('queued', 'running');

CREATE TABLE agent_events (
    run_id bigint NOT NULL REFERENCES agent_runs(id),
    seq integer NOT NULL CHECK (seq > 0),
    kind text NOT NULL,
    data jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    -- 一个任务内的序号唯一，客户端可以用它做事件游标。
    PRIMARY KEY (run_id, seq)
);
