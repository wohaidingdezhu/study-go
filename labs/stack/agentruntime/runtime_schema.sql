-- 单行表存新任务的默认版本。已有任务记录自己的 release_id。
CREATE TABLE IF NOT EXISTS agent_active_release (
    singleton boolean PRIMARY KEY CHECK (singleton),
    release_id text NOT NULL
);

-- 切换历史同时记录候选版本的评估结果。数据库事务保证两份数据一起提交。
CREATE TABLE IF NOT EXISTS agent_release_changes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    previous_release text NOT NULL,
    next_release text NOT NULL,
    evaluation jsonb NOT NULL,
    changed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- 旧数据库和新数据库都执行这一项升级。约束存在后，启动时跳过 ALTER。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'agent_runs'::regclass AND conname = 'agent_runs_status_v2_check'
    ) THEN
        ALTER TABLE agent_runs
            DROP CONSTRAINT IF EXISTS agent_runs_status_check,
            ADD CONSTRAINT agent_runs_status_v2_check
                CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled'));
    END IF;
    -- 旧任务没有父任务，升级时该列保持 NULL。
    IF NOT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'agent_runs'::regclass AND attname = 'retry_of' AND NOT attisdropped
    ) THEN
        ALTER TABLE agent_runs ADD COLUMN retry_of bigint REFERENCES agent_runs(id);
    END IF;
END
$$;

-- 等值筛选状态后，按同一索引的 ID 范围继续翻页。
CREATE INDEX IF NOT EXISTS agent_runs_history_status_id ON agent_runs (status, id DESC);

-- 并发请求下，每个失败或已取消任务也最多有一个直接重试。
CREATE UNIQUE INDEX IF NOT EXISTS agent_runs_one_retry ON agent_runs (retry_of) WHERE retry_of IS NOT NULL;
