"""
Burst / spike load shape — 30s, 5 đợt sóng để xem server chịu tải spike thế nào.

Timeline (tổng 30s):
    0-5s:    100 users  (baseline)
    5-10s:   500 users  (spike #1)
    10-15s:  100 users  (recover)
    15-20s:  800 users  (spike #2, lớn hơn)
    20-25s:  100 users  (recover)
    25-30s: 1200 users  (spike #3, peak)

Usage:
    locust -f tests/burst.py --processes -1 --headless \\
        --host http://localhost:8080 -t 35s --only-summary

Lưu ý: KHÔNG truyền -u / -r khi dùng LoadTestShape — shape tự điều khiển.
"""

from locust import LoadTestShape

# Re-export user class từ locustfile chính để Locust pick up
from locustfile import URLShortenerUser  # noqa: F401


class BurstShape(LoadTestShape):
    # (duration_seconds_from_start, target_users, spawn_rate)
    stages = [
        (5, 100, 100),
        (10, 500, 200),
        (15, 100, 200),
        (20, 800, 300),
        (25, 100, 300),
        (30, 1200, 400),
    ]

    def tick(self):
        run_time = self.get_run_time()
        for end, users, rate in self.stages:
            if run_time < end:
                return users, rate
        return None
