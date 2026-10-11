#!/usr/bin/env python3
"""Verify generic tasks with a fixed project, local model and real CLI/PTY.

The model is deterministic and checks actual tool evidence before continuing.
This proves program behavior, not a real model's reasoning effectiveness.
No credentials, remote services or personal configuration are used.
"""
import argparse
import http.server
import json
import os
import pathlib
import tempfile
import termios
import threading
import time

from verify_terminal import MockModel, Terminal


REQUESTS = {
    "route": "帮我解释PATH=/bin; touch ROUTE_MUST_NOT_RUN",
    "route_path": "请解释/tmp/file",
    "route_var": "帮我解释$PATH=/bin",
    "understand": "解释这个项目嵌套目录中的 entrypoint，并给出文件与行号。",
    "diagnose": "启动报错 expected service_port=9100，定位配置原因并运行验证。",
    "machine": "查看模拟机器的端口占用，取得实际状态后提出处理。",
    "modify": "把 service_port 从 9000 改为 9100，查看差异并验证。",
    "failed": "验证项目；失败时说明失败依据，不能宣称修复成功。",
    "boundary": "检查忽略文件、越界和特殊文件是否阻止自动读取。",
    "cancel": "查找 entrypoint，然后等待继续分析以测试取消。",
}
SOURCE = "src/nested/service.sh"
SEARCH = {"path": "src", "query": "entrypoint"}
PORT_SEARCH = {"path": "src", "query": "service_port="}
RANGE = {"path": SOURCE, "start_line": 119, "line_count": 5}
VERIFY = f"bash -n {SOURCE} && grep -q '^service_port=9100$' {SOURCE} && printf 'TASK_VERIFY_OK\\n'"


class TaskModel(MockModel):
    errors = []
    requests = []
    proposals = []
    waiting = threading.Event()

    def do_POST(self):
        try:
            # The fixture must not inherit a credential from the parent shell.
            assert "Authorization" not in self.headers, "mock received an unexpected credential header"
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            self.requests.append(body)
            request_index = len(self.requests) - 1
            assert body.get("tools"), "task unexpectedly used classifier"
            messages = body["messages"]
            initial = json.loads(messages[1]["content"])
            request = initial["request"]
            scenario = "diagnose" if request.startswith("Diagnose the last user-issued shell command") else next(key for key, text in REQUESTS.items() if text == request)
            runtime = initial["runtime_context"]
            assert runtime["project_root"] == runtime["cwd"], "fixture scope changed"
            assert set(runtime["tools"]) == {"list_files", "read_file", "search_text", "read_file_range", "run_command"}
            results = [json.loads(m["content"]) for m in messages if m["role"] == "tool"]
            # Prior command diagnostics are synthetic tool data, never user text.
            if scenario == "diagnose":
                assert results[0]["exit_code"] == 1 and "expected service_port=9100" in results[0]["output"]
                results = results[1:]
            answer = self.decide(scenario, results)
            for call in answer.get("tool_calls", []):
                if call["function"]["name"] == "run_command":
                    self.proposals.append(request_index)
            payload = json.dumps({"choices": [{"message": answer, "finish_reason": "tool_calls" if answer.get("tool_calls") else "stop"}]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            try:
                self.wfile.write(payload)
            except (BrokenPipeError, ConnectionResetError):
                pass
        except Exception as exc:
            self.errors.append(f"{type(exc).__name__}: {exc}")
            self.send_error(500, "mock scenario evidence failed")

    def decide(self, scenario, results):
        step = len(results)
        if scenario.startswith("route"):
            assert step == 0
            return self.completed("TASK_ROUTE_PREFIX: 已进入Agent，未执行输入。")
        if scenario in {"understand", "diagnose", "modify", "cancel"}:
            if step == 0:
                return self.tool("search", "search_text", **(PORT_SEARCH if scenario == "diagnose" else SEARCH))
            found = results[0]
            expected_line = 121 if scenario == "diagnose" else 122
            assert found["matches"] == [{"path": SOURCE, "line": expected_line, "text": "service_port=9000" if scenario == "diagnose" else 'entrypoint() { printf "%s\\n" "$service_port"; }'}]
            assert not found["truncated"]
            if step == 1:
                return self.tool("range", "read_file_range", **RANGE)
            fragment = results[1]
            assert fragment["start_line"] == 119 and fragment["end_line"] == 122
            assert "service_port=9000" in fragment["content"] and "entrypoint()" in fragment["content"]
            assert not fragment["truncated"]
            if scenario == "understand":
                return self.completed(f"TASK_UNDERSTAND: {SOURCE}:122 定义 entrypoint，读取119至122行作为依据；未执行命令。")
            if scenario == "cancel":
                self.waiting.set()
                time.sleep(1.5)
                return self.completed("CANCELED_RESPONSE_MUST_NOT_APPEAR")
            if scenario == "diagnose":
                if step == 2:
                    return self.tool("validate", "run_command", command=f"bash -n {SOURCE} && grep '^service_port=' {SOURCE}", purpose="验证脚本语法并确认报错对应的实际配置")
                assert results[2]["exit_code"] == 0 and "service_port=9000" in results[2]["output"]
                return self.completed(f"TASK_DIAGNOSE: {SOURCE}:121 为9000，与报错期待9100不符；语法验证已执行通过，配置修改尚未执行。")
            if step == 2:
                return self.tool("modify", "run_command", command=f"sed -i 's/^service_port=9000$/service_port=9100/' {SOURCE} && printf 'TASK_EDITED\\n'", purpose="修改已定位的配置")
            assert results[2]["exit_code"] == 0 and "TASK_EDITED" in results[2]["output"]
            if step == 3:
                return self.tool("diff", "run_command", command=f"diff -u baseline.sh {SOURCE}", purpose="查看实际修改差异")
            assert results[3]["exit_code"] == 1
            assert "-service_port=9000" in results[3]["output"] and "+service_port=9100" in results[3]["output"]
            if step == 4:
                return self.tool("verify", "run_command", command=VERIFY, purpose="检查Bash语法和修改后的端口值")
            assert results[4]["exit_code"] == 0 and "TASK_VERIFY_OK" in results[4]["output"]
            return self.completed("TASK_MODIFIED: 已修改端口，已查看实际差异，语法和配置验证通过。")
        if scenario == "machine":
            if step == 0:
                return self.tool("status", "run_command", command="cat machine-status.txt", purpose="读取固定模拟机器状态，确认实际占用证据")
            assert results[0]["exit_code"] == 0 and "LISTEN port=9000 pid=4242" in results[0]["output"]
            return self.tool("treat", "run_command", command="touch treatment-marker", purpose="模拟处理已观察到的端口占用；等待用户批准")
        if scenario == "failed":
            if step == 0:
                return self.tool("verify", "run_command", command="bash verify-fail.sh", purpose="执行固定失败验证脚本")
            assert results[0]["exit_code"] == 7 and "TASK_FAILURE_EVIDENCE" in results[0]["output"]
            return self.completed("TASK_VALIDATION_FAILED: 验证已执行，退出码7，TASK_FAILURE_EVIDENCE；修复尚未执行，验证失败。")
        if scenario == "boundary":
            paths = [".env.local", "ignored/hidden.txt", "../outside", "fifo"]
            if step > 0:
                assert "error" in results[-1], "boundary read unexpectedly succeeded"
            if step < len(paths):
                return self.tool(f"boundary-{step}", "read_file_range", path=paths[step])
            return self.completed("TASK_BOUNDARY: 自动读取全部拒绝；未执行命令。")
        raise AssertionError("unknown scenario")

    @staticmethod
    def completed(content):
        return {"role": "assistant", "content": content}


def scenario_requests(since):
    return TaskModel.requests[since:]


def report(scenario, since, confirmations, outcome):
    calls = scenario_requests(since)
    assert calls, "scenario had no decisions"
    assert not TaskModel.errors, TaskModel.errors
    observed = sum(index >= since for index in TaskModel.proposals)
    assert observed == confirmations, "unexpected number of command confirmations"
    print(f"PASS task {scenario}: decisions={len(calls)} confirmations={observed} result={outcome}")


def approve(tty):
    tty.confirm_ready()
    tty.send("y\r")


def after_output(tty, marker):
    tty.until(marker)
    offset = tty.data.index(marker.encode()) + len(marker.encode())
    tty.data = tty.data[offset:]


def verify(binary):
    TaskModel.requests.clear()
    TaskModel.errors.clear()
    TaskModel.proposals.clear()
    TaskModel.waiting.clear()
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), TaskModel)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="lingcell-tasks-") as temporary:
            root = pathlib.Path(temporary)
            (root / ".git").mkdir()
            (root / "src/nested").mkdir(parents=True)
            source = "# fixture\n" * 120 + 'service_port=9000\nentrypoint() { printf "%s\\n" "$service_port"; }\n'
            (root / SOURCE).write_text(source)
            (root / "baseline.sh").write_text(source)
            (root / ".bashrc").write_text("PROMPT_COMMAND=':'\n")
            (root / ".gitignore").write_text("ignored/\n")
            (root / "ignored").mkdir()
            (root / "ignored/hidden.txt").write_text("PRIVATE_TASK_SENTINEL")
            (root / ".env.local").write_text("PRIVATE_TASK_SENTINEL")
            (root / "machine-status.txt").write_text("LISTEN port=9000 pid=4242\n")
            (root / "verify-fail.sh").write_text("printf 'TASK_FAILURE_EVIDENCE\\n'\nexit 7\n")
            os.mkfifo(root / "fifo", 0o600)
            config = root / "config.json"
            config.write_text(json.dumps({"base_url": f"http://127.0.0.1:{server.server_port}/v1", "model": "mock-task", "api_key_env": "", "timeout_seconds": 5}))
            tty = Terminal(binary, root, config)
            try:
                tty.prompt()
                for scenario in ("route", "route_path", "route_var"):
                    before = len(TaskModel.requests)
                    tty.send(REQUESTS[scenario] + "\r")
                    tty.finish("TASK_ROUTE_PREFIX:")
                    assert not (root / "ROUTE_MUST_NOT_RUN").exists()
                    report(scenario, before, 0, "request prefix takes precedence over embedded paths/assignments")
                before = len(TaskModel.requests)
                # A local natural-language request uses the same Agent loop.
                tty.send(REQUESTS["understand"] + "\r")
                tty.finish("TASK_UNDERSTAND:")
                report("understand", before, 0, "nested entrypoint and cited lines")

                before = len(TaskModel.requests)
                # Diagnose starts with a genuine failed command result. The
                # mock derives its next decisions from that tool data.
                failed_command = "printf 'expected service_port=9100\\n'; false"
                tty.send(":sh " + failed_command + "\r")
                approve(tty)
                tty.finish("TASK_DIAGNOSE:")
                report("diagnose", before, 1, "failure data, candidate cause, actual verification")

                before = len(TaskModel.requests)
                tty.send(":ai " + REQUESTS["machine"] + "\r")
                approve(tty)
                after_output(tty, "LISTEN port=9000 pid=4242\r\n")
                tty.confirm_ready()
                assert not (root / "treatment-marker").exists()
                tty.send("n\r")
                tty.finish("command declined")
                assert not (root / "treatment-marker").exists()
                report("machine", before, 2, "observed status, treatment refused")

                before = len(TaskModel.requests)
                tty.send(":ai " + REQUESTS["modify"] + "\r")
                tty.confirm_ready()
                assert (root / SOURCE).read_text() == source, "modified before approval"
                tty.send("y\r")
                after_output(tty, "TASK_EDITED\r\n")
                approve(tty)
                after_output(tty, "+service_port=9100\r\n")
                approve(tty)
                tty.finish("TASK_MODIFIED:")
                assert (root / SOURCE).read_text() == source.replace("service_port=9000", "service_port=9100")
                report("modify", before, 3, "actual edit, diff and verification")

                before = len(TaskModel.requests)
                tty.send(":ai " + REQUESTS["failed"] + "\r")
                approve(tty)
                output = tty.until("TASK_VALIDATION_FAILED:")
                assert b"TASK_MODIFIED:" not in output
                tty.finish("TASK_VALIDATION_FAILED:")
                report("failed", before, 1, "exit 7 reported, no success claim")

                before = len(TaskModel.requests)
                tty.send(":ai " + REQUESTS["boundary"] + "\r")
                tty.finish("TASK_BOUNDARY:")
                assert "PRIVATE_TASK_SENTINEL" not in json.dumps(scenario_requests(before))
                report("boundary", before, 0, "excluded, ignored, traversal, FIFO denied")

                # Reset only the synthetic file so the cancellation scenario can
                # inspect the same original fixture independently of modification.
                (root / SOURCE).write_text(source)
                before = len(TaskModel.requests)
                tty.send(":ai " + REQUESTS["cancel"] + "\r")
                assert TaskModel.waiting.wait(3), "did not reach analysis after reading"
                time.sleep(.05)
                tty.send("\x03")
                tty.finish("model request canceled")
                assert b"CANCELED_RESPONSE_MUST_NOT_APPEAR" not in tty.data
                tty.command("printf 'TASK_CANCEL_RECOVERED\\n'", "TASK_CANCEL_RECOVERED\r\n")
                report("cancel", before, 0, "Ctrl-C stops task and restores prompt")
                tty.send(":exit\r")
                assert tty.process.wait(3) == 0
                assert termios.tcgetattr(tty.slave) == tty.original
                history = (root / ".cache/lingcell/history").read_text()
                assert "PRIVATE_TASK_SENTINEL" not in history and "TASK_MODIFIED:" not in history
                assert "sed -i" not in history and "TASK_FAILURE_EVIDENCE" not in history
                print("PASS generic task evidence, approval boundary, history and terminal restoration")
            finally:
                tty.close()
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=pathlib.Path, default=pathlib.Path("bin/lingcell"))
    args = parser.parse_args()
    try:
        verify(args.binary.resolve())
    except Exception:
        print("FAIL generic task acceptance; required scenarios incomplete", flush=True)
        raise
