package runtime

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"lingcell/internal/model"
)

// Runtime owns one active conversation. A nil Store makes it ephemeral and
// disables memory; history still connects requests within the running process.
type Runtime struct {
	Store     *Store
	Session   Session
	Turns     []Turn
	Summary   string
	Through   int
	MaxBytes  int
	ModelName string
	Timeout   time.Duration
	LastBytes int
	lock      *os.File
	active    *Turn
	fault     error
}

// New starts a new session, or explicitly restores id under a process lock.
// Restore never executes tools. The caller must Close before closing Store.
func New(s *Store, root, id string, maxBytes int, modelName string, timeout time.Duration) (*Runtime, error) {
	if maxBytes == 0 {
		maxBytes = 128 * 1024
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	r := &Runtime{Store: s, MaxBytes: maxBytes, ModelName: modelName, Timeout: timeout}
	if id == "" {
		var err error
		id, err = newID()
		if err != nil {
			return nil, err
		}
		r.Session = Session{ID: id, Root: root, Created: timestamp()}
		if s != nil {
			if _, err = s.db.Exec("INSERT INTO sessions VALUES(?,?,?)", id, root, r.Session.Created); err != nil {
				return nil, err
			}
		}
	} else {
		if s == nil {
			return nil, errors.New("cannot resume in ephemeral mode")
		}
		if err := s.db.QueryRow("SELECT id,root,created FROM sessions WHERE id=?", id).Scan(&r.Session.ID, &r.Session.Root, &r.Session.Created); err != nil {
			return nil, fmt.Errorf("find session: %w", err)
		}
		if r.Session.Root != root {
			return nil, errors.New("session project root differs; start from its project directory")
		}
	}
	if s != nil {
		f, err := s.lock(id)
		if err != nil {
			return nil, err
		}
		r.lock = f
		fail := func(err error) (*Runtime, error) { r.Close(); return nil, err }
		r.Turns, err = s.turns(id)
		if err != nil {
			return fail(err)
		}
		for i := range r.Turns {
			t := &r.Turns[i]
			if t.Status != "running" {
				continue
			}
			t.Status = "interrupted"
			for j := range t.Steps {
				if t.Steps[j].Status == "started" {
					t.Steps[j].Status = "unknown"
				} else if t.Steps[j].Status == "proposed" {
					t.Steps[j].Status = "not_executed"
				}
			}
			if err = s.saveTurn(id, *t); err != nil {
				return fail(err)
			}
		}
		err = s.db.QueryRow("SELECT through,content FROM summaries WHERE session=?", id).Scan(&r.Through, &r.Summary)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fail(err)
		}
	}
	return r, nil
}

// Close releases the active session lock; it is safe to call repeatedly.
func (r *Runtime) Close() error {
	if r.lock != nil {
		err := r.lock.Close()
		r.lock = nil
		return err
	}
	return nil
}
func (r *Runtime) persist() error {
	if r.fault != nil {
		return r.fault
	}
	if r.active == nil {
		return errors.New("no active turn")
	}
	if r.Store != nil {
		if err := r.Store.saveTurn(r.Session.ID, *r.active); err != nil {
			r.fault = fmt.Errorf("runtime write failed; Agent stopped: %w", err)
			return r.fault
		}
	} else {
		b, _ := json.Marshal(r.active)
		if len(b) > maxTurnBytes {
			r.fault = errors.New("turn archive exceeds 2 MiB")
			return r.fault
		}
		size := len(b)
		for _, turn := range r.Turns {
			raw, _ := json.Marshal(turn)
			size += len(raw)
		}
		if size > maxSessionBytes {
			r.fault = errors.New("session archive exceeds 64 MiB; start a new session")
			return r.fault
		}
	}
	return nil
}

// Begin records a request before asking a model or executing a command.
func (r *Runtime) Begin(messages []model.Message) error {
	if r.fault != nil {
		return r.fault
	}
	if r.active != nil {
		return errors.New("turn already active")
	}
	r.active = &Turn{Number: len(r.Turns) + 1, Status: "running", Messages: messages, Created: timestamp()}
	return r.persist()
}

// Append persists a bounded message before the loop proceeds.
func (r *Runtime) Append(m model.Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(b) > 256*1024 {
		return errors.New("message exceeds 256 KiB")
	}
	r.active.Messages = append(r.active.Messages, m)
	if m.Role == "assistant" {
		for _, c := range m.ToolCalls {
			r.active.Steps = append(r.active.Steps, Step{ID: c.ID, Name: c.Function.Name, Arguments: c.Function.Arguments, Status: "proposed"})
		}
	}
	if err := r.persist(); err != nil {
		return err
	}
	return nil
}

// ToolState durably records approval, execution start, or result. A failed
// write must stop execution, even when it follows a successful confirmation.
func (r *Runtime) ToolState(id, status string, approved bool, result string) error {
	for i := range r.active.Steps {
		s := &r.active.Steps[i]
		if s.ID == id {
			s.Status = status
			s.Approved = approved
			s.Result = result
			return r.persist()
		}
	}
	return errors.New("tool step not found")
}

// Finish closes a turn without retrying uncertain execution. It records a
// transport failure as unknown, and other unfinished proposals as not executed.
func (r *Runtime) Finish(success bool) error {
	if r.active == nil {
		return nil
	}
	t := r.active
	t.Status = "failed"
	if success {
		t.Status = "completed"
	}
	for i := range t.Steps {
		if t.Steps[i].Status == "started" {
			t.Steps[i].Status = "unknown"
		} else if t.Steps[i].Status == "proposed" {
			t.Steps[i].Status = "not_executed"
		}
	}
	err := r.persist()
	if err == nil && r.Store != nil {
		var raw string
		loadErr := r.Store.db.QueryRow("SELECT record FROM turns WHERE session=? AND number=?", r.Session.ID, t.Number).Scan(&raw)
		var saved Turn
		if loadErr == nil {
			loadErr = json.Unmarshal([]byte(raw), &saved)
		}
		if loadErr != nil {
			r.fault = loadErr
			err = loadErr
		} else {
			t = &saved
		}
	}
	r.Turns = append(r.Turns, *t)
	r.active = nil
	return err
}

// Interrupted describes unfinished execution explicitly for the restore UI.
func (r *Runtime) Interrupted() string {
	text := ""
	for _, t := range r.Turns {
		if t.Status == "interrupted" {
			text += fmt.Sprintf("轮次 %d 已中断。\n", t.Number)
		}
		for _, s := range t.Steps {
			if s.Status == "unknown" {
				text += fmt.Sprintf("轮次 %d 工具 %s：结果未知，绝不自动重试。\n", t.Number, s.ID)
			}
		}
	}
	return text
}

// History returns a bounded page including original message/step archives.
func (r *Runtime) History(start, count int, offsets ...int) (any, error) {
	offset := 0
	if len(offsets) > 0 {
		offset = offsets[0]
	}
	if offset < 0 || offset > maxTurnBytes || (offset > 0 && count != 1) {
		return nil, errors.New("invalid history offset; use count=1 for fragments")
	}
	if start < 1 || count < 1 || count > 20 {
		return nil, errors.New("history requires start_turn >= 1 and count between 1 and 20")
	}
	type page struct {
		Turns     []Turn `json:"turns"`
		Truncated bool   `json:"truncated"`
		Next      int    `json:"next_turn"`
		Reference string `json:"reference"`
	}
	p := page{Reference: r.Session.ID}
	archive := r.Turns
	if r.active != nil {
		var current Turn
		if r.Store != nil {
			var raw string
			if err := r.Store.db.QueryRow("SELECT record FROM turns WHERE session=? AND number=?", r.Session.ID, r.active.Number).Scan(&raw); err != nil {
				r.fault = err
				return nil, err
			}
			if err := json.Unmarshal([]byte(raw), &current); err != nil {
				r.fault = err
				return nil, err
			}
		} else {
			raw, _ := json.Marshal(r.active)
			if err := json.Unmarshal(raw, &current); err != nil {
				return nil, err
			}
		}
		// A snapshot goes inside tool data, never into executable protocol calls.
		archive = append(append([]Turn(nil), archive...), current)
	}
	for _, t := range archive {
		if t.Number < start {
			continue
		}
		if len(p.Turns) >= count {
			p.Truncated = true
			p.Next = t.Number
			break
		}
		p.Turns = append(p.Turns, t)
		b, _ := json.Marshal(p)
		if len(b) > 32*1024 || offset > 0 {
			p.Turns = p.Turns[:len(p.Turns)-1]
			p.Truncated = true
			p.Next = t.Number
			if len(p.Turns) == 0 {
				raw, _ := json.Marshal(t)
				if offset >= len(raw) {
					return nil, errors.New("history offset exceeds turn archive")
				}
				fragment := clip(string(raw[offset:]), 16*1024)
				page := map[string]any{"turn": t.Number, "reference": r.Session.ID, "record_fragment": fragment, "offset": offset, "next_offset": offset + len(fragment), "more": offset+len(fragment) < len(raw), "truncated": true}
				for {
					encoded, _ := json.Marshal(page)
					if len(encoded) <= 32*1024 {
						break
					}
					fragment = clip(fragment, len(fragment)/2)
					page["record_fragment"] = fragment
					page["next_offset"] = offset + len(fragment)
					page["more"] = true
				}
				return page, nil
			}
			break
		}
	}
	return p, nil
}
