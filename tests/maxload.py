"""
Ramp-to-fail: tăng dần users để TÌM giới hạn (max RPS, knee point, breakdown point).

Mục tiêu:
  - Tăng user tuyến tính 50 → 5000 trong 5 phút
  - Tự dừng khi đạt 1 trong các điều kiện fail:
      * Error rate > 5%
      * p95 latency > 2000ms
  - Stage cuối ngắn (10s) để dễ nhận biết "knee" trong CSV history

Usage:
    locust -f tests/maxload.py --processes -1 --headless \\
        --host http://localhost:8080 -t 6m \\
        --csv=reports/maxload --html=reports/maxload.html

    # Theo dõi real-time:
    tail -f reports/maxload_stats_history.csv | awk -F, '$3=="Aggregated"{print $1,$2,$5,$11,$13}'

Sau khi chạy: phân tích `reports/maxload_stats_history.csv` để tìm
RPS cao nhất trước khi p95 hoặc error rate vượt ngưỡng.
"""

import gevent
from locust import LoadTestShape, events
from locust.runners import MasterRunner

from locustfile import URLShortenerUser  # noqa: F401 — re-export user class


# Ngưỡng dừng sớm (early-stop) khi server "gãy"
MAX_ERROR_RATE = 0.05   # 5%
MAX_P95_MS = 2000.0     # 2s


class RampToFailShape(LoadTestShape):
    """
    Tăng tuyến tính từ start_users → max_users trong duration giây.
    Mỗi stage 10s để Locust có thời gian gom stats ổn định.
    """

    start_users = 50
    max_users = 5000
    step_users = 250         # mỗi stage tăng thêm 250 user
    step_duration = 10       # 10s mỗi stage
    spawn_rate = 100         # users/giây khi ramp

    def tick(self):
        run_time = self.get_run_time()
        stage_index = int(run_time // self.step_duration)
        users = self.start_users + stage_index * self.step_users
        if users > self.max_users:
            return None
        return users, self.spawn_rate


@events.init.add_listener
def _init(environment, **_kwargs):
    """Đăng ký watchdog dừng sớm khi server breakdown."""
    if isinstance(environment.runner, MasterRunner) or environment.runner is None:
        # Chỉ chạy watchdog ở local runner hoặc master (không chạy trên worker)
        pass

    def _watchdog():
        """Sliding-window watchdog: nhìn 10s gần nhất, không phải cumulative."""
        # Cần vượt ngưỡng 3 lần liên tiếp mới quit, tránh false positive do GC spike
        breach_streak = 0
        required_streak = 3

        while environment.runner is not None and environment.runner.state in ("running", "spawning"):
            gevent.sleep(5)
            stats = environment.runner.stats.total
            if stats.num_requests < 500:
                continue

            # Current = sliding window ~10s; total = cumulative từ đầu test
            err_rate = stats.num_failures / max(stats.num_requests, 1)
            p95_now = stats.get_current_response_time_percentile(0.95) or 0

            breached = err_rate > MAX_ERROR_RATE or p95_now > MAX_P95_MS
            if breached:
                breach_streak += 1
                print(
                    f"[WATCHDOG] breach {breach_streak}/{required_streak} "
                    f"(p95_10s={p95_now:.0f}ms, err={err_rate:.2%}, users={environment.runner.user_count})"
                )
            else:
                breach_streak = 0

            if breach_streak >= required_streak:
                reason = (
                    f"err {err_rate:.1%} > {MAX_ERROR_RATE:.0%}"
                    if err_rate > MAX_ERROR_RATE
                    else f"p95 {p95_now:.0f}ms > {MAX_P95_MS:.0f}ms"
                )
                print(f"\n[WATCHDOG] STOP after sustained breach: {reason}")
                environment.runner.quit()
                return

    gevent.spawn(_watchdog)
