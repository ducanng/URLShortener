"""
URL Shortener — Stress Test with Locust

Usage:
    locust -f tests/locustfile.py --host http://localhost:8080

Scenarios (chạy từ Locust Web UI tại http://localhost:8089):
    Warm-up:  10 users,  ramp 2/s,  3 min
    Load:    100 users, ramp 10/s, 10 min
    Stress:  500 users, ramp 50/s, 10 min
    Spike:  1000 users, instant,    2 min
"""

import random
import string

from locust import HttpUser, between, task


# Pool chung giữa tất cả users — thread-safe vì GIL
_shared_paths: list[str] = []


class URLShortenerUser(HttpUser):
    wait_time = between(0.5, 2)  # mỗi user chờ 0.5-2s giữa các request

    def on_start(self):
        """Tạo vài URL ban đầu để có data cho read tests."""
        for _ in range(3):
            self._create_url()

    def _random_url(self):
        suffix = "".join(random.choices(string.ascii_lowercase, k=8))
        return f"https://example.com/{suffix}"

    def _create_url(self):
        url = self._random_url()
        with self.client.post(
            "/shorted",
            json={"url": url},
            catch_response=True,
            name="POST /shorted",
        ) as resp:
            if resp.status_code == 200:
                try:
                    data = resp.json()
                    # grpc-gateway trả proto JSON: {"url": {"shortenedURL": "http://…/Pedyu"}}
                    shortened = data.get("url", {}).get("shortenedURL", "")
                    if shortened:
                        # Extract path: http://localhost:8080/Pedyu → Pedyu
                        path = shortened.rstrip("/").split("/")[-1]
                        if path:
                            _shared_paths.append(path)
                    resp.success()
                except Exception:
                    resp.success()
            else:
                resp.failure(f"Status {resp.status_code}")

    def _get_random_path(self):
        """Lấy ngẫu nhiên 1 path từ pool các URL đã tạo thành công."""
        if _shared_paths:
            return random.choice(_shared_paths)
        return None

    @task(3)
    def shorten_url(self):
        """POST /shorted — tạo short URL mới (write-heavy)."""
        self._create_url()

    @task(5)
    def redirect_url(self):
        """GET /:path — redirect đến original URL (read-heavy, cache hit)."""
        path = self._get_random_path()
        if not path:
            return
        with self.client.get(
            f"/{path}",
            allow_redirects=False,  # không follow redirect, chỉ đo latency của server
            catch_response=True,
            name="GET /:path (redirect)",
        ) as resp:
            if resp.status_code in (301, 302):
                resp.success()
            elif resp.status_code == 404:
                resp.failure("URL not found")
            else:
                resp.failure(f"Status {resp.status_code}")

    @task(2)
    def get_info(self):
        """GET /info/:path — lấy thông tin URL (read, có thể cache miss)."""
        path = self._get_random_path()
        if not path:
            return
        with self.client.get(
            f"/info/{path}",
            catch_response=True,
            name="GET /info/:path",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Status {resp.status_code}")
