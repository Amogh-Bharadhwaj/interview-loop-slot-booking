#!/usr/bin/env python3
"""Static file server + /api/* reverse proxy for the slot-booking-fe app.

Serves this folder's static files and forwards any request under /api/ to
the Go backend (stripping the /api prefix), so the browser only ever talks
to one origin and no CORS headers are needed on the backend.

Usage:
    python3 server.py
    BACKEND_URL=http://localhost:9090 PORT=3000 python3 server.py
"""
import os
import urllib.error
import urllib.request
from http.server import HTTPServer, SimpleHTTPRequestHandler

BACKEND_URL = os.environ.get("BACKEND_URL", "http://localhost:8080").rstrip("/")
PORT = int(os.environ.get("PORT", "5500"))
HOP_BY_HOP = {
    "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
    "te", "trailers", "transfer-encoding", "upgrade", "host", "content-length",
}


class Handler(SimpleHTTPRequestHandler):
    def do_GET(self):
        self._handle()

    def do_POST(self):
        self._handle()

    def do_PATCH(self):
        self._handle()

    def do_DELETE(self):
        self._handle()

    def do_PUT(self):
        self._handle()

    def _handle(self):
        if self.path.startswith("/api/") or self.path == "/api":
            self._proxy()
        else:
            super().do_GET() if self.command == "GET" else self.send_error(405)

    def _proxy(self):
        target = BACKEND_URL + self.path[len("/api"):]
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length) if length else None

        headers = {k: v for k, v in self.headers.items() if k.lower() not in HOP_BY_HOP}
        req = urllib.request.Request(target, data=body, headers=headers, method=self.command)
        try:
            with urllib.request.urlopen(req) as resp:
                self._relay(resp.status, resp.getheaders(), resp.read())
        except urllib.error.HTTPError as e:
            self._relay(e.code, e.headers.items(), e.read())
        except urllib.error.URLError as e:
            body = f'{{"error":"backend unreachable at {BACKEND_URL}: {e.reason}"}}'.encode()
            self._relay(502, [("Content-Type", "application/json")], body)

    def _relay(self, status, headers, body):
        self.send_response(status)
        for k, v in headers:
            if k.lower() not in HOP_BY_HOP:
                self.send_header(k, v)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        print(f"[{self.log_date_time_string()}] {self.command} {self.path}")


if __name__ == "__main__":
    os.chdir(os.path.dirname(os.path.abspath(__file__)))
    print(f"slot-booking-fe: http://localhost:{PORT}  (proxying /api/* -> {BACKEND_URL})")
    HTTPServer(("localhost", PORT), Handler).serve_forever()
