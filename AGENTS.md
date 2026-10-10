# Repository instructions

Read [AGENT.md](AGENT.md) before changing this repository. It is the single
maintenance guide for architecture, Go conventions, validation and Git workflow.
Keep instructions in that file; this standard entrypoint exists for agents that
automatically discover `AGENTS.md`.

Development and acceptance testing must use separate agents and worktrees.
Spawn the test subagent with a fresh context and a minimal handoff. All changes
must be committed on a task branch; follow the gates in AGENT.md before merging.
