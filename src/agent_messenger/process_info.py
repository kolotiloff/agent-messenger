"""Record a launch. Do not decide from that record that the work stopped.

A missing PID, a reused PID, a start time, an empty process group, and a
changed boot id are recorded when the platform exposes them. None of those
facts is proof that every descendant has exited, and none of them proves the
agent did not already perform the message. ``process_exit_proven`` is the
only gate for a new launch after uncertainty, and it stays closed.
"""
from __future__ import annotations

import os
from pathlib import Path
import signal
import time


def _linux_start_ticks(pid):
    try:
        data = Path('/proc/%d/stat' % pid).read_text(encoding='utf-8')
    except OSError:
        return None
    end = data.rfind(')')
    if end < 0:
        return None
    fields = data[end + 2:].split()
    # /proc/<pid>/stat field 22 is starttime; field 3 is the first token after comm.
    try:
        return int(fields[19])
    except (IndexError, ValueError):
        return None


def _linux_start_unix(ticks):
    try:
        hz = os.sysconf('SC_CLK_TCK')
        boot = None
        for line in Path('/proc/stat').read_text(encoding='utf-8').splitlines():
            if line.startswith('btime '):
                boot = int(line.split()[1])
                break
        if boot is None or not hz:
            return None
        return boot + (ticks / float(hz))
    except (OSError, ValueError):
        return None


def read_boot_id():
    try:
        value = Path('/proc/sys/kernel/random/boot_id').read_text(encoding='utf-8').strip()
    except OSError:
        return None
    return value or None


def capture_process(pid):
    """Pid, process group, and start observation. Not a liveness proof."""
    record = {
        'pid': int(pid),
        'pgid': None,
        'supervisor_observed_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
        'linux_start_ticks': None,
        'linux_start_unix': None,
        'boot_id': read_boot_id(),
        'start_time_source': 'supervisor_clock',
        'exit_proof': False,
        'note': 'pid_start_and_group_are_not_proof_of_exit',
    }
    if hasattr(os, 'getpgid'):
        try:
            record['pgid'] = os.getpgid(pid)
        except OSError:
            record['pgid'] = None
    ticks = _linux_start_ticks(pid)
    if ticks is not None:
        record['linux_start_ticks'] = ticks
        record['start_time_source'] = 'linux_proc_stat'
        record['linux_start_unix'] = _linux_start_unix(ticks)
    return record


def process_exit_proven(_record=None):
    """Reliable process-tree exit proof is not implemented. Always False."""
    return False


def signal_process_group(pgid, own_pgid=None):
    """Best-effort SIGTERM to a recorded group. The return value is not proof."""
    if own_pgid is None and hasattr(os, 'getpgrp'):
        try:
            own_pgid = os.getpgrp()
        except OSError:
            own_pgid = None
    if not isinstance(pgid, int) or pgid <= 1:
        return 'not_signaled_missing_or_unsafe_group'
    if own_pgid is not None and pgid == own_pgid:
        return 'not_signaled_own_group'
    if not hasattr(os, 'killpg'):
        return 'not_signaled_unsupported'
    try:
        os.killpg(int(pgid), signal.SIGTERM)
    except ProcessLookupError:
        return 'group_missing_not_proof'
    except OSError:
        return 'signal_failed_not_proof'
    return 'sigterm_sent_not_proof'
