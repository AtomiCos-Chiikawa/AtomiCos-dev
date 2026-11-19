#!/usr/bin/env python3
"""macOS stand-in for GNU `timeout` plus run.sh's watch_mem (no /proc on macOS).

    mac_timeout.py <duration> <memout-marker> <mem-limit-kb> <command...>

- Runs the command in its own process group (session), so the whole `go test` tree
  (go command, compiler/linker, test binary) can be found and killed together.
- Wall-clock limit (<duration>: 25m, 90s, 1h, or seconds): on expiry the group gets SIGTERM,
  then SIGKILL 5 s later; exit status 124, as GNU timeout.
- Memory: every 10 s (same period as watch_mem), each test binary of this group (command line
  contains ".test -test.paniconexit0", as watch_mem matched it) whose RSS is above the limit is
  killed with SIGKILL and its RSS (KB) is written to <memout-marker>. As with watch_mem, only the
  test binary is killed; `go test` then reports "signal: killed" and run.sh classifies MEMOUT.
  The compiler/linker are not watched (the linker alone peaks near 3 GB).
- If this wrapper is terminated (SIGTERM/SIGINT/SIGHUP), the group is killed too, so stopping
  run.sh leaves no orphaned test binaries.
- Otherwise exits with the command's status (128+signal if it died from a signal).
"""
import os
import signal
import subprocess
import sys
import time

MEM_PERIOD = 10.0
TEST_BINARY_MARK = ".test -test.paniconexit0"


def parse_duration(s):
    mult = {"s": 1, "m": 60, "h": 3600, "d": 86400}
    if s and s[-1] in mult:
        return float(s[:-1]) * mult[s[-1]]
    return float(s)


def group_processes(pgid):
    """[(pid, rss_kb, command)] of the processes in process group pgid."""
    try:
        out = subprocess.run(["ps", "-A", "-o", "pid=,pgid=,rss=,command="],
                             capture_output=True, text=True, timeout=30).stdout
    except Exception:
        return []
    procs = []
    for line in out.splitlines():
        parts = line.split(None, 3)
        if len(parts) < 4:
            continue
        try:
            pid, g, rss = int(parts[0]), int(parts[1]), int(parts[2])
        except ValueError:
            continue
        if g == pgid:
            procs.append((pid, rss, parts[3]))
    return procs


def kill_group(pgid, sig):
    try:
        os.killpg(pgid, sig)
    except (ProcessLookupError, PermissionError):
        pass


def main():
    if len(sys.argv) < 5:
        sys.stderr.write(__doc__)
        return 125
    limit = parse_duration(sys.argv[1])
    marker = sys.argv[2]
    mem_kb = int(sys.argv[3])
    cmd = sys.argv[4:]

    child = subprocess.Popen(cmd, start_new_session=True)
    pgid = child.pid

    def on_signal(signum, _frame):
        kill_group(pgid, signal.SIGTERM)
        time.sleep(2)
        kill_group(pgid, signal.SIGKILL)
        sys.exit(128 + signum)

    for s in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
        signal.signal(s, on_signal)

    t0 = time.monotonic()
    next_mem = t0 + MEM_PERIOD
    while True:
        try:
            rc = child.wait(timeout=1.0)
            break
        except subprocess.TimeoutExpired:
            pass
        now = time.monotonic()
        if now - t0 >= limit:
            kill_group(pgid, signal.SIGTERM)
            try:
                child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                pass
            kill_group(pgid, signal.SIGKILL)
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                pass
            return 124
        if now >= next_mem:
            next_mem = now + MEM_PERIOD
            for pid, rss, command in group_processes(pgid):
                if TEST_BINARY_MARK in command and rss > mem_kb:
                    with open(marker, "w") as f:
                        f.write("%d\n" % rss)
                    try:
                        os.kill(pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
    # the go command is gone; make sure nothing of its group survives
    kill_group(pgid, signal.SIGKILL)
    return 128 - rc if rc < 0 else rc


if __name__ == "__main__":
    sys.exit(main())
