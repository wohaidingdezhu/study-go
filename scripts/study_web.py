#!/usr/bin/env python3
"""从本机提供 React、学习进度和可选的 Agent 实验接口。"""

import argparse
from contextlib import closing
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import re
import sqlite3
from urllib.parse import parse_qs, urlsplit

from study import DEFAULT_DB, INTERVALS, ROOT, due_cards, grade_card, init_progress, load_deck, record_practice


DIST = ROOT / "web/dist"
ASSETS = {
    "/": ("index.html", "text/html; charset=utf-8"),
    "/assets/app.css": ("assets/app.css", "text/css; charset=utf-8"),
    "/assets/app.js": ("assets/app.js", "text/javascript; charset=utf-8"),
}
AGENT_PATH = re.compile(r"/(releases|readyz|runs(?:/[1-9][0-9]*(?:/events)?)?)\Z")
GUIDE_DIRS = {"docs", "exercises", "go", "java", "labs", "scripts"}
GUIDE_TYPES = {".md", ".go", ".java", ".py", ".sh", ".sql", ".yaml", ".xml"}


class RequestError(Exception):
    def __init__(self, status, message):
        self.status = status
        self.message = message


class StudyHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.handle_request()

    def do_POST(self):
        self.handle_request()

    def handle_request(self):
        port = self.server.server_port
        hosts = {f"127.0.0.1:{port}", f"localhost:{port}"}
        origin = self.headers.get("Origin")
        try:
            if self.headers.get("Host") not in hosts or (origin and origin not in {f"http://{host}" for host in hosts}):
                raise RequestError(403, "请通过本机地址访问学习工作台。")
            url = urlsplit(self.path)
            if url.path.startswith("/api/agent/"):
                self.proxy_agent(url.path.removeprefix("/api/agent"), url.query)
            elif url.path.startswith("/api/"):
                self.send_json(self.study_data(url.path, parse_qs(url.query)))
            elif self.command == "GET" and url.path in ASSETS:
                name, content_type = ASSETS[url.path]
                self.send_body(200, content_type, (DIST / name).read_bytes())
            else:
                raise RequestError(404, "没有找到这个页面。")
        except RequestError as error:
            self.send_json({"error": error.message}, error.status)
        except (BrokenPipeError, ConnectionResetError):
            # 页面切换时浏览器可以主动取消轮询，无需再次尝试写响应。
            pass
        except (OSError, sqlite3.Error, http.client.HTTPException) as error:
            self.log_error("%s", error)
            self.send_json({"error": "本机服务暂时无法处理，请重试。"}, 500)

    def read_body(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError as error:
            raise RequestError(400, "请求长度不正确。") from error
        if not 0 < length <= 16_384:
            raise RequestError(400, "请提交内容，最多 16 KB。")
        return self.rfile.read(length)

    def read_json(self):
        if self.headers.get_content_type() != "application/json":
            raise RequestError(415, "请提交 JSON 内容。")
        try:
            payload = json.loads(self.read_body())
        except (ValueError, UnicodeError) as error:
            raise RequestError(400, "请求内容格式不正确。") from error
        if not isinstance(payload, dict):
            raise RequestError(400, "请提交 JSON 对象。")
        return payload

    def study_data(self, path, query):
        if path == "/api/guide" and self.command == "GET":
            return self.read_guide(query.get("path", [""])[0])

        # 每个请求使用自己的 SQLite 连接，命令行和浏览器使用同一套事务方法。
        with closing(sqlite3.connect(self.server.db_path)) as db:
            tasks, cards = self.server.deck
            if path == "/api/study" and self.command == "GET":
                with db:
                    db.execute("BEGIN")
                    evidence = dict(db.execute("SELECT task_id, evidence FROM practice"))
                    queue = due_cards(db, cards)
                return {
                    "tasks": [{**{key: task[key] for key in ("id", "title", "guide", "practice", "evidence")},
                               "note": evidence.get(task["id"])} for task in tasks.values()],
                    "due": [{"id": card_id, "task": cards[card_id][0]["title"],
                             "question": cards[card_id][1]["question"], "date": due, "streak": streak}
                            for card_id, streak, due in queue],
                }
            if path.startswith("/api/practice/") and self.command == "POST":
                task_id = path.removeprefix("/api/practice/")
                if task_id not in tasks:
                    raise RequestError(404, "找不到这项练习。")
                note = self.read_json().get("evidence")
                if not isinstance(note, str) or not 0 < len(note.strip()) <= 2000:
                    raise RequestError(400, "请写一条实际结果，最多 2000 字。")
                record_practice(db, tasks[task_id], note.strip())
                return {"saved": True}
            if path.startswith("/api/review/"):
                card_id = path.removeprefix("/api/review/")
                if card_id not in cards:
                    raise RequestError(404, "找不到这张卡片。")
                _, card = cards[card_id]
                if self.command == "GET":
                    return {"answer": card["answer"], "repair": card["repair"]}
                payload = self.read_json()
                streak, due, correct = payload.get("streak"), payload.get("date"), payload.get("correct")
                if type(streak) is not int or not 0 <= streak < len(INTERVALS) or not isinstance(due, str) or type(correct) is not bool:
                    raise RequestError(400, "复习记录格式不正确。")
                next_due = grade_card(db, card_id, streak, due, correct)
                if next_due is None:
                    raise RequestError(409, "这张卡的复习进度已经更新，请刷新后继续。")
                return {"nextDue": next_due}
        raise RequestError(404, "没有找到这个接口。")

    def read_guide(self, name):
        # 只开放教学文件类型。相对链接解析结束后再次检查根目录及允许的目录。
        if not name or "\x00" in name:
            raise RequestError(404, "找不到这份材料。")
        path = (ROOT / name).resolve()
        try:
            relative = path.relative_to(ROOT)
        except ValueError as error:
            raise RequestError(404, "找不到这份材料。") from error
        if relative.as_posix() != "README.md":
            if not relative.parts or relative.parts[0] not in GUIDE_DIRS or path.suffix not in GUIDE_TYPES or any(part.startswith(".") for part in relative.parts):
                raise RequestError(404, "找不到这份材料。")
        if not path.is_file():
            raise RequestError(404, "找不到这份材料。")
        return {"path": relative.as_posix(), "type": path.suffix, "text": path.read_text(encoding="utf-8")}

    def proxy_agent(self, path, query):
        if not AGENT_PATH.fullmatch(path) or (self.command == "POST" and path != "/runs") or (self.command == "GET" and path == "/runs"):
            raise RequestError(404, "没有找到这个 Agent 接口。")
        data = self.read_body() if self.command == "POST" else None
        headers = {"Content-Type": "application/json", "Idempotency-Key": self.headers.get("Idempotency-Key", "")}
        # 同源代理只调用本机的实验服务。Agent 停机时学习功能仍独立运行。
        with closing(http.client.HTTPConnection("127.0.0.1", self.server.agent_port, timeout=5)) as agent:
            try:
                agent.request(self.command, path + (f"?{query}" if query else ""), data, headers)
                response = agent.getresponse()
                content = response.read()
            except (OSError, http.client.HTTPException) as error:
                raise RequestError(503, "Agent 服务暂不可用。启动实验容器后再重试。") from error
        self.send_body(response.status, response.getheader("Content-Type", "application/json"), content)

    def send_json(self, payload, status=200):
        self.send_body(status, "application/json; charset=utf-8", json.dumps(payload, ensure_ascii=False).encode())

    def send_body(self, status, content_type, content):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(content)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        self.wfile.write(content)


def main():
    parser = argparse.ArgumentParser(description="Go × Java 本地学习工作台")
    parser.add_argument("--port", type=int, default=4173)
    parser.add_argument("--agent-port", type=int, default=18084)
    parser.add_argument("--db", type=type(DEFAULT_DB), default=DEFAULT_DB)
    args = parser.parse_args()
    if not (DIST / "index.html").exists():
        parser.error("请先执行 npm --prefix web run build")
    init_progress(args.db)
    with ThreadingHTTPServer(("127.0.0.1", args.port), StudyHandler) as server:
        server.db_path, server.agent_port, server.deck = args.db, args.agent_port, load_deck()
        print(f"学习工作台 http://127.0.0.1:{server.server_port}", flush=True)
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            pass


if __name__ == "__main__":
    main()
