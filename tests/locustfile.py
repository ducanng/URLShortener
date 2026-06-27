"""
URL Shortener — Realistic Load Test with Locust

Key optimizations để load generator KHÔNG trở thành bottleneck:
  1. FastHttpUser (geventhttpclient) — nhanh ~5-6x so với HttpUser (requests)
  2. Multi-process: dùng `--processes -1` để tận dụng mọi core (bypass GIL)
  3. Bounded _shared_paths (deque maxlen) — không phình vô hạn
  4. Bỏ catch_response khi không cần — Locust tự đếm status code
  5. wait_time mô phỏng user thực (constant_throughput hoặc between dài hơn)
  6. Reuse connection: FastHttpSession đã keep-alive sẵn

Usage (single-host, multi-process — tận dụng đa core):
    locust -f tests/locustfile.py --processes -1 --host http://localhost:8080

Usage (distributed — load generator tách khỏi máy chạy server):
    # Trên máy A (master):
    locust -f tests/locustfile.py --master --host http://localhost:8080
    # Trên máy B/C/... (worker):
    locust -f tests/locustfile.py --worker --master-host=<A_IP>

Headless (CI / benchmark có thể script được):
    locust -f tests/locustfile.py --processes -1 --headless \\
        -u 500 -r 50 -t 10m --host http://localhost:8080 \\
        --csv=reports/run --html=reports/run.html

Scenarios gợi ý (chạy từ Locust Web UI tại http://localhost:8089):
    Warm-up:  10 users,  ramp 2/s,  3 min
    Load:    100 users, ramp 10/s, 10 min
    Stress:  500 users, ramp 50/s, 10 min
    Spike:  1000 users, instant,    2 min
"""

import random
import string
from collections import deque

from locust import FastHttpUser, between, task


# Pool chung giữa tất cả users trong cùng process.
# Dùng deque có maxlen để không phình bộ nhớ trong stress test dài.
# Lưu ý: ở chế độ multi-process / distributed, mỗi worker có pool riêng — chấp
# nhận được vì mỗi worker vẫn tự tạo URL trong on_start().
_SHARED_PATHS_MAX = 10_000
_shared_paths: deque[str] = deque(maxlen=_SHARED_PATHS_MAX)


class URLShortenerUser(FastHttpUser):
    # FastHttpUser tự keep-alive. wait_time giữa 1-3s mô phỏng user thực
    # tương tác với short URL (click, đợi, click tiếp). Muốn stress cao hơn,
    # tăng số user thay vì giảm wait_time — sát thực tế hơn.
    wait_time = between(1, 3)

    # Tăng connection pool & timeout để client không tự throttle
    network_timeout = 10.0
    connection_timeout = 10.0
    max_retries = 0  # không retry — đo đúng lỗi server

    def on_start(self):
        """Tạo vài URL ban đầu để có data cho read tests."""
        for _ in range(3):
            self._create_url()

    def _random_url(self):
        suffix = "".join(random.choices(string.ascii_lowercase, k=8))
        return f"https://example.com/{suffix}"

    def _create_url(self):
        url = self._random_url()
        # Bỏ catch_response: Locust tự đếm status 2xx = success, còn lại = fail.
        # Chỉ parse JSON nếu cần lấy path cho read tests.
        resp = self.client.post(
            "/shorted",
            json={"url": url},
            name="POST /shorted",
        )
        if resp.status_code != 200:
            return
        try:
            # grpc-gateway trả proto JSON: {"url": {"shortenedURL": "http://…/Pedyu"}}
            data = resp.json()
            shortened = data.get("url", {}).get("shortenedURL", "")
        except (ValueError, AttributeError):
            return
        if not shortened:
            return
        # Extract path cuối: http://localhost:8080/Pedyu → Pedyu
        path = shortened.rpartition("/")[-1]
        if path:
            _shared_paths.append(path)

    def _get_random_path(self):
        """Lấy ngẫu nhiên 1 path từ pool các URL đã tạo thành công."""
        if _shared_paths:
            return random.choice(_shared_paths)
        return None

    # Weight phản ánh traffic thực: read-heavy (~70% redirect, ~20% info, ~10% create).
    @task(1)
    def shorten_url(self):
        """POST /shorted — tạo short URL mới (write)."""
        self._create_url()

    @task(7)
    def redirect_url(self):
        """GET /:path — redirect đến original URL (read-heavy, cache hit)."""
        path = self._get_random_path()
        if not path:
            return
        # 301/302 là success path. catch_response chỉ dùng khi 404 cần đánh dấu fail
        # khác status mặc định — ở đây 404 sẽ tự bị tính là fail.
        with self.client.get(
            f"/{path}",
            allow_redirects=False,  # đo latency server, không follow redirect
            catch_response=True,
            name="GET /:path (redirect)",
        ) as resp:
            if resp.status_code in (301, 302):
                resp.success()
            else:
                resp.failure(f"Status {resp.status_code}")

    @task(2)
    def get_info(self):
        """GET /info/:path — lấy thông tin URL (read, có thể cache miss)."""
        path = self._get_random_path()
        if not path:
            return
        # Status 200 = success mặc định, để Locust tự xử lý.
        self.client.get(f"/info/{path}", name="GET /info/:path")