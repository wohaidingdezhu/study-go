#!/usr/bin/env python3
"""从仓库中的练习题库生成今天的任务和闭卷复习队列。"""

import argparse
from contextlib import closing
from datetime import date, timedelta
import json
from pathlib import Path
import sqlite3


ROOT = Path(__file__).resolve().parents[1]
INTERVALS = (1, 3, 7, 14, 30)
DEFAULT_DB = ROOT / ".study/progress.sqlite3"


def load_deck():
    tasks = {task["id"]: task for task in json.loads((ROOT / "docs/study-deck.json").read_text(encoding="utf-8"))}
    cards = {card["id"]: (task, card) for task in tasks.values() for card in task["cards"]}
    return tasks, cards


def init_progress(path):
    path.parent.mkdir(parents=True, exist_ok=True)
    with closing(sqlite3.connect(path)) as db:
        db.executescript("""
            CREATE TABLE IF NOT EXISTS practice (
                task_id TEXT PRIMARY KEY,
                evidence TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS review (
                card_id TEXT PRIMARY KEY,
                streak INTEGER NOT NULL,
                due TEXT NOT NULL
            );
        """)


def due_cards(db, cards):
    rows = db.execute("SELECT card_id, streak, due FROM review WHERE due <= ? ORDER BY due, card_id",
                      (date.today().isoformat(),))
    # 题库移除旧题后保留旧进度，复习时跳过失效卡片。
    return [row for row in rows if row[0] in cards]


def sync_review_cards(db, tasks):
    # 只给已经记录证据的练习补题。课程新增卡片后，旧进度也能继续学习新内容。
    # ON CONFLICT 保留原卡的评分和日期，事务由调用方管理，与进度读写一起提交。
    today = date.today().isoformat()
    db.executemany("""
        INSERT INTO review (card_id, streak, due)
        SELECT ?, 0, ? WHERE EXISTS (SELECT 1 FROM practice WHERE task_id = ?)
        ON CONFLICT (card_id) DO NOTHING
    """, [(card["id"], today, task["id"]) for task in tasks.values() for card in task["cards"]])


def grade_card(db, card_id, streak, previous_due, correct):
    today = date.today()
    delay = INTERVALS[streak] if correct else 1
    next_streak = min(streak + 1, len(INTERVALS) - 1) if correct else 0
    next_due = (today + timedelta(days=delay)).isoformat()
    # 使用旧日期和档位做并发校验，浏览器和命令行同时复习也只提交一次。
    with db:
        update = db.execute("""
            UPDATE review SET streak = ?, due = ?
            WHERE card_id = ? AND streak = ? AND due = ? AND due <= ?
        """, (next_streak, next_due, card_id, streak, previous_due, today.isoformat()))
    return next_due if update.rowcount else None


def print_task(task):
    print(f"{task['id']} · {task['title']}")
    print(f"  练习  {task['practice']}")
    print(f"  验收  {task['evidence']}")
    print(f"  入口  {task['guide']}")


def show_today(db, tasks, cards):
    completed = {row[0] for row in db.execute("SELECT task_id FROM practice")}
    due = len(due_cards(db, cards))
    print(f"已交练习证据 {len(completed & tasks.keys())}/{len(tasks)}，到期复习 {due} 张")
    if due:
        print("先闭卷复习：python3 scripts/study.py review")
    next_task = next((task for task_id, task in tasks.items() if task_id not in completed), None)
    if next_task:
        print("\n下一项练习")
        print_task(next_task)
    else:
        print("主线练习已交齐。继续按到期队列复习，可回到项目做变式。")


def record_practice(db, task, evidence):
    # 练习证据和新卡片一起提交。重复更新证据时保留已有复习进度。
    with db:
        db.execute("""
            INSERT INTO practice (task_id, evidence) VALUES (?, ?)
            ON CONFLICT (task_id) DO UPDATE SET evidence = excluded.evidence
        """, (task["id"], evidence))
        sync_review_cards(db, {task["id"]: task})


def complete(db, task, evidence):
    if evidence is None:
        evidence = input("记录实际结果（例如响应码、测试输出或 SQL 结果）：")
    evidence = evidence.strip()
    if not evidence:
        raise SystemExit("请写下一条实际结果，再记录完成。")
    record_practice(db, task, evidence)
    print("已保存。首次闭卷：python3 scripts/study.py review")


def review(db, cards, limit):
    available = due_cards(db, cards)
    if not available:
        print("今天没有到期卡片。")
        return

    for card_id, streak, previous_due in available[:limit]:
        task, card = cards[card_id]
        print(f"\n{task['title']} · {card['question']}")
        if input("闭卷说出答案或写下结果。回车看要点，输入 q 退出：").strip().lower() == "q":
            return
        print(card["answer"])
        while True:
            grade = input("1 = 独立答全，0 = 卡住或漏掉关键点，q = 退出：").strip().lower()
            if grade in {"0", "1", "q"}:
                break
        if grade == "q":
            return

        # 间隔从实际答题日期算起。漏答重回第一档，答对逐级拉开间隔。
        next_due = grade_card(db, card_id, streak, previous_due, grade == "1")
        if next_due is None:
            print("这张卡的进度已在另一个会话更新。")
            continue
        print(f"下次复习 {next_due}")
        if grade == "0":
            print(f"现在做一次纠错：{card['repair']}")
    if len(available) > limit:
        print(f"还有 {len(available) - limit} 张到期卡片，下次运行可继续。")


def main():
    tasks, cards = load_deck()
    parser = argparse.ArgumentParser(description="Go / Java 订单系统学习工具")
    parser.add_argument("--db", type=Path, default=DEFAULT_DB, help="另选本地进度数据库")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("today", help="查看到期复习和下一项练习")
    commands.add_parser("progress", help="查看已记录的练习证据")
    practice = commands.add_parser("practice", help="查看所有练习，或指定一项")
    practice.add_argument("task", nargs="?", choices=tuple(tasks))
    done = commands.add_parser("done", help="记录练习证据，加入对应复习卡")
    done.add_argument("task", choices=tuple(tasks))
    done.add_argument("--evidence", help="实际结果，省略时会提示输入")
    review_command = commands.add_parser("review", help="闭卷复习今天到期的卡片")
    review_command.add_argument("--limit", type=int, default=5, help="本次最多几张，默认 5")
    args = parser.parse_args()
    if args.command == "practice":
        for task in ([tasks[args.task]] if args.task else tasks.values()):
            print_task(task)
        return
    if args.command == "review" and args.limit < 1:
        parser.error("--limit 至少为 1")

    init_progress(args.db)
    db = sqlite3.connect(args.db)
    try:
        with db:
            sync_review_cards(db, tasks)
        if args.command == "today":
            show_today(db, tasks, cards)
        elif args.command == "progress":
            evidence = dict(db.execute("SELECT task_id, evidence FROM practice"))
            if not evidence.keys() & tasks.keys():
                print("还没有练习证据。")
            for task_id in tasks:
                if task_id in evidence:
                    print(f"{task_id} · {tasks[task_id]['title']}\n  {evidence[task_id]}")
        elif args.command == "done":
            complete(db, tasks[args.task], args.evidence)
        else:
            review(db, cards, args.limit)
    except (EOFError, KeyboardInterrupt):
        print("\n已退出，之前提交的进度已保存。")
    finally:
        db.close()


if __name__ == "__main__":
    main()
