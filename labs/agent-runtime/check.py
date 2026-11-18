import argparse
import json
import os
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime
from pathlib import Path
import subprocess
import time
from urllib.error import HTTPError
from urllib.parse import urlencode
from urllib.request import Request, urlopen
from uuid import uuid4


COMPOSE_FILE = Path(__file__).resolve().with_name("compose.yaml")
EVENTS = ["run_created", "tool_requested", "tool_completed", "run_completed"]


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def docker(*args, check=True, expected=0, timeout=30):
    # 端口 0 让 Docker 分配空闲端口，隔离并行验收和常驻演示。
    result = subprocess.run(
        ["docker", *args], capture_output=True, text=True, check=False, timeout=timeout,
        env={**os.environ, "AGENT_API_PORT": "0"},
    )
    if check and result.returncode != expected:
        raise RuntimeError(f"docker {' '.join(args)}\nexit={result.returncode}\n{result.stdout}{result.stderr}")
    return result.stdout.strip()


def until(description, probe, seconds=20):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        value = probe()
        if value:
            return value
        time.sleep(0.15)
    raise TimeoutError(f"超时：{description}")


def database_time(value):
    # Go 会省略小数末尾的零，Python 3.9 的 fromisoformat 不接受所有这些长度。
    pattern = "%Y-%m-%dT%H:%M:%S.%f%z" if "." in value else "%Y-%m-%dT%H:%M:%S%z"
    return datetime.strptime(value, pattern)


class Verification:
    def __init__(self):
        self.project = f"study-go-agent-check-{uuid4().hex[:8]}"
        self.slow_workers = []

    def compose(self, *args, **options):
        return docker("compose", "-p", self.project, "-f", str(COMPOSE_FILE), *args, **options)

    def request(self, path, expected=200, *, body=None, key=None, method=None):
        headers = {"Content-Type": "application/json", "Idempotency-Key": key} if key else {}
        payload = json.dumps(body).encode() if body is not None else None
        request = Request(self.api + path, data=payload, headers=headers, method=method)
        try:
            response = urlopen(request, timeout=4)
        except HTTPError as error:
            response = error
        with response:
            result = json.load(response)
            require(response.status == expected, f"{path}: HTTP {response.status}, {result}")
            return result, response.headers.get("Location")

    def submit(self, key, question):
        run, path = self.request("/runs", 202, body={"input": question}, key=key)
        require(path == f"/runs/{run['id']}", f"Location 异常：{path}")
        return run["id"], path

    def succeeded(self, path, *citations):
        def finished():
            result, _ = self.request(path)
            require(result["status"] != "failed", f"任务失败：{result}")
            return result if result["status"] == "succeeded" else None

        run = until("任务完成", finished)
        require(run["state"]["answer"]["citations"] == list(citations), f"引用异常：{run}")
        events, _ = self.request(path + "/events")
        require([event["kind"] for event in events] == EVENTS, f"事件异常：{events}")
        require([event["seq"] for event in events] == [1, 2, 3, 4], f"序号异常：{events}")
        evidence_ids = [item["id"] for item in events[2]["data"]["evidence"]]
        require(evidence_ids == list(citations), f"工具结果异常：{events[2]}")
        return run, events

    def release_command(self, *args, **options):
        return json.loads(self.compose("run", "--rm", "-T", "--no-deps", "worker", *args, **options))

    def active_release(self):
        releases, _ = self.request("/releases")
        active = [release["id"] for release in releases if release["active"]]
        require(len(active) == 1, f"默认版本异常：{releases}")
        return active[0]

    def check_overview(self):
        empty, _ = self.request("/runtime/overview")
        require(empty["queue"] == {"queued": 0, "running": 0, "expiredLeases": 0, "oldestQueuedAt": None},
                f"空库队列统计异常：{empty}")
        require(empty["recent"] == {"hours": 24, "succeeded": 0, "failed": 0, "cancelled": 0,
                                    "durationSamples": 0, "p50Ms": None, "p95Ms": None}, f"空样本统计异常：{empty}")
        self.compose("stop", "worker")
        psql = ("exec", "-T", "postgres", "psql", "-U", "agent", "-d", "agent_runtime", "-v", "ON_ERROR_STOP=1", "-c")
        # 使用数据库自己的时钟布置边界及固定耗时，避免把网络耗时混进分位数断言。
        self.compose(*psql, """
            WITH fixtures(label, status, age, lease, kind, duration) AS (VALUES
                ('oldest', 'queued', interval '2 days', NULL::interval, NULL::text, interval '0'),
                ('next', 'queued', interval '1 hour', NULL, NULL, interval '0'),
                ('live', 'running', interval '2 hours', interval '1 hour', NULL, interval '0'),
                ('expired', 'running', interval '2 hours', interval '-1 hour', NULL, interval '0'),
                ('fast', 'succeeded', interval '1 hour', NULL, 'run_completed', interval '100 milliseconds'),
                ('slow', 'succeeded', interval '1 hour', NULL, 'run_completed', interval '300 milliseconds'),
                ('outside', 'succeeded', interval '25 hours', NULL, 'run_completed', interval '2 seconds'),
                ('failed', 'failed', interval '1 hour', NULL, 'run_failed', interval '2 seconds'),
                ('denied', 'failed', interval '1 hour', NULL, 'tool_denied', interval '1 second'),
                ('cancelled', 'cancelled', interval '1 hour', NULL, 'run_cancelled', interval '1 second')
            ), inserted AS (
                INSERT INTO agent_runs (idempotency_key, release_id, input, status, step, lease_until)
                SELECT 'overview-test-' || label, 'ops-runbook-v1', label, status,
                       CASE WHEN kind IS NULL THEN 0 ELSE 1 END, statement_timestamp() + lease
                FROM fixtures RETURNING id, input
            )
            INSERT INTO agent_events (run_id, seq, kind, data, created_at)
            SELECT id, 1, 'run_created', '{}'::jsonb, statement_timestamp() - age
                FROM inserted JOIN fixtures ON input=label
            UNION ALL
            SELECT id, 2, kind, '{}'::jsonb, statement_timestamp() - age + duration
                FROM inserted JOIN fixtures ON input=label WHERE kind IS NOT NULL
        """)
        view, _ = self.request("/runtime/overview")
        pending, _ = self.request("/runs?status=queued")
        oldest = next(row["createdAt"] for row in pending["items"] if row["input"] == "oldest")
        expected_queue = {"queued": 2, "running": 2, "expiredLeases": 1, "oldestQueuedAt": oldest}
        require(view["queue"] == expected_queue, f"待领取或租约统计错误：{view}")
        expected_recent = {"hours": 24, "succeeded": 2, "failed": 2, "cancelled": 1,
                           "durationSamples": 2, "p50Ms": 200, "p95Ms": 290}
        require(view["recent"] == expected_recent, f"24 小时终态或完成耗时统计错误：{view}")
        before, after = (database_time(sample["asOf"]) for sample in (empty, view))
        require(after >= before, f"统计时间异常：{view}")
        self.compose(*psql, """
            BEGIN;
            DELETE FROM agent_events WHERE run_id IN (
                SELECT id FROM agent_runs WHERE idempotency_key LIKE 'overview-test-%'
            );
            DELETE FROM agent_runs WHERE idempotency_key LIKE 'overview-test-%';
            COMMIT;
        """)
        self.compose("start", "worker")
        print("通过：空概览、过期租约、24 小时窗口、拒绝计入失败和分位数", flush=True)

    def check_releases(self):
        baseline, candidate, regression = "ops-runbook-v1", "ops-runbook-v2", "ops-runbook-regression"
        self.compose("stop", "worker")
        question = "Redis、PostgreSQL 和 RabbitMQ 如何排查？"
        old_id, old_task = self.submit("before-release", question)
        report = self.release_command("publish", baseline, regression, expected=1)
        require(not report["passed"] and self.active_release() == baseline, f"评估门禁未拦截：{report}")
        self.request("/runs", 404, body={"input": question, "release": regression}, key="denied-release")

        report = self.release_command("publish", baseline, candidate)
        require(report["passed"] and self.active_release() == candidate, f"候选发布失败：{report}")
        self.release_command("publish", baseline, baseline, expected=1)
        require(self.active_release() == candidate, "过期的发布覆盖了默认版本")
        replay, _ = self.request("/runs", body={"input": question}, key="before-release")
        require(replay["id"] == old_id and replay["release"] == baseline, f"切换后重试没有返回原任务：{replay}")
        _, new_task = self.submit("after-release", question)

        # 两个任务都还未执行。回滚后才启动 worker，具体验证每个任务按自身记录选择版本。
        self.release_command("publish", candidate, baseline)
        require(self.active_release() == baseline, "默认版本回滚失败")
        self.compose("start", "worker")
        first, _ = self.succeeded(old_task, "cache-fallback", "database-readiness")
        second, _ = self.succeeded(new_task, "cache-fallback", "database-readiness", "outbox-backlog")
        require(first["release"] == baseline and second["release"] == candidate, "两个任务的版本不正确")
        changes = self.compose("exec", "-T", "postgres", "psql", "-U", "agent", "-d", "agent_runtime", "-Atc", """
            SELECT previous_release || '>' || next_release || ':' || (evaluation->>'dataset')
            FROM agent_release_changes ORDER BY id
        """)
        expected = [f"{baseline}>{candidate}:runbook-smoke-v1", f"{candidate}>{baseline}:runbook-smoke-v1"]
        require(changes.splitlines() == expected, f"发布审计异常：{changes}")
        print("通过：拒绝回归候选、发布与回滚、固定任务版本、跨版本重试和审计", flush=True)

    def cancelled(self, path, expected_events):
        run, _ = self.request(path)
        events, _ = self.request(path + "/events")
        require(run["status"] == "cancelled" and "leaseUntil" not in run, f"取消状态或租约异常：{run}")
        require("pending" not in run["state"] and "answer" not in run["state"], f"取消后任务状态异常：{run}")
        require([event["kind"] for event in events] == expected_events, f"取消事件异常：{events}")
        require([event["seq"] for event in events] == list(range(1, len(events) + 1)), f"取消事件序号异常：{events}")
        require(events[-1]["data"] == {"reason": "user_requested"}, f"取消原因异常：{events}")
        return run

    def check_cancellation(self):
        self.compose("stop", "worker")
        question = "Redis 缓存下线怎么办？"
        _, queued_path = self.submit("cancel-queued", question)

        # 两个客户端同时取消同一条排队任务，应该只写一条取消事件。
        with ThreadPoolExecutor(max_workers=2) as pool:
            first = pool.submit(self.request, queued_path + "/cancel", method="POST")
            second = pool.submit(self.request, queued_path + "/cancel", method="POST")
            require(first.result() == second.result(), "并发取消返回了不同的终态")
        queued = self.cancelled(queued_path, ["run_created", "run_cancelled"])
        require(queued["leaseEpoch"] == 0, f"排队时取消的任务已经被领取：{queued}")
        replay, _ = self.request("/runs", body={"input": question}, key="cancel-queued")
        require(replay == queued, "重发创建请求意外重启了已取消的任务")

        name = f"{self.project}-cancel"
        self.slow_workers.append(name)
        self.compose("run", "-d", "--no-deps", "--name", name, "-e", "AGENT_CHECKPOINT_DELAY=4s", "worker")
        run_id, path = self.submit("cancel-running", question)
        in_flight = self.wait_log(name, msg="step_computed", run_id=run_id, kind="tool_completed")
        aborted, _ = self.request(path + "/cancel", method="POST")
        # 等执行中的 worker 尝试写回已计算的结果，验证取消不会被旧租约覆盖。
        rejection = self.wait_log(name, msg="checkpoint_rejected", run_id=run_id, epoch=in_flight["epoch"])
        require(rejection["error"] == "worker lease is no longer valid", f"提交拒绝原因异常：{rejection}")
        expected = ["run_created", "tool_requested", "run_cancelled"]
        require(aborted == self.cancelled(path, expected), "取消结果被 worker 修改")
        require(self.request(path + "/events?after=2")[0][0]["kind"] == "run_cancelled", "增量事件缺少取消记录")
        docker("rm", name, "--force")
        self.compose("start", "worker")
        print("通过：排队时并发取消、运行中取消、迟到提交拒绝和幂等重试", flush=True)

    def check_history(self):
        self.compose("stop", "worker")
        ids = []
        for index in range(4):
            run_id, path = self.submit(f"history-{index}", f"历史分页验收 {index}")
            ids.append(str(run_id))
            if index in (1, 3):
                self.request(path + "/cancel", method="POST")

        first, _ = self.request("/runs?limit=2")
        require([row["id"] for row in first["items"]] == [ids[3], ids[2]], f"首屏排序异常：{first}")
        require(first["nextCursor"] == ids[2], f"首屏游标异常：{first}")
        created, _ = self.request(f"/runs/{ids[3]}/events")
        require(first["items"][0]["createdAt"] == created[0]["createdAt"], "摘要中的创建时间与事件不符")
        self.submit("history-new", "翻页过程中创建的新任务")

        # 新任务插到列表前面，游标仍从上一页最后一个 ID 之后读取。
        second, _ = self.request("/runs?" + urlencode({"limit": 2, "before": first["nextCursor"]}))
        require([row["id"] for row in second["items"]] == [ids[1], ids[0]], f"分页有重复或遗漏：{second}")
        filtered, _ = self.request("/runs?limit=1&status=cancelled")
        require(filtered["nextCursor"] == ids[3], f"状态筛选第一条异常：{filtered}")
        older, _ = self.request(f"/runs?limit=1&status=cancelled&before={filtered['nextCursor']}")
        require(older["items"][0]["id"] == ids[1] and older["items"][0]["status"] == "cancelled", f"筛选翻页异常：{older}")
        empty, _ = self.request("/runs?before=1")
        require(empty == {"items": [], "nextCursor": None}, f"末尾空页异常：{empty}")

        for query in ("limit=0", "limit=51", "limit=1&limit=2", "status=invalid", "before=-1", "before=%XX", "unknown=1"):
            bad, _ = self.request(f"/runs?{query}", 400)
            require(bad["error"] == "invalid_history_query", f"无效查询未正确处理：{bad}")
        self.compose("start", "worker")
        print("通过：历史倒序、游标翻页、状态筛选、并发新增和非法参数", flush=True)

    def check_retries(self):
        baseline, candidate = "ops-runbook-v1", "ops-runbook-v2"
        self.compose("stop", "worker")
        question = "Redis 下线后怎么办？"
        source_id, source_path = self.submit("retry-source", question)
        bad, _ = self.request(source_path + "/retry", 409, method="POST")
        require(bad["error"] == "run_not_retryable", f"排队任务不应创建重试：{bad}")
        self.request(source_path + "/cancel", method="POST")
        source, _ = self.request(source_path)
        source_events, _ = self.request(source_path + "/events")
        retired_id, retired_path = self.submit("retry-retired-source", question)
        self.request(retired_path + "/cancel", method="POST")
        self.release_command("publish", baseline, candidate)

        def retry():
            request = Request(self.api + source_path + "/retry", method="POST")
            with urlopen(request, timeout=4) as response:
                return response.status, response.headers["Location"], json.load(response)

        with ThreadPoolExecutor(max_workers=2) as pool:
            first, second = [pool.submit(retry) for _ in range(2)]
            status, path, child = first.result()
            other_status, other_path, other_child = second.result()
        require(sorted([status, other_status]) == [200, 202] and path == other_path and child == other_child,
                f"并发重试创建了不一致的结果：{child}, {other_child}")
        require(child["id"] != source_id and child["retryOf"] == str(source_id) and child["state"] == {},
                f"新任务应保留来源并从空检查点启动：{child}")
        require(child["release"] == baseline and child["input"] == question, f"没有继承原问题和版本：{child}")
        require(self.active_release() == candidate, "重试测试应在默认版本变化时执行")
        require(self.request(source_path)[0] == source and self.request(source_path + "/events")[0] == source_events,
                "重试修改了原任务或原事件")
        self.request("/runs", 400, body={"input": question}, key=f"retry:{source_id}")

        # 内置只读工具正常完成。用隔离数据库准备一次失败，验证 failed 的重试入口。
        failed_id, failed_path = self.submit("retry-failed-source", question)
        self.compose("exec", "-T", "postgres", "psql", "-U", "agent", "-d", "agent_runtime", "-v", "ON_ERROR_STOP=1", "-c", f"""
            WITH marked AS (
                UPDATE agent_runs SET status='failed', step=1, state='{{"error":"test_failure"}}'
                WHERE id={failed_id} AND status='queued' RETURNING id
            )
            INSERT INTO agent_events (run_id, seq, kind, data)
            SELECT id, 2, 'run_failed', '{{"code":"test_failure"}}' FROM marked
        """)
        second_child, second_path = self.request(failed_path + "/retry", 202, method="POST")
        require(second_child["retryOf"] == str(failed_id) and second_child["release"] == candidate, f"失败后的重试异常：{second_child}")
        self.release_command("publish", candidate, baseline)
        self.compose("start", "worker")

        finished, events = self.succeeded(path, "cache-fallback")
        require(events[0]["data"] == {"release": baseline, "retryOf": str(source_id)}, f"首条事件缺少重试来源：{events[0]}")
        latest, _ = self.succeeded(second_path, "cache-fallback")
        history, _ = self.request("/runs?limit=10&status=succeeded")
        parents = {row["id"]: row.get("retryOf") for row in history["items"]}
        require(parents[str(latest["id"])] == str(failed_id) and parents[str(finished["id"])] == str(source_id),
                f"历史摘要缺少重试来源：{history}")
        require(self.request(source_path + "/retry", method="POST")[0] == finished, "重发没有返回已完成的原重试")
        self.request(path + "/retry", 409, method="POST")
        self.request("/runs/9223372036854775807/retry", 404, method="POST")

        # 在隔离库模拟退役版本。已有重试可读取，新请求也不会生成无法领取的任务。
        self.compose("exec", "-T", "postgres", "psql", "-U", "agent", "-d", "agent_runtime", "-v", "ON_ERROR_STOP=1", "-c",
                     f"UPDATE agent_runs SET release_id='retired-release' WHERE id IN ({source_id}, {child['id']}, {retired_id})")
        retained, _ = self.request(source_path + "/retry", method="POST")
        require(retained["id"] == child["id"], "退役版本的既有重试记录丢失")
        error, _ = self.request(retired_path + "/retry", 409, method="POST")
        require(error["error"] == "retry_release_unavailable", f"退役版本应拒绝新重试：{error}")
        print("通过：取消与失败重试、并发幂等、版本继承、退役版本保护和来源追溯", flush=True)

    def wait_log(self, name, **fields):
        def matching_log():
            for line in docker("logs", name).splitlines():
                log = json.loads(line)
                if all(log.get(key) == value for key, value in fields.items()):
                    return log
            return None

        return until(f"{name} 记录 {fields}", matching_log)

    def interrupted_run(self, action):
        self.compose("stop", "worker")
        name = f"{self.project}-{action}"
        self.slow_workers.append(name)
        self.compose("run", "-d", "--no-deps", "--name", name, "-e", "AGENT_CHECKPOINT_DELAY=4s", "worker")
        run_id, path = self.submit(action, "PostgreSQL 为什么返回 503？")
        # 等待工具算完但在事务提交之前注入故障，验证能从上一个检查点接手。
        in_flight = self.wait_log(name, msg="step_computed", run_id=run_id, kind="tool_completed")
        before, _ = self.request(path)
        events, _ = self.request(path + "/events")
        require(before["status"] == "running" and before["step"] == 1, f"故障注入时机异常：{before}")
        require([event["kind"] for event in events] == EVENTS[:2], f"工具结果提前提交：{events}")

        if action == "kill":
            docker("kill", "--signal=KILL", name)
        else:
            docker("pause", name)
        self.compose("start", "worker")
        completed, events = self.succeeded(path, "database-readiness")
        require(completed["id"] == run_id, f"恢复后任务 ID 变化：{completed}")
        require(completed["leaseEpoch"] > in_flight["epoch"], f"恢复后未出现更高的租约代次：{completed}")
        if action == "pause":
            # 同一旧进程恢复执行后会尝试提交，用它来验证租约代次的写入保护。
            docker("unpause", name)
            rejection = self.wait_log(name, msg="checkpoint_rejected", run_id=run_id, epoch=in_flight["epoch"])
            require(rejection["error"] == "worker lease is no longer valid", f"拒绝原因异常：{rejection}")
            require(self.request(path)[0] == completed, "旧 worker 修改了最终状态")
            require(self.request(path + "/events")[0] == events, "旧 worker 修改了持久化事件")
        docker("rm", name, "--force")
        print(f"通过：{action}，任务 {run_id} 从租约代次 {in_flight['epoch']} 恢复到 {completed['leaseEpoch']}", flush=True)

    def check(self, reuse_image=False):
        self.compose("up", "--no-build" if reuse_image else "--build", "--wait", "--wait-timeout", "60", timeout=240)
        address = self.compose("port", "api", "8084")
        require(address.startswith("127.0.0.1:") and not address.endswith(":0"), f"隔离端口异常：{address}")
        self.api = f"http://{address}"
        self.request("/readyz")
        require(self.active_release() == "ops-runbook-v1", "初始默认版本不正确")
        report = self.release_command("eval")
        require(report["passed"] and len(report["cases"]) == 4, f"发布评估失败：{report}")
        print("通过：发布评估的 4 个样本，越权工具未执行", flush=True)
        self.check_overview()

        question = "Redis 缓存下线怎么办？"
        run_id, path = self.submit("normal", question)
        replay, location = self.request("/runs", body={"input": question}, key="normal")
        require(replay["id"] == run_id and location == path, f"相同请求创建了新任务：{replay}")
        conflict, _ = self.request("/runs", 409, body={"input": "不同的问题"}, key="normal")
        require(conflict["error"] == "idempotency_conflict", f"幂等冲突异常：{conflict}")
        _, events = self.succeeded(path, "cache-fallback")
        require(self.request(path + "/events?after=2")[0] == events[2:], "事件游标返回异常")
        for query in ("after=1&after=2", "after=%XX", "after=2147483648", "after=-1", "after="):
            error, _ = self.request(path + "/events?" + query, 400)
            require(error["error"] == "invalid_cursor", f"无效游标未正确处理：{error}")
        finished, _ = self.request(path + "/cancel", 409, method="POST")
        require(finished["error"] == "run_already_finished", f"已完成任务的取消响应异常：{finished}")
        require(self.request(path + "/events")[0] == events, "取消请求修改了已完成任务")
        self.request("/runs/9223372036854775807/cancel", 404, method="POST")
        print("通过：提交、幂等、证据引用和事件游标", flush=True)

        self.check_cancellation()
        self.interrupted_run("kill")
        self.interrupted_run("pause")
        self.check_releases()
        self.check_history()
        self.check_retries()

    def clean(self):
        # 只清理带本次随机项目名的容器和卷，常驻演示数据留在独立项目里。
        for name in self.slow_workers:
            docker("unpause", name, check=False)
            docker("rm", "--force", name, check=False)
        self.compose("down", "--volumes", "--remove-orphans")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="验收本机 Agent 运行时")
    parser.add_argument("--reuse-image", action="store_true", help="使用已构建的 Agent 镜像")
    args = parser.parse_args()
    verification = Verification()
    try:
        verification.check(args.reuse_image)
    finally:
        verification.clean()
