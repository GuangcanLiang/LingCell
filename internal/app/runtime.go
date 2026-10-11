package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"lingcell/internal/runtime"
)

func (a *App) showContext() error {
	if a.Runtime == nil {
		return errors.New("runtime unavailable")
	}
	r := a.Runtime
	return a.status(fmt.Sprintf("会话 %s；已完成/关闭轮次 %d；最近请求 %d/%d bytes；摘要覆盖至轮次 %d。", r.Session.ID, len(r.Turns), r.LastBytes, r.MaxBytes, r.Through))
}
func (a *App) sessionCommand(input string) error {
	if a.Runtime == nil {
		return errors.New("runtime unavailable")
	}
	fields := strings.Fields(input)
	if len(fields) < 2 {
		return errors.New("usage: :session list/new/resume <id>/show [id]/delete <id>")
	}
	r := a.Runtime
	switch fields[1] {
	case "list":
		if len(fields) != 2 {
			return errors.New("usage: :session list")
		}
		if r.Store == nil {
			return a.status("临时会话：" + r.Session.ID)
		}
		sessions, err := r.Store.Sessions(r.Session.Root)
		if err != nil {
			return err
		}
		text, err := runtime.MemoryJSON(sessions)
		if err != nil {
			return err
		}
		return a.status(text)
	case "new", "resume":
		id := ""
		if fields[1] == "resume" {
			if len(fields) != 3 {
				return errors.New("usage: :session resume <id>")
			}
			id = fields[2]
		} else if len(fields) != 2 {
			return errors.New("usage: :session new")
		}
		if id == r.Session.ID {
			return a.showContext()
		}
		next, err := runtime.New(r.Store, r.Session.Root, id, r.MaxBytes, r.ModelName, r.Timeout)
		if err != nil {
			return err
		}
		if err = r.Close(); err != nil {
			next.Close()
			return err
		}
		a.Runtime = next
		a.last = nil
		if err = a.status("当前会话：" + next.Session.ID + "；Bash 状态未切换。恢复后手动继续，历史确认不授予新权限。"); err != nil {
			return err
		}
		if text := next.Interrupted(); text != "" {
			return a.status(text)
		}
		return nil
	case "show":
		if len(fields) > 3 {
			return errors.New("usage: :session show [id]")
		}
		target := r
		if len(fields) == 3 && fields[2] != r.Session.ID {
			var err error
			target, err = runtime.New(r.Store, r.Session.Root, fields[2], r.MaxBytes, r.ModelName, r.Timeout)
			if err != nil {
				return err
			}
			defer target.Close()
		}
		// Terminal view is the full bounded archive; the model tool is separately paged.
		raw, err := json.MarshalIndent(target.Turns, "", "  ")
		if err != nil {
			return err
		}
		_, err = (modelOutput{a.Output}).Write(append(raw, '\n'))
		return err
	case "delete":
		if len(fields) != 3 {
			return errors.New("usage: :session delete <id>")
		}
		if r.Store == nil {
			return errors.New("no persisted session in ephemeral mode")
		}
		if fields[2] == r.Session.ID {
			return errors.New("switch to another session before deleting the active session")
		}
		yes, err := a.confirm("删除会话 "+fields[2], r.Session.Root, "永久删除会话记录和摘要；独立长期记忆保留其来源引用")
		if err != nil || !yes {
			return err
		}
		return r.Store.DeleteSession(fields[2], r.Session.Root)
	}
	return errors.New("unknown session operation")
}
func (a *App) memoryCommand(input string) error {
	if a.Runtime == nil || a.Runtime.Store == nil {
		return errors.New("memory disabled or unavailable")
	}
	fields := strings.Fields(input)
	if len(fields) < 2 {
		return errors.New("usage: :memory list [scope]/show <id>/add <scope> <topic> <content>/update <id> <content>/forget <id>")
	}
	r := a.Runtime
	switch fields[1] {
	case "list":
		if len(fields) > 3 {
			return errors.New("usage: :memory list [scope]")
		}
		scope := "all"
		if len(fields) == 3 {
			scope = fields[2]
		}
		items, err := r.ListMemory(scope)
		if err != nil {
			return err
		}
		text, err := runtime.MemoryJSON(items)
		if err != nil {
			return err
		}
		return a.status(text)
	case "show":
		if len(fields) != 3 {
			return errors.New("usage: :memory show <id>")
		}
		m, err := r.Memory(fields[2])
		if err != nil {
			return err
		}
		text, err := runtime.MemoryJSON(m)
		if err != nil {
			return err
		}
		return a.status(text)
	case "add":
		if len(fields) < 5 {
			return errors.New("usage: :memory add <user|project|machine> <topic> <content>")
		}
		kind := "fact"
		if fields[2] == "user" {
			kind = "preference"
		}
		_, err := r.SaveMemory(runtime.Memory{Scope: fields[2], Kind: kind, Topic: fields[3], Content: strings.Join(fields[4:], " ")}, true)
		return err
	case "update":
		if len(fields) < 4 {
			return errors.New("usage: :memory update <id> <content>")
		}
		m, err := r.Memory(fields[2])
		if err != nil {
			return err
		}
		m.Content = strings.Join(fields[3:], " ")
		_, err = r.SaveMemory(m, true)
		return err
	case "forget":
		if len(fields) != 3 {
			return errors.New("usage: :memory forget <id>")
		}
		return r.ForgetMemory(fields[2])
	}
	return errors.New("unknown memory operation")
}
