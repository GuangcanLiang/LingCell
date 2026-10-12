#!/usr/bin/env python3
"""Verify session and memory lifecycle using a local model and real Linux PTYs."""
import argparse
import http.server
import json
import os
import pathlib
import sqlite3
import tempfile
import threading
import time

from verify_terminal import MockModel, Terminal

FIRST = "Please use Chinese answers and keep them short. Our task-specific code is ORCHID42."
FOLLOW = "Follow up using the task-specific code."
RESTORE = "Restore follow up using earlier code."
NEW = "New isolated conversation: search my language preference."
HISTORY = "Read this session history."
EPHEMERAL_MEMORY = "Check ephemeral memory search is disabled."
UNKNOWN = "Run a command then simulate abrupt CLI death."
REFUSE = "Refuse this proposed command."


class RuntimeModel(MockModel):
    errors = []
    decisions = 0
    extractions = 0

    def do_POST(self):
        try:
            assert "Authorization" not in self.headers, "unexpected mock credentials"
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            assert len(json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode()) <= 131072
            messages = body["messages"]
            if not body.get("tools"):
                assert messages[0]["content"].startswith("Extract at most"), "unexpected background model call"
                self.__class__.extractions += 1
                saved = json.loads(messages[1]["content"])
                turn = saved["saved_turn"]
                request = json.loads(turn["messages"][0]["content"])["request"]
                candidates = []
                if request == FIRST:
                    candidates = [{"scope": "user", "kind": "preference", "topic": "language", "content": "Use Chinese answers", "source": {"session": saved["session"], "turn": turn["number"], "message": 0, "quote": "Please use Chinese answers"}}]
                answer = self.completed(json.dumps({"memories": candidates}))
            else:
                self.__class__.decisions += 1
                index = max(i for i, m in enumerate(messages) if m["role"] == "user" and "runtime_context" in json.loads(m["content"]))
                request = json.loads(messages[index]["content"])["request"]
                results = [json.loads(m["content"]) for m in messages[index + 1:] if m["role"] == "tool"]
                earlier = json.dumps(messages[:index], ensure_ascii=False)
                # Check all protocol pairings, including failed historical calls.
                pending = []
                for m in messages:
                    if m.get("tool_calls"):
                        assert not pending, "unanswered earlier calls"
                        pending = [c["id"] for c in m["tool_calls"]]
                    elif m["role"] == "tool":
                        assert m["tool_call_id"] in pending, "orphan tool result"
                        pending.remove(m["tool_call_id"])
                    else:
                        assert not pending, "unfinished calls sent to model"
                assert not pending
                if request == FIRST:
                    answer = self.completed("RUNTIME_FIRST_OK")
                elif request in (FOLLOW, RESTORE):
                    assert FIRST in earlier and "ORCHID42" in earlier, "previous turn lost"
                    if request == RESTORE:
                        assert FOLLOW in earlier, "restored follow-up lost"
                    answer = self.completed("RUNTIME_FOLLOW_OK")
                elif request == NEW:
                    assert FIRST not in earlier and FOLLOW not in earlier, "new session leaked prior conversation"
                    if not results:
                        assert "Use Chinese answers" not in json.dumps(messages), "memory injected before search"
                        answer = self.tool("memory", "search_memory", query="Chinese", scope="user")
                    else:
                        assert len(results) == 1 and any(m["content"] == "Use Chinese answers" and m["status"] == "valid" for m in results[0])
                        assert all("owner" not in m for m in results[0]), "local identity sent"
                        answer = self.completed("RUNTIME_NEW_MEMORY_OK")
                elif request == HISTORY:
                    if not results:
                        answer = self.tool("history", "read_session_history", start_turn=1, count=1)
                    else:
                        assert results[0]["turns"][0]["status"] == "completed"
                        answer = self.completed("RUNTIME_HISTORY_OK")
                elif request == EPHEMERAL_MEMORY:
                    if not results:
                        answer = self.tool("memory", "search_memory", query="Chinese", scope="user")
                    else:
                        assert results[0]["status"] == "memory disabled in ephemeral mode"
                        answer = self.completed("RUNTIME_EPHEMERAL_MEMORY_DISABLED")
                elif request == UNKNOWN:
                    assert not results, "uncertain command was retried"
                    answer = self.tool("unknown", "run_command", command="printf 'once\\n' >> unknown-count; printf 'UNKNOWN_COMMAND_STARTED\\n'; sleep 3", purpose="test unknown result lifecycle")
                elif request == REFUSE:
                    answer = self.tool("refusal", "run_command", command="touch MUST_NOT_EXIST", purpose="test refusal")
                else:
                    raise AssertionError("unexpected request")
            payload = json.dumps({"choices": [{"message": answer, "finish_reason": "tool_calls" if answer.get("tool_calls") else "stop"}]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
        except Exception as exc:
            self.errors.append(f"{type(exc).__name__}: {exc}")
            self.send_error(500, "runtime fixture failed")

    @staticmethod
    def completed(content):
        return {"role": "assistant", "content": content}


def sessions(root):
    with sqlite3.connect(root / ".state/lingcell/runtime.db") as db:
        return list(db.execute("SELECT id FROM sessions ORDER BY created,rowid"))


def stop(tty):
    tty.send(":exit\r")
    assert tty.process.wait(3) == 0
    tty.close()


def start(binary, root, config, flags=()):
    tty = Terminal(binary, root, config, flags=flags)
    tty.prompt()
    return tty


def verify(binary):
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), RuntimeModel)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    live = []
    try:
        with tempfile.TemporaryDirectory(prefix="lingcell-runtime-") as directory:
            root = pathlib.Path(directory)
            (root / ".git").mkdir()
            (root / ".bashrc").write_text("PROMPT_COMMAND=':'\n")
            config = root / "config.json"
            config.write_text(json.dumps({"base_url": f"http://127.0.0.1:{server.server_port}/v1", "model": "mock-runtime", "api_key_env": "", "timeout_seconds": 5}))
            tty = start(binary, root, config); live.append(tty)
            first_id = sessions(root)[0][0]
            tty.command(":ai " + FIRST, "RUNTIME_FIRST_OK")
            tty.command(":ai " + FOLLOW, "RUNTIME_FOLLOW_OK")
            tty.command(":sh export LOCAL_RUNTIME_VAR=active; printf 'VAR_SET\\n'", "VAR_SET\r\n")
            tty.command(":context", "bytes；摘要覆盖")
            stop(tty); live.remove(tty)

            tty = start(binary, root, config, ("--resume", first_id)); live.append(tty)
            # Context restores, Bash environment does not.
            tty.command(":sh printf 'FRESH_BASH:%s\\n' \"${LOCAL_RUNTIME_VAR-unset}\"", "FRESH_BASH:unset\r\n")
            tty.command(":ai " + RESTORE, "RUNTIME_FOLLOW_OK")
            concurrent = Terminal(binary, root, config, flags=("--resume", first_id)); live.append(concurrent)
            concurrent.until("session is already in use")
            assert concurrent.process.wait(3) == 1
            concurrent.close(); live.remove(concurrent)
            other = root / "other-project"; other.mkdir(); (other / ".git").mkdir()
            # Preserve the same state dir across projects to test root matching.
            (other / ".state").symlink_to(root / ".state", target_is_directory=True)
            wrong = Terminal(binary, other, config, flags=("--resume", first_id)); live.append(wrong)
            wrong.until("session project root differs")
            assert wrong.process.wait(3) == 1
            wrong.close(); live.remove(wrong)

            tty.quiet(":memory add user verbosity Keep replies concise")
            with sqlite3.connect(root / ".state/lingcell/runtime.db") as db:
                memory_id = db.execute("SELECT id FROM memories WHERE topic='verbosity'").fetchone()[0]
            tty.command(":memory show " + memory_id, "Keep replies concise")
            tty.quiet(":memory update " + memory_id + " Keep replies very concise")
            tty.command(":memory show " + memory_id, "Keep replies very concise")
            tty.quiet(":memory forget " + memory_id)
            with sqlite3.connect(root / ".state/lingcell/runtime.db") as db:
                assert db.execute("SELECT count(*) FROM memory_fts WHERE id=?", (memory_id,)).fetchone()[0] == 0
            tty.command(":session new", "当前会话：")
            new_id = sessions(root)[-1][0]
            tty.command(":ai " + NEW, "RUNTIME_NEW_MEMORY_OK")
            tty.command(":ai " + HISTORY, "RUNTIME_HISTORY_OK")
            tty.command(":session resume " + first_id, "恢复后手动继续")
            tty.send(":session delete " + new_id + "\r"); tty.confirm_ready(); tty.send("n\r"); tty.prompt()
            assert (new_id,) in sessions(root)
            tty.send(":session delete " + new_id + "\r"); tty.confirm_ready(); tty.send("y\r"); tty.prompt()
            assert (new_id,) not in sessions(root)
            tty.send(":ai " + REFUSE + "\r"); tty.confirm_ready(); tty.send("n\r"); tty.finish("command declined")
            assert not (root / "MUST_NOT_EXIST").exists()
            before_extraction = RuntimeModel.extractions
            tty.send(":ai " + UNKNOWN + "\r"); tty.confirm_ready(); tty.send("y\r"); tty.until("UNKNOWN_COMMAND_STARTED\r\n")
            with sqlite3.connect(root / ".state/lingcell/runtime.db") as db:
                record = json.loads(db.execute("SELECT record FROM turns WHERE session=? ORDER BY number DESC LIMIT 1", (first_id,)).fetchone()[0])
                assert record["steps"][0]["status"] == "started"
            tty.process.kill(); tty.process.wait(3); tty.close(); live.remove(tty)
            time.sleep(.1)
            tty = Terminal(binary, root, config, flags=("--resume", first_id)); live.append(tty)
            tty.until("结果未知，绝不自动重试")
            tty.prompt()
            assert (root / "unknown-count").read_text() == "once\n"
            assert RuntimeModel.extractions == before_extraction, "failed/interrupted turn extracted"
            tty.command(":sh printf 'SHELL_AFTER_RECOVERY\\n'", "SHELL_AFTER_RECOVERY\r\n")
            stop(tty); live.remove(tty)
            print("PASS runtime: multi-turn, restore, Bash reset, locks/root checks, isolated memory retrieval, archive tool, refusal and unknown-result no replay")

            ephemeral = root / "ephemeral"; ephemeral.mkdir(); (ephemeral / ".git").mkdir()
            (ephemeral / ".bashrc").write_text("PROMPT_COMMAND=':'\n")
            before_extraction = RuntimeModel.extractions
            tty = start(binary, ephemeral, config, ("--ephemeral",)); live.append(tty)
            tty.command(":ai " + FIRST, "RUNTIME_FIRST_OK")
            tty.command(":ai " + FOLLOW, "RUNTIME_FOLLOW_OK")
            tty.command(":ai " + EPHEMERAL_MEMORY, "RUNTIME_EPHEMERAL_MEMORY_DISABLED")
            tty.command(":memory list", "memory disabled or unavailable")
            stop(tty); live.remove(tty)
            assert not (ephemeral / ".state").exists() and not (ephemeral / ".cache").exists()
            assert RuntimeModel.extractions == before_extraction
            print("PASS runtime ephemeral: continuous in-memory turns, no session/history files, memory reads or extraction")
            tty = start(binary, root, config, ("--ephemeral",)); live.append(tty)
            tty.command(":ai " + EPHEMERAL_MEMORY, "RUNTIME_EPHEMERAL_MEMORY_DISABLED")
            stop(tty); live.remove(tty)
            no_history = root / "no-history"; no_history.mkdir(); (no_history / ".git").mkdir()
            (no_history / ".bashrc").write_text("PROMPT_COMMAND=':'\n")
            tty = start(binary, no_history, config, ("--no-history",)); live.append(tty)
            tty.command(":sh printf 'NO_HISTORY_OK\\n'", "NO_HISTORY_OK\r\n")
            stop(tty); live.remove(tty)
            assert not (no_history / ".cache/lingcell/history").exists() and (no_history / ".state/lingcell/runtime.db").exists()
            print("PASS runtime flags: ephemeral ignores existing memory; no-history still persists sessions")

            broken = root / "broken"; broken.mkdir(); (broken / ".git").mkdir()
            (broken / ".bashrc").write_text("PROMPT_COMMAND=':'\n")
            state = broken / ".state/lingcell"; state.mkdir(parents=True)
            (state / "runtime.db").write_text("broken sqlite")
            tty = start(binary, broken, config); live.append(tty)
            before = RuntimeModel.decisions
            tty.command(":ai " + FIRST, "runtime unavailable")
            assert RuntimeModel.decisions == before
            tty.command(":sh printf 'SHELL_WITH_BROKEN_DB\\n'", "SHELL_WITH_BROKEN_DB\r\n")
            stop(tty); live.remove(tty)
            assert not RuntimeModel.errors, RuntimeModel.errors
            print("PASS runtime storage failure: Agent blocked, ordinary shell remains usable")
    finally:
        if RuntimeModel.errors:
            print("mock evidence failures:", RuntimeModel.errors)
        for tty in live:
            tty.close()
        server.shutdown(); server.server_close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True, type=pathlib.Path)
    args = parser.parse_args()
    verify(args.binary.resolve())
