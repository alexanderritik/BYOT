"""
BYOT support module, loaded next to the customer's locustfile.

- Fits the run inside the test timeout: with no run time set it picks one
  that fits, and a run time that cannot fit fails at once with a clear message
  instead of being killed mid-run with no summary.
- Adds pass/fail thresholds, settable as CLI flags, LOCUST_* env vars or
  locust.conf keys:
    --max-p95-ms      LOCUST_MAX_P95_MS      p95 response time, ms
    --max-avg-ms      LOCUST_MAX_AVG_MS      average response time, ms
    --max-fail-ratio  LOCUST_MAX_FAIL_RATIO  allowed failed-request ratio (0.01 = 1%)
    --min-rps         LOCUST_MIN_RPS         minimum requests per second
  Without --max-fail-ratio, any failed request fails the run (Locust's default).
"""

import os
import sys
import time

from locust import events

DEFAULT_RUN_TIME = 30  # seconds, when nothing else is set
GRACE = 5  # seconds kept for Locust to stop and print its summary


def _log(msg):
    print(f"[byot] {msg}", file=sys.stderr, flush=True)


@events.init_command_line_parser.add_listener
def _add_arguments(parser):
    group = parser.add_argument_group("BYOT checks")
    group.add_argument("--max-p95-ms", type=float, env_var="LOCUST_MAX_P95_MS",
                       help="Fail if p95 response time exceeds this (ms)")
    group.add_argument("--max-avg-ms", type=float, env_var="LOCUST_MAX_AVG_MS",
                       help="Fail if average response time exceeds this (ms)")
    group.add_argument("--max-fail-ratio", type=float, env_var="LOCUST_MAX_FAIL_RATIO",
                       help="Fail if the failed-request ratio exceeds this (0.01 = 1%%)")
    group.add_argument("--min-rps", type=float, env_var="LOCUST_MIN_RPS",
                       help="Fail if requests per second fall below this")


def _remaining_seconds():
    timeout = int(os.environ.get("BYOT_TIMEOUT_SECONDS", "0") or 0)
    if timeout <= 0:
        return None
    started = float(os.environ.get("BYOT_STARTED_AT", "0") or 0) or time.time()
    return int(timeout - (time.time() - started) - GRACE)


def _abort(msg):
    _log(f"ERROR: {msg}")
    # os._exit: Locust would otherwise catch SystemExit and carry on.
    sys.stderr.flush()
    os._exit(1)


@events.init.add_listener
def _check_host(environment, **_kwargs):
    if environment.host or all(u.host for u in environment.user_classes):
        return
    _abort("no target host. Set the LOCUST_HOST env var on the test "
           "(e.g. https://api.example.com) or a host attribute on your User class.")


@events.init.add_listener
def _fit_run_time(environment, **_kwargs):
    opts = environment.parsed_options
    if opts is None:
        return
    remaining = _remaining_seconds()
    timeout = os.environ.get("BYOT_TIMEOUT_SECONDS")

    if not opts.run_time:
        if remaining is None:
            opts.run_time = DEFAULT_RUN_TIME
        else:
            opts.run_time = max(1, min(DEFAULT_RUN_TIME, remaining))
        _log(f"no run time set, running for {opts.run_time}s "
             f"(set LOCUST_RUN_TIME to change)")
        return

    if remaining is not None and opts.run_time > remaining:
        _abort(f"run time {opts.run_time}s does not fit in the test timeout "
               f"({timeout}s, about {remaining}s left after startup). Raise the test "
               f"timeout to at least {opts.run_time + (int(timeout) - remaining)}s "
               f"or shorten LOCUST_RUN_TIME.")


@events.quitting.add_listener
def _apply_checks(environment, **_kwargs):
    opts = environment.parsed_options
    stats = environment.stats.total
    if opts is None:
        return

    checks = []
    if opts.max_p95_ms is not None:
        p95 = stats.get_response_time_percentile(0.95) or 0
        checks.append((f"p95 {p95:.0f}ms <= {opts.max_p95_ms:g}ms", p95 <= opts.max_p95_ms))
    if opts.max_avg_ms is not None:
        avg = stats.avg_response_time
        checks.append((f"avg {avg:.0f}ms <= {opts.max_avg_ms:g}ms", avg <= opts.max_avg_ms))
    if opts.max_fail_ratio is not None:
        ratio = stats.fail_ratio
        checks.append((f"fail ratio {ratio:.2%} <= {opts.max_fail_ratio:.2%}",
                       ratio <= opts.max_fail_ratio))
    if opts.min_rps is not None:
        rps = stats.total_rps
        checks.append((f"rps {rps:.2f} >= {opts.min_rps:g}", rps >= opts.min_rps))

    if stats.num_requests == 0:
        checks.append(("at least one request was made", False))

    for label, ok in checks:
        _log(f"{'PASS' if ok else 'FAIL'} {label}")

    if not all(ok for _, ok in checks):
        environment.process_exit_code = 1
    elif opts.max_fail_ratio is not None and environment.process_exit_code is None:
        # Failures within the allowed ratio shouldn't fail the run.
        environment.process_exit_code = 0
