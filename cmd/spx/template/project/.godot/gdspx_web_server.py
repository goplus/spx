#!/usr/bin/env python3

import argparse
import contextlib
import ipaddress
import json
import os
import socket
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


DEFAULT_PORT = 8060
DEFAULT_HOST = "127.0.0.1"
DEFAULT_AI_TIMEOUT_SECONDS = 60.0
AI_PROXY_PREFIX = "/api/ai/interaction"
AI_PROXY_ACTIONS = {"turn", "archive"}

CROSS_ORIGIN_HEADERS = {
    "Cross-Origin-Embedder-Policy": "require-corp",
    "Cross-Origin-Opener-Policy": "same-origin",
}

NO_CACHE_HEADERS = {
    "Cache-Control": "no-store, no-cache, must-revalidate",
    "Pragma": "no-cache",
    "Expires": "0",
}


class DualStackServer(ThreadingHTTPServer):
    daemon_threads = True

    def server_bind(self):
        # Allow one server socket to accept IPv4 and IPv6 connections where supported.
        with contextlib.suppress(Exception):
            self.socket.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 0)
        return super().server_bind()


class OriginAccessError(Exception):
    pass


class SPXRequestHandler(SimpleHTTPRequestHandler):
    def end_headers(self):
        for name, value in CROSS_ORIGIN_HEADERS.items():
            self.send_header(name, value)
        allowed_origin = self.allowed_origin()
        if allowed_origin:
            self.send_header("Access-Control-Allow-Origin", allowed_origin)
            self.send_header("Vary", "Origin")
        for name, value in NO_CACHE_HEADERS.items():
            self.send_header(name, value)
        super().end_headers()

    def do_OPTIONS(self):
        if self.ai_proxy_action() is not None:
            try:
                self.require_same_origin_browser_request()
            except OriginAccessError as err:
                self.respond_json(403, {"error": str(err)})
                return
        self.send_response(204)
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        self.end_headers()

    def do_POST(self):
        action = self.ai_proxy_action()
        if action is None:
            self.send_error(404, "Unknown POST endpoint")
            return
        self.handle_ai_proxy(action)

    def ai_proxy_action(self):
        path = urllib.parse.urlparse(self.path).path
        prefix = AI_PROXY_PREFIX + "/"
        if not path.startswith(prefix):
            return None
        action = path[len(prefix):]
        if action in AI_PROXY_ACTIONS:
            return action
        return None

    def handle_ai_proxy(self, action):
        endpoint = configured_ai_endpoint()
        if not endpoint:
            self.respond_json(
                503,
                {
                    "error": (
                        "AI interaction proxy is not configured. "
                        "Set SPX_AI_INTERACTION_ENDPOINT to a Builder-compatible backend."
                    )
                },
            )
            return

        try:
            self.require_same_origin_browser_request()
            payload = self.read_json_body()
            self.proxy_ai_request(endpoint, action, payload)
        except OriginAccessError as err:
            self.respond_json(403, {"error": str(err)})
        except UnicodeDecodeError:
            self.respond_json(400, {"error": "Request body must be valid UTF-8"})
        except json.JSONDecodeError as err:
            self.respond_json(400, {"error": f"Invalid JSON request body: {err}"})
        except ValueError as err:
            self.respond_json(400, {"error": str(err)})
        except urllib.error.HTTPError as err:
            self.respond_raw(err.code, content_type_from_headers(err.headers), err.read())
        except urllib.error.URLError as err:
            self.respond_json(502, {"error": f"Failed to reach AI backend: {err.reason}"})
        except Exception as err:
            self.respond_json(500, {"error": str(err)})

    def read_json_body(self):
        raw_length = self.headers.get("Content-Length", "0")
        try:
            length = int(raw_length)
        except ValueError:
            raise ValueError("Invalid Content-Length header")

        if length <= 0:
            return {}

        data = self.rfile.read(length)
        return json.loads(data.decode("utf-8"))

    def allowed_origin(self):
        origin = self.headers.get("Origin", "").strip()
        if not origin:
            return None

        parsed = urllib.parse.urlparse(origin)
        if parsed.scheme not in {"http", "https"}:
            return None
        if not parsed.netloc or parsed.path not in {"", "/"}:
            return None
        if parsed.params or parsed.query or parsed.fragment:
            return None

        normalized_origin = f"{parsed.scheme}://{parsed.netloc}"
        if parsed.netloc != self.headers.get("Host", "").strip():
            return None
        return normalized_origin

    def require_same_origin_browser_request(self):
        if not server_allows_ai_proxy(self.server):
            raise OriginAccessError("AI interaction proxy is only available on loopback servers")

        origin = self.headers.get("Origin", "").strip()
        if not origin:
            raise OriginAccessError("AI interaction proxy requires a same-origin Origin header")
        if self.allowed_origin() is None:
            raise OriginAccessError("AI interaction proxy only accepts same-origin browser requests")

    def proxy_ai_request(self, endpoint, action, payload):
        target = ai_action_url(endpoint, action)
        body = json.dumps(payload).encode("utf-8")
        request = urllib.request.Request(
            target,
            data=body,
            headers=self.ai_proxy_headers(),
            method="POST",
        )
        with urllib.request.urlopen(request, timeout=ai_request_timeout()) as response:
            self.respond_raw(response.status, content_type_from_headers(response.headers), response.read())

    def ai_proxy_headers(self):
        headers = {"Content-Type": "application/json"}

        token = os.environ.get("SPX_AI_INTERACTION_TOKEN", "").strip()
        if token:
            headers["Authorization"] = "Bearer " + token
            return headers

        incoming_auth = self.headers.get("Authorization")
        if incoming_auth:
            headers["Authorization"] = incoming_auth
        return headers

    def respond_json(self, status, payload):
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.respond_raw(status, "application/json", body)

    def respond_raw(self, status, content_type, body):
        if isinstance(body, str):
            body = body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", content_type or "application/octet-stream")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        # Keep CLI output quiet; spx reports server lifecycle messages itself.
        pass


def configured_ai_endpoint():
    return os.environ.get("SPX_AI_INTERACTION_ENDPOINT", "").strip()


def ai_action_url(endpoint, action):
    return endpoint.rstrip("/") + "/" + action


def ai_request_timeout():
    value = os.environ.get("SPX_AI_TIMEOUT", "").strip()
    if value:
        with contextlib.suppress(ValueError):
            return float(value)
    return DEFAULT_AI_TIMEOUT_SECONDS


def content_type_from_headers(headers):
    if headers is None:
        return "application/octet-stream"
    return headers.get_content_type()


def is_loopback_host(host):
    if not host:
        return False
    if host == "localhost":
        return True
    if host.startswith("[") and host.endswith("]"):
        host = host[1:-1]
    if "%" in host:
        host = host.split("%", 1)[0]
    with contextlib.suppress(ValueError):
        return ipaddress.ip_address(host).is_loopback
    return False


def server_allows_ai_proxy(server):
    return bool(getattr(server, "spx_ai_proxy_loopback_only", False))


def open_in_browser(url):
    if sys.platform == "win32":
        os.startfile(url)
        return

    opener = "open" if sys.platform == "darwin" else "xdg-open"
    subprocess.call([opener, url])


def serve_url(host, port):
    if host in {"", "0.0.0.0", "::"}:
        host = DEFAULT_HOST
    if ":" in host and not host.startswith("["):
        host = f"[{host}]"
    return f"http://{host}:{port}"


def serve(root, host, port, open_browser):
    url = serve_url(host, port)
    handler = partial(SPXRequestHandler, directory=str(root))

    with DualStackServer((host, port), handler) as httpd:
        httpd.spx_ai_proxy_loopback_only = is_loopback_host(host)
        print(f"Serving {root} at: {url}")
        if open_browser:
            print(f"Opening the served URL in the default browser: {url}")
            open_in_browser(url)

        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nKeyboard interrupt received, stopping server.")


def resolve_root(script_dir, root):
    if root.is_absolute():
        return root
    return script_dir / root


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", help="address to listen on", default=DEFAULT_HOST)
    parser.add_argument("-p", "--port", help="port to listen on", default=DEFAULT_PORT, type=int)
    parser.add_argument("-r", "--root", help="path to serve as root", default=".", type=Path)

    browser_parser = parser.add_mutually_exclusive_group(required=False)
    browser_parser.add_argument(
        "-n",
        "--no-browser",
        help="do not open the default web browser automatically",
        dest="browser",
        action="store_false",
    )
    parser.set_defaults(browser=True)
    return parser.parse_args()


def main():
    args = parse_args()
    script_dir = Path(__file__).resolve().parent
    root = resolve_root(script_dir, args.root).resolve()
    serve(root, args.host, args.port, args.browser)


if __name__ == "__main__":
    main()
