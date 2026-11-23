"""检查课程补题时，原有练习证据和复习日期仍然保留。"""

from datetime import date
from pathlib import Path
import sqlite3
import subprocess
import sys
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest

from study import ROOT, init_progress, load_deck, record_practice, sync_review_cards
from study_web import StudyHandler


class ReviewSyncTests(unittest.TestCase):
    def setUp(self):
        self.directory = TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.path = Path(self.directory.name) / "progress.sqlite3"
        init_progress(self.path)
        self.db = sqlite3.connect(self.path)
        self.addCleanup(self.db.close)
        self.task = {"id": "cache", "cards": [{"id": "existing"}, {"id": "added"}]}

    def seed_previous_progress(self):
        record_practice(self.db, {**self.task, "cards": self.task["cards"][:1]}, "已经验证缓存回源")
        with self.db:
            self.db.execute("UPDATE review SET streak=3, due='2099-01-01' WHERE card_id='existing'")

    def test_adds_new_cards_to_completed_tasks_and_keeps_schedule(self):
        self.seed_previous_progress()
        with self.db:
            sync_review_cards(self.db, {"cache": self.task})
        self.assertEqual(self.db.execute("SELECT streak, due FROM review WHERE card_id='existing'").fetchone(),
                         (3, "2099-01-01"))
        self.assertEqual(self.db.execute("SELECT streak, due FROM review WHERE card_id='added'").fetchone(),
                         (0, date.today().isoformat()))
        self.assertEqual(self.db.execute("SELECT evidence FROM practice WHERE task_id='cache'").fetchone(),
                         ("已经验证缓存回源",))

    def test_repeated_sync_keeps_existing_rows(self):
        self.seed_previous_progress()
        with self.db:
            sync_review_cards(self.db, {"cache": self.task})
        previous_changes = self.db.total_changes
        with self.db:
            sync_review_cards(self.db, {"cache": self.task})
        self.assertEqual(self.db.total_changes, previous_changes)
        self.assertEqual(self.db.execute("SELECT COUNT(*) FROM review").fetchone()[0], 2)

    def test_unfinished_tasks_do_not_get_review_cards(self):
        with self.db:
            sync_review_cards(self.db, {"cache": self.task})
        self.assertEqual(self.db.execute("SELECT COUNT(*) FROM review").fetchone()[0], 0)

    def test_removed_tasks_and_cards_remain_preserved(self):
        with self.db:
            self.db.execute("INSERT INTO practice VALUES ('retired', '旧练习证据')")
            self.db.execute("INSERT INTO review VALUES ('retired-card', 2, '2099-01-02')")
            sync_review_cards(self.db, {"cache": self.task})
        self.assertEqual(self.db.execute("SELECT * FROM review").fetchall(),
                         [("retired-card", 2, "2099-01-02")])
        self.assertEqual(self.db.execute("SELECT evidence FROM practice WHERE task_id='retired'").fetchone(),
                         ("旧练习证据",))

    def test_saving_evidence_also_queues_new_cards_without_resetting_old(self):
        self.seed_previous_progress()
        record_practice(self.db, self.task, "第二次验证结果")
        self.assertEqual(self.db.execute("SELECT streak, due FROM review WHERE card_id='existing'").fetchone(),
                         (3, "2099-01-01"))
        self.assertEqual(self.db.execute("SELECT COUNT(*) FROM review").fetchone()[0], 2)
        self.assertEqual(self.db.execute("SELECT evidence FROM practice WHERE task_id='cache'").fetchone(),
                         ("第二次验证结果",))

    def test_cli_today_syncs_existing_progress(self):
        tasks, _ = load_deck()
        task = tasks["cache"]
        record_practice(self.db, {**task, "cards": task["cards"][:1]}, "已完成缓存练习")
        result = subprocess.run([sys.executable, str(ROOT / "scripts/study.py"),
                                 "--db", str(self.path), "today"],
                                capture_output=True, text=True, check=True)
        self.assertIn(f"到期复习 {len(task['cards'])} 张", result.stdout)
        self.assertEqual(self.db.execute("SELECT COUNT(*) FROM review").fetchone()[0], len(task["cards"]))

    def test_workbench_snapshot_syncs_existing_progress(self):
        tasks, cards = load_deck()
        task = tasks["cache"]
        record_practice(self.db, {**task, "cards": task["cards"][:1]}, "已完成缓存练习")
        handler = object.__new__(StudyHandler)
        handler.command = "GET"
        handler.server = SimpleNamespace(db_path=self.path, deck=(tasks, cards))
        snapshot = handler.study_data("/api/study", {})
        self.assertEqual({card["id"] for card in snapshot["due"]}, {card["id"] for card in task["cards"]})
        self.assertEqual(next(row for row in snapshot["tasks"] if row["id"] == "cache")["note"], "已完成缓存练习")


if __name__ == "__main__":
    unittest.main()
