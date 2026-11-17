import json
import os
from pathlib import Path
import subprocess
import time
from urllib.error import HTTPError
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


class Verification:
    def __init__(self):
        self.project = f"study-go-agent-check-{uuid4().hex[:8]}"
        self.slow_workers = []

    def compose(self, *args, **options):
        return docker("compose", "-p", self.project, "-f", str(COMPOSE_FILE), *args, **options)

    def request(self, path, expected=200, *, body=None, key=None):
        headers = {"Content-Type": "application/json", "Idempotency-Key": key} if key else {}
        payload = json.dumps(body).encode() if body is not None else None
        request = Request(self.api + path, data=payload, headers=headers)
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

    def check(self):
        self.compose("up", "--build", "--wait", "--wait-timeout", "60", timeout=240)
        address = self.compose("port", "api", "8084")
        require(address.startswith("127.0.0.1:") and not address.endswith(":0"), f"隔离端口异常：{address}")
        self.api = f"http://{address}"
        self.request("/readyz")
        require(self.active_release() == "ops-runbook-v1", "初始默认版本不正确")
        report = self.release_command("eval")
        require(report["passed"] and len(report["cases"]) == 4, f"发布评估失败：{report}")
        print("通过：发布评估的 4 个样本，越权工具未执行", flush=True)

        question = "Redis 缓存下线怎么办？"
        run_id, path = self.submit("normal", question)
        replay, location = self.request("/runs", body={"input": question}, key="normal")
        require(replay["id"] == run_id and location == path, f"相同请求创建了新任务：{replay}")
        conflict, _ = self.request("/runs", 409, body={"input": "不同的问题"}, key="normal")
        require(conflict["error"] == "idempotency_conflict", f"幂等冲突异常：{conflict}")
        _, events = self.succeeded(path, "cache-fallback")
        require(self.request(path + "/events?after=2")[0] == events[2:], "事件游标返回异常")
        print("通过：提交、幂等、证据引用和事件游标", flush=True)

        self.interrupted_run("kill")
        self.interrupted_run("pause")
        self.check_releases()

    def clean(self):
        # 只清理带本次随机项目名的容器和卷，常驻演示数据留在独立项目里。
        for name in self.slow_workers:
            docker("unpause", name, check=False)
            docker("rm", "--force", name, check=False)
        self.compose("down", "--volumes", "--remove-orphans")


if __name__ == "__main__":
    verification = Verification()
    try:
        verification.check()
    finally:
        verification.clean()
