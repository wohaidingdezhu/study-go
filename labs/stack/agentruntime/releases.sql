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
