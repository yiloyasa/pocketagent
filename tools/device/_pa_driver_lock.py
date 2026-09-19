# -*- coding: utf-8 -*-
"""真机驱动单实例锁。

【为什么需要】历史踩坑：后台/超时被 detach 的驱动进程会活过发起它的那一轮，
继续抢输入事件（uiautomator dump / input tap），污染下一轮验证。

用法（在每个真机驱动脚本最开头）：
    import _pa_driver_lock
    _pa_driver_lock.acquire()      # 已有活着的驱动 → 直接退出(exit 2)

退出时（含异常/被中断）自动释放锁。
"""
import atexit
import os
import subprocess
import sys

LOCK = r"C:\Users\yanzi\_pa_driver.lock"


def _pid_alive(pid):
    try:
        r = subprocess.run(["tasklist", "/FI", "PID eq %d" % pid],
                           capture_output=True, text=True, timeout=15)
        return str(pid) in (r.stdout or "")
    except Exception:
        return False


def _read_pid():
    try:
        with open(LOCK) as f:
            return int((f.read() or "0").strip() or 0)
    except Exception:
        return 0


def acquire(tag="driver"):
    old = _read_pid()
    if old and old != os.getpid() and _pid_alive(old):
        sys.stderr.write("[lock] 已有真机驱动在跑 (pid=%d, tag=%s); 拒绝启动。\n" % (old, tag))
        sys.exit(2)
    with open(LOCK, "w") as f:
        f.write(str(os.getpid()))
    atexit.register(release)
    return True


def release():
    try:
        if os.path.exists(LOCK) and _read_pid() == os.getpid():
            os.remove(LOCK)
    except Exception:
        pass
