# -*- coding: utf-8 -*-
"""
window_monitor 单测（AUDIT §4 缺陷即测试：弹窗红线工具的行为钉子）。

运行：python -m unittest tools/ops/test_window_monitor.py -v
纯标准库；窗口清单一律注入，不真 EnumWindows、不依赖当前桌面状态。
覆盖：快照写读 round-trip / diff 新增检测（多重集）/ 过滤逻辑 / CLI 退出码契约。
"""

import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import window_monitor as wm  # noqa: E402


def raw(pid, title, cls="App"):
    """构造一枚未过滤的原始窗口条目（模拟 raw_window_entries 的产出）。"""
    return {"pid": pid, "title": title, "class_name": cls}


def filtered(*tuples):
    """构造过滤后的窗口清单（模拟 enumerate_windows 的产出）。"""
    return [{"pid": p, "title": t} for p, t in tuples]


class TempDirCase(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.tmp = Path(self._tmp.name)


# ── 过滤逻辑（纯函数注入） ───────────────────────────────────────────────────

class TestFilterEntries(TempDirCase):

    def test_default_drops_empty_title_and_shell_windows(self):
        entries = [raw(1, ""), raw(2, "   "), raw(2, "桌面", "Progman"),
                   raw(3, "壁纸层", "WorkerW"), raw(4, "记事本")]
        self.assertEqual(wm.filter_entries(entries), filtered((4, "记事本")))

    def test_all_keeps_everything_including_empty_and_shell(self):
        entries = [raw(1, ""), raw(2, "桌面", "Progman"), raw(3, "壁纸层", "WorkerW")]
        out = wm.filter_entries(entries, include_all=True)
        self.assertEqual(out, [{"pid": 1, "title": ""},
                               {"pid": 2, "title": "桌面"},
                               {"pid": 3, "title": "壁纸层"}])

    def test_output_drops_class_name_and_sorts_by_pid_title(self):
        entries = [raw(9, "b"), raw(1, "z"), raw(1, "a")]
        self.assertEqual(wm.filter_entries(entries),
                         [{"pid": 1, "title": "a"},
                          {"pid": 1, "title": "z"},
                          {"pid": 9, "title": "b"}])

    def test_entry_without_class_name_is_not_treated_as_shell(self):
        e = {"pid": 5, "title": "无类名"}  # 防御：缺类名键时按普通窗口保留
        self.assertEqual(wm.filter_entries([e]), filtered((5, "无类名")))


# ── 快照写读 round-trip ──────────────────────────────────────────────────────

class TestSnapshotRoundTrip(TempDirCase):

    def test_write_read_roundtrip(self):
        windows = wm.filter_entries([raw(1, "A"), raw(2, "B")])
        snap = wm.build_snapshot(windows, include_all=False,
                                 captured_at="2026-10-07 00:00:00")
        p = self.tmp / "base.json"
        wm.write_snapshot(p, snap)
        back = wm.read_snapshot(p)
        self.assertEqual(back["schema"], wm.SNAPSHOT_SCHEMA)
        self.assertEqual(back["captured_at"], "2026-10-07 00:00:00")
        self.assertEqual(back["include_all"], False)
        self.assertEqual(back["count"], 2)
        self.assertEqual(back["windows"], windows)

    def test_roundtrip_keeps_utf8_and_special_chars_in_title(self):
        title = "中文弹窗 ✱ \t制表"  # 含非 ASCII 与制表符，TSV 装不下的形状
        p = self.tmp / "base.json"
        wm.write_snapshot(p, wm.build_snapshot([{"pid": 7, "title": title}]))
        self.assertEqual(wm.read_snapshot(p)["windows"],
                         [{"pid": 7, "title": title}])

    def test_read_rejects_garbage(self):
        p = self.tmp / "bad.json"
        p.write_text("not json", encoding="utf-8")
        with self.assertRaises(ValueError):
            wm.read_snapshot(p)

    def test_read_rejects_wrong_schema(self):
        p = self.tmp / "wrong.json"
        p.write_text(json.dumps({"schema": "x", "windows": []}), encoding="utf-8")
        with self.assertRaises(ValueError):
            wm.read_snapshot(p)

    def test_read_rejects_missing_windows_list(self):
        p = self.tmp / "nowin.json"
        p.write_text(json.dumps({"schema": wm.SNAPSHOT_SCHEMA}), encoding="utf-8")
        with self.assertRaises(ValueError):
            wm.read_snapshot(p)

    def test_read_missing_file_raises_oserror(self):
        with self.assertRaises(OSError):
            wm.read_snapshot(self.tmp / "nope.json")


# ── diff 新增检测（多重集，纯函数注入） ──────────────────────────────────────

class TestDiffWindows(unittest.TestCase):

    def test_empty_base_everything_is_new(self):
        cur = filtered((1, "a"), (2, "b"))
        self.assertEqual(wm.diff_windows([], cur),
                         [{"pid": 1, "title": "a", "count": 1},
                          {"pid": 2, "title": "b", "count": 1}])

    def test_identical_lists_no_new(self):
        cur = filtered((1, "a"), (2, "b"))
        self.assertEqual(wm.diff_windows(cur, cur), [])

    def test_disappeared_windows_are_not_reported(self):
        base = filtered((1, "a"), (2, "b"))
        self.assertEqual(wm.diff_windows(base, filtered((1, "a"))), [])

    def test_second_identical_popup_not_cancelled_by_first(self):
        base = filtered((1, "a"))
        cur = filtered((1, "a"), (1, "a"), (1, "a"))
        self.assertEqual(wm.diff_windows(base, cur),
                         [{"pid": 1, "title": "a", "count": 2}])

    def test_pid_reuse_same_title_reported_as_new(self):
        base = filtered((7, "设置"))
        cur = filtered((9, "设置"))
        self.assertEqual(wm.diff_windows(base, cur),
                         [{"pid": 9, "title": "设置", "count": 1}])

    def test_result_sorted_by_pid_then_title(self):
        base = []
        cur = filtered((9, "b"), (1, "z"), (1, "a"))
        self.assertEqual([w["pid"] for w in wm.diff_windows(base, cur)], [1, 1, 9])
        self.assertEqual([w["title"] for w in wm.diff_windows(base, cur)],
                         ["a", "z", "b"])


# ── CLI 退出码契约（注入 enumerate_windows，不碰真实桌面） ────────────────────

FAKE_WINDOWS = filtered((100, "PocketAgent"), (101, "Visual Studio Code"))


class TestCli(TempDirCase):

    def _patch_enum(self, windows):
        return mock.patch.object(wm, "enumerate_windows",
                                 mock.Mock(return_value=list(windows)))

    def _make_base(self, name, windows):
        p = self.tmp / name
        wm.write_snapshot(p, wm.build_snapshot(windows, include_all=False,
                                               captured_at="T0"))
        return str(p)

    def test_snapshot_writes_readable_snapshot(self):
        with self._patch_enum(FAKE_WINDOWS):
            rc = wm.main(["--snapshot", str(self.tmp / "base.json")])
        self.assertEqual(rc, wm.EXIT_CLEAN)
        back = wm.read_snapshot(self.tmp / "base.json")
        self.assertEqual(back["windows"], FAKE_WINDOWS)
        self.assertEqual(back["count"], len(FAKE_WINDOWS))

    def test_diff_clean_exit0(self):
        base = self._make_base("base.json", FAKE_WINDOWS)
        with self._patch_enum(FAKE_WINDOWS):
            self.assertEqual(wm.main(["--diff", base]), wm.EXIT_CLEAN)

    def test_diff_new_window_exit1_and_report_written(self):
        base = self._make_base("base.json", FAKE_WINDOWS)
        popup = FAKE_WINDOWS + filtered((200, "报警弹窗"))
        out = self.tmp / "report.txt"
        with self._patch_enum(popup):
            rc = wm.main(["--diff", base, "--out", str(out)])
        self.assertEqual(rc, wm.EXIT_NEW_WINDOW)
        text = out.read_text(encoding="utf-8")
        self.assertIn("新增 200\t报警弹窗", text)
        self.assertIn("有新增（退出码 1）", text)

    def test_diff_corrupt_base_exit2(self):
        bad = self.tmp / "bad.json"
        bad.write_text("garbage", encoding="utf-8")
        with self._patch_enum(FAKE_WINDOWS):
            self.assertEqual(wm.main(["--diff", str(bad)]), wm.EXIT_ERROR)

    def test_diff_missing_base_exit2(self):
        with self._patch_enum(FAKE_WINDOWS):
            self.assertEqual(wm.main(["--diff", str(self.tmp / "nope.json")]),
                             wm.EXIT_ERROR)

    def test_snapshot_and_diff_mutually_exclusive_exit2(self):
        rc = wm.main(["--snapshot", "a.json", "--diff", "b.json"])
        self.assertEqual(rc, wm.EXIT_ERROR)

    def test_out_requires_diff_exit2(self):
        rc = wm.main(["--out", str(self.tmp / "r.txt")])
        self.assertEqual(rc, wm.EXIT_ERROR)


if __name__ == "__main__":
    unittest.main()
