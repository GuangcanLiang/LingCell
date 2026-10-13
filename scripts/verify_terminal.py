#!/usr/bin/env python3
"""Exercise the built CLI through a real Linux PTY and a local mock model.

Defaults use only local mock HTTP and a synthetic bashrc. Explicit --ssh-target
and --bashrc options opt into a user-authorized remote host and personal config.
No provider credentials or third-party Python packages are required.
"""
import argparse
import errno
import fcntl
import http.server
import json
import os
import pathlib
import re
import select
import shutil
import shlex
import signal
import struct
import subprocess
import tempfile
import termios
import threading
import time


class MockModel(http.server.BaseHTTPRequestHandler):
    requests = []
    waiting = threading.Event()

    def log_message(self, *_):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        if not body.get("tools") and body["messages"][0]["content"].startswith("Extract at most"):
            payload = json.dumps({"choices": [{"message": {"role": "assistant", "content": '{"memories":[]}'}, "finish_reason": "stop"}]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
            return
        self.requests.append(body)
        messages = body["messages"]
        if not body.get("tools"):
            answer = {"role": "assistant", "content": '{"kind":"shell"}'}
        else:
            current = max(i for i, m in enumerate(messages) if m["role"] == "user" and "runtime_context" in json.loads(m["content"]))
            messages = [messages[0]] + messages[current:]
            request = json.loads(messages[1]["content"])["request"]
            if request.startswith("错误"):
                status = int(request.removeprefix("错误"))
                self.send_response(status)
                self.end_headers()
                self.wfile.write(b"PRIVATE_REMOTE_BODY")
                return
            if request == "协议错误":
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b"PRIVATE_INVALID_JSON")
                return
            results = [m for m in messages if m["role"] == "tool"]
            if "等待取消" in request:
                self.waiting.set()
                time.sleep(1.5)
                answer = {"role": "assistant", "content": "canceled response"}
            elif "拒绝" in request or request.startswith("Diagnose"):
                answer = self.tool("deny", "run_command", command="touch rejected-marker", purpose="verify refusal")
            elif len(results) == 0:
                answer = self.tool("list", "list_files", path=".")
            elif len(results) == 1:
                answer = self.tool("read", "read_file", path="README.md")
            elif len(results) == 2:
                answer = self.tool("run", "run_command", command="printf 'APPROVED_RESULT\\n'; export AGENT_TEST=ok", purpose="verify persistent execution")
            else:
                answer = {"role": "assistant", "content": "AGENT_FINISHED"}
        payload = json.dumps({"choices": [{"message": answer, "finish_reason": "tool_calls" if answer.get("tool_calls") else "stop"}]}).encode()
        try:
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
        except (BrokenPipeError, ConnectionResetError):
            pass

    @staticmethod
    def tool(id_, tool_name, **arguments):
        return {"role": "assistant", "tool_calls": [{"id": id_, "type": "function", "function": {"name": tool_name, "arguments": json.dumps(arguments)}}]}


class Terminal:
    def __init__(self, binary, root, config, private=False, flags=()):
        self.private = private
        self.master, self.slave = os.openpty()
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
        self.original = termios.tcgetattr(self.slave)
        self.data = b""
        env = dict(os.environ, HOME=str(root), XDG_CONFIG_HOME=str(root / ".config"), XDG_CACHE_HOME=str(root / ".cache"), XDG_STATE_HOME=str(root / ".state"), TERM="xterm-256color", PATH="/usr/bin:/bin")

        def setup():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)

        self.process = subprocess.Popen([str(binary), "--config", str(config), *flags], stdin=self.slave, stdout=self.slave, stderr=self.slave, cwd=root, env=env, preexec_fn=setup)

    def send(self, text):
        self.data = b""
        os.write(self.master, text.encode())

    def until(self, text, timeout=8):
        end = time.monotonic() + timeout
        expected = text.encode()
        while expected not in self.data:
            remaining = end - time.monotonic()
            if remaining <= 0:
                raise AssertionError("missing terminal marker (private session output omitted)" if self.private else f"missing {text!r}: {self.data[-2500:]!r}")
            if select.select([self.master], [], [], min(remaining, .1))[0]:
                try:
                    part = os.read(self.master, 65536)
                except OSError as exc:
                    if exc.errno == errno.EIO:
                        part = b""
                    else:
                        raise
                if not part:
                    raise AssertionError("terminal exited before expected marker (private session output omitted)" if self.private else f"terminal exited waiting for {text!r}: {self.data[-2500:]!r}")
                self.data += part
                # Answer readline's cursor-position query like a terminal emulator.
                if b"\x1b[6n" in part:
                    os.write(self.master, b"\x1b[1;1R")
        return self.data

    def prompt(self):
        self.until(" > ")
        marker = b" > "
        self.data = self.data[self.data.rindex(marker) + len(marker):]
        self.until("\x1b[?25h")
        time.sleep(.05)

    def finish(self, marker):
        self.until(marker)
        offset = self.data.index(marker.encode()) + len(marker.encode())
        self.data = self.data[offset:]
        self.prompt()

    def command(self, command, expected):
        self.send(command + "\r")
        self.finish(expected)

    def start_command(self, command):
        self.send(command + "\r")
        self.until("\x1b[0 q")
        self.data = b""
        time.sleep(.08)

    def confirm_ready(self):
        marker = "执行？[y/N]"
        self.until(marker)
        self.data = self.data[self.data.index(marker.encode()) + len(marker.encode()):]
        self.until("\x1b[?25h")

    def quiet(self, command):
        self.start_command(command)
        self.prompt()

    def close(self):
        if self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(3)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait()
        os.close(self.master)
        os.close(self.slave)


def verify_ssh(tty, target):
    # This bounded, read-only remote sh script has handshake points instead of
    # matching a remote prompt. SIGINT interrupts the remote sleep then returns
    # control to the same session; no remote file or persistent state is written.
    remote = "; ".join([
        "trap 'printf \"SSH_INTERRUPT_OK\\n\"' INT",
        "printf 'SSH_READY\\n'",
        "stty size",
        "IFS= read -r reply || exit 1",
        "stty size",
        "printf 'SSH_SLEEPING\\n'",
        "sleep 30",
        "printf 'SSH_AFTER_INTERRUPT\\n'",
        "IFS= read -r reply || exit 1",
        "exit 0",
    ])
    command = "ssh -tt -o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 -- " + shlex.quote(target) + " " + shlex.quote("sh -c " + shlex.quote(remote))
    tty.send(":sh " + command + "\r")
    tty.until("SSH_READY", timeout=15)
    tty.until("40 100\r\n")
    fcntl.ioctl(tty.slave, termios.TIOCSWINSZ, struct.pack("HHHH", 36, 110, 0, 0))
    os.kill(tty.process.pid, signal.SIGWINCH)
    time.sleep(.15)
    tty.send("continue\r")
    tty.until("36 110\r\n")
    tty.until("SSH_SLEEPING")
    time.sleep(.15)
    tty.send("\x03")
    tty.until("SSH_INTERRUPT_OK", timeout=5)
    tty.until("SSH_AFTER_INTERRUPT", timeout=5)
    tty.send("exit\r")
    tty.prompt()
    tty.command("printf 'SSH_LOCAL_INPUT_OK\\n'", "SSH_LOCAL_INPUT_OK\r\n")


def verify(binary, ssh_target=None, bashrc=None):
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), MockModel)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="lingcell-terminal-") as temporary:
            root = pathlib.Path(temporary)
            (root / "README.md").write_text("PRIVATE_FILE_CONTEXT\n")
            if bashrc is None:
                (root / ".bashrc").write_text("printf 'RC_LOADED\\n'\nalias personal='echo PERSONAL_ALIAS'\nPROMPT_COMMAND=':'\n")
            else:
                shutil.copyfile(bashrc, root / ".bashrc")
            config = root / "config.json"
            config.write_text(json.dumps({"base_url": f"http://127.0.0.1:{server.server_port}/v1", "model": "mock", "api_key_env": "UNSET_TEST_KEY", "timeout_seconds": 5}))
            tty = Terminal(binary, root, config, private=bool(bashrc or ssh_target))
            try:
                if bashrc is None:
                    tty.until("RC_LOADED")
                tty.prompt()
                if bashrc is None:
                    tty.command("personal", "PERSONAL_ALIAS\r\n")
                    tty.command("person\t", "PERSONAL_ALIAS\r\n")
                tty.command("basenam\t /tmp/file", "file\r\n")
                tty.command("cat READ\t", "PRIVATE_FILE_CONTEXT\r\n")
                tty.command("export LC_CLI=state; printf '%s\\n' \"$LC_CLI\"", "state\r\n")
                tty.send("echo 'first\r")
                tty.until("... ")
                tty.send("second'\r")
                tty.finish("first\r\nsecond\r\n")
                tty.start_command("sleep 30")
                tty.send("\x03")
                tty.prompt()
                tty.start_command("sleep 30")
                tty.send("\x1a")
                tty.finish("Stopped")
                tty.command("jobs", "Stopped")
                tty.command("bg; printf 'BG_OK\\n'", "BG_OK\r\n")
                tty.start_command("fg")
                tty.send("\x03")
                tty.prompt()
                tty.send(":sh stty size; sleep .4; stty size\r")
                tty.until("24 80\r\n")
                fcntl.ioctl(tty.slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 100, 0, 0))
                os.kill(tty.process.pid, signal.SIGWINCH)
                tty.finish("40 100\r\n")
                if shutil.which("vim"):
                    tty.send("vim -Nu NONE -i NONE vim.txt\r")
                    tty.until('"vim.txt"')
                    tty.send("iPTY_OK\x1b:wq\r")
                    tty.prompt()
                    assert (root / "vim.txt").read_text() == "PTY_OK\n"
                    print("PASS vim interactive edit and exit")
                else:
                    print("SKIP vim: executable unavailable")
                if shutil.which("top"):
                    tty.send("top\r")
                    tty.until("Tasks:")
                    tty.send("q")
                    tty.prompt()
                    print("PASS top interactive display and exit")
                else:
                    print("SKIP top: executable unavailable")
                if shutil.which("ssh"):
                    tty.command("ssh -V", "OpenSSH_")
                    print("PASS ssh local version command")
                else:
                    print("SKIP ssh local version: executable unavailable")
                if ssh_target:
                    if not shutil.which("ssh"):
                        raise AssertionError("authorized SSH check requires ssh executable")
                    verify_ssh(tty, ssh_target)
                    print("PASS remote SSH interaction, resize, interrupt and local input restoration")
                else:
                    print("SKIP remote SSH: no authorized --ssh-target")
                for request, expected in [("错误401", "model authentication failed"), ("错误429", "model rate limited"), ("错误503", "model service unavailable"), ("错误504", "model request timed out"), ("协议错误", "model protocol incompatible")]:
                    count = len(MockModel.requests)
                    tty.send(":ai " + request + "\r")
                    tty.until(expected)
                    assert b"PRIVATE_REMOTE_BODY" not in tty.data and b"PRIVATE_INVALID_JSON" not in tty.data
                    tty.finish(expected)
                    assert len(MockModel.requests) == count + 1, "unexpected model retry"
                    tty.command("printf 'ERROR_RECOVERED\\n'", "ERROR_RECOVERED\r\n")
                tty.send(":ai 请演示批准\r")
                tty.until("等待模型（第 1/8 轮）；Ctrl-C 取消")
                tty.until("读取文件")
                tty.until("等待确认")
                tty.confirm_ready()
                tty.send("y\r")
                tty.until("APPROVED_RESULT\r\n")
                tty.finish("AGENT_FINISHED")
                tty.command("echo \"$AGENT_TEST\"", "ok\r\n")
                tty.send(":ai 请演示拒绝\r")
                tty.confirm_ready()
                tty.send("n\r")
                tty.finish("command declined")
                assert not (root / "rejected-marker").exists()
                tty.send("false\r")
                tty.confirm_ready()
                tty.send("n\r")
                tty.finish("command declined")
                assert not (root / "rejected-marker").exists()
                count = len(MockModel.requests)
                tty.quiet("grep absent README.md")
                tty.quiet("test -f absent-file")
                assert len(MockModel.requests) == count
                tty.send("lss -la\r")
                tty.until("模型将模糊输入识别为命令")
                tty.confirm_ready()
                tty.send("n\r")
                tty.finish("command declined")
                tty.start_command(":ai 等待取消")
                assert MockModel.waiting.wait(3)
                tty.send("\x03")
                tty.until("model request canceled", timeout=2)
                tty.finish("model request canceled")
                # Move outside the fixed root and verify shell remains usable.
                tty.command("cd /tmp; printf 'CWD_OK\\n'", "CWD_OK\r\n")
                tty.command("pwd", "/tmp\r\n")
                tty.send(":exit\r")
                assert tty.process.wait(3) == 0
                assert termios.tcgetattr(tty.slave) == tty.original, "terminal mode was not restored"
                history = (root / ".cache/lingcell/history").read_text()
                assert "PRIVATE_FILE_CONTEXT" not in history and "AGENT_FINISHED" not in history
                assert "printf 'APPROVED_RESULT" not in history and ":ai" in history
                assert any(m.get("role") == "tool" for r in MockModel.requests for m in r["messages"])
                print("PASS session 1: shell state, multiline, jobs, resize, Agent tools, errors, history and terminal restoration")
            finally:
                tty.close()
            # Shell works without a provider, and failed model requests return
            # to the same usable Bash session.
            before = len(MockModel.requests)
            tty = Terminal(binary, root, root / "no-config.json", private=bool(bashrc))
            try:
                tty.prompt()
                tty.command("echo OFFLINE_OK", "OFFLINE_OK\r\n")
                tty.send(":ai 帮我查看项目\r")
                tty.finish("model configuration missing")
                tty.command("echo STILL_USABLE", "STILL_USABLE\r\n")
                tty.send(":exit\r")
                assert tty.process.wait(3) == 0
                assert termios.tcgetattr(tty.slave) == tty.original
                assert len(MockModel.requests) == before
                print("PASS session 2: offline model independence and terminal restoration")
            finally:
                tty.close()
            # A fresh process must not inherit state from either prior Bash.
            tty = Terminal(binary, root, config, private=bool(bashrc))
            try:
                tty.prompt()
                tty.command(":sh printf 'NEW_SESSION:%s\\n' \"${LC_CLI-unset}\"", "NEW_SESSION:unset\r\n")
                tty.command("export SESSION_THREE=ok; echo \"$SESSION_THREE\"", "ok\r\n")
                tty.send("echo 'session\r")
                tty.until("... ")
                tty.send("three'\r")
                tty.finish("session\r\nthree\r\n")
                tty.start_command("sleep 30")
                tty.send("\x1a")
                tty.finish("Stopped")
                tty.command("bg; printf 'SESSION_BG\\n'", "SESSION_BG\r\n")
                tty.start_command("fg")
                tty.send("\x03")
                tty.prompt()
                MockModel.waiting.clear()
                tty.start_command(":ai 等待取消")
                assert MockModel.waiting.wait(3)
                tty.send("\x03")
                tty.finish("model request canceled")
                tty.command("echo \"$SESSION_THREE\"", "ok\r\n")
                tty.send(":exit\r")
                assert tty.process.wait(3) == 0
                assert termios.tcgetattr(tty.slave) == tty.original
                print("PASS session 3: fresh Bash, state, multiline, job control, Agent cancellation and terminal restoration")
            finally:
                tty.close()
        if bashrc:
            print("PASS supplied bashrc loaded in three temporary CLI sessions")
        else:
            print("SKIP personal bashrc: synthetic config only; no authorized --bashrc")
        print("PASS required local CLI PTY checks (three independent sessions); optional checks reported separately")
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=pathlib.Path, default=pathlib.Path("bin/lingcell"))
    parser.add_argument("--ssh-target", help="authorized user@host; known host and existing noninteractive authentication required")
    parser.add_argument("--bashrc", type=pathlib.Path, help="authorized personal bashrc to load in temporary sessions (may execute local hooks)")
    args = parser.parse_args()
    try:
        verify(args.binary.resolve(), args.ssh_target, args.bashrc)
    except Exception:
        print("FAIL terminal acceptance; required checks did not complete", flush=True)
        raise
