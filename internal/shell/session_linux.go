// Package shell runs a real, persistent, interactive Bash behind a PTY.
package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// OutputLimit caps captured command output; live PTY output is not truncated.
const OutputLimit = 64 * 1024

// Result contains the command status and merged PTY output for a completed run.
type Result struct {
	Command   string `json:"command"`
	ExitCode  int    `json:"exit_code"`
	Cwd       string `json:"cwd"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

// Options configures a session. Nil Output discards live output; nil Input
// disables terminal input forwarding. RCFile overrides ~/.bashrc for tests.
type Options struct {
	Dir, RCFile string
	Output      io.Writer
	Input       *os.File
}
type event struct {
	code  int
	cwd   string
	names []string
	path  string
	err   error
}

// Session owns a Bash process and its terminal/control descriptors.
// Run calls are serialized. Cwd, Names and Knows can be queried concurrently.
// Close is idempotent and may interrupt an active Run.
type Session struct {
	cmd                         *exec.Cmd
	terminal, control, commands *os.File
	ttyFD, controlFD            int
	events                      chan event
	done                        chan struct{}
	stopped                     chan struct{}
	mu                          sync.Mutex
	runMu                       sync.Mutex
	cwd                         string
	names                       []string
	path                        string
	capture                     []byte
	truncated                   bool
	output                      io.Writer
	input                       *os.File
	closeOnce                   sync.Once
}

// Start launches an interactive Bash and waits for its startup control event.
// The caller owns the returned session and must close it.
func Start(ctx context.Context, opts Options) (*Session, error) {
	rc := opts.RCFile
	if rc == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		rc = home + "/.bashrc"
	}
	startup, err := os.CreateTemp("", "lingcell-bash-*.rc")
	if err != nil {
		return nil, err
	}
	defer os.Remove(startup.Name())
	script := "if [[ -f " + Quote(rc) + " ]]; then builtin source " + Quote(rc) + "; fi\n" + hook
	if _, err = startup.WriteString(script); err != nil {
		startup.Close()
		return nil, err
	}
	if err := startup.Close(); err != nil {
		return nil, fmt.Errorf("close Bash startup file: %w", err)
	}
	cr, cw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	qr, qw, err := os.Pipe()
	if err != nil {
		cr.Close()
		cw.Close()
		return nil, err
	}
	cmd := exec.Command("bash", "--noprofile", "--rcfile", startup.Name(), "-i")
	cmd.Dir = opts.Dir
	cmd.Env = append(os.Environ(), "HISTFILE=/dev/null")
	cmd.ExtraFiles = []*os.File{cw, qr}
	tty, err := pty.Start(cmd)
	cw.Close()
	qr.Close()
	if err != nil {
		cr.Close()
		qw.Close()
		return nil, err
	}
	if opts.Output == nil {
		opts.Output = io.Discard
	}
	s := &Session{cmd: cmd, terminal: tty, control: cr, commands: qw, events: make(chan event, 8), done: make(chan struct{}), stopped: make(chan struct{}), output: opts.Output, input: opts.Input}
	s.ttyFD, s.controlFD = int(tty.Fd()), int(cr.Fd())
	// Configure nonblocking reads before starting the polling goroutine. A failed
	// setup must not leave a reader blocked indefinitely while Close waits for it.
	for _, fd := range []int{s.ttyFD, s.controlFD} {
		if err := unix.SetNonblock(fd, true); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			_ = tty.Close()
			_ = cr.Close()
			_ = qw.Close()
			return nil, fmt.Errorf("configure shell descriptor: %w", err)
		}
	}
	go func() { _ = cmd.Wait(); close(s.done) }()
	go s.pump()
	select {
	case ev := <-s.events:
		if ev.err != nil {
			s.Close()
			return nil, ev.err
		}
		s.update(ev)
	case <-s.done:
		s.Close()
		return nil, errors.New("bash exited during startup")
	case <-ctx.Done():
		s.Close()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		s.Close()
		return nil, errors.New("bash startup hook did not respond")
	}
	return s, nil
}

const hook = `
# Install after the user's rc file, preserving both scalar and array hooks.
__lc_old_prompt=("${PROMPT_COMMAND[@]}")
__lc_save() { __lc_status=$1; return "$1"; }
__lc_report() {
    PS1= PS2= PS0=
    stty -echo
    builtin printf '%s\0' ready "$__lc_status" "$(builtin pwd -P)" "$(builtin compgen -abk -A function)" "$PATH" >&3
}
PROMPT_COMMAND=('__lc_save "$?"' "${__lc_old_prompt[@]}" '__lc_report')
PS1= PS2= PS0=
HISTFILE=/dev/null
set +o history
set +o emacs +o vi
stty -echo
`

// Quote returns a single literal Bash word, escaping embedded single quotes.
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func (s *Session) update(ev event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cwd = ev.cwd
	s.names = ev.names
	s.path = ev.path
}

// Cwd returns the physical working directory reported by Bash.
func (s *Session) Cwd() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}

// Names returns a snapshot of Bash's builtin, keyword, alias and function names.
func (s *Session) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.names...)
}

// Knows resolves Bash names and executable files in the current shell PATH.
func (s *Session) Knows(name string) bool {
	for _, n := range s.Names() {
		if n == name {
			return true
		}
	}
	s.mu.Lock()
	path, cwd := s.path, s.cwd
	s.mu.Unlock()
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return true
		}
	}
	return false
}

// Done closes when the Bash process exits.
func (s *Session) Done() <-chan struct{} { return s.done }

// pump reads both descriptors in one goroutine. Before publishing a control event
// it drains the PTY, so output preceding the event is included in the result.
func (s *Session) pump() {
	defer close(s.stopped)
	tfd, cfd := s.ttyFD, s.controlFD
	var pending []byte
	buf := make([]byte, 8192)
	drain := func() {
		for {
			n, err := unix.Read(tfd, buf)
			if n > 0 {
				_, _ = s.output.Write(buf[:n])
				s.mu.Lock()
				left := OutputLimit - len(s.capture)
				if n > left {
					s.truncated = true
				}
				s.capture = append(s.capture, buf[:min(n, left)]...)
				s.mu.Unlock()
			}
			if err != nil || n == 0 {
				return
			}
		}
	}
	for {
		fds := []unix.PollFd{{Fd: int32(tfd), Events: unix.POLLIN}, {Fd: int32(cfd), Events: unix.POLLIN}}
		_, err := unix.Poll(fds, 100)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			s.publish(event{err: errors.New("bash control polling failed")})
			return
		}
		drain()
		if fds[1].Revents&unix.POLLIN != 0 {
			for {
				n, err := unix.Read(cfd, buf)
				if n > 0 {
					pending = append(pending, buf[:n]...)
				}
				if err != nil || n == 0 {
					break
				}
			}
			if len(pending) > 4*1024*1024 {
				s.publish(event{err: errors.New("bash control event exceeded limit")})
				return
			}
			for bytes.Count(pending, []byte{0}) >= 5 {
				fields := make([]string, 5)
				for i := range fields {
					end := bytes.IndexByte(pending, 0)
					fields[i] = string(pending[:end])
					pending = pending[end+1:]
				}
				drain()
				ev := event{cwd: fields[2], names: strings.Split(fields[3], "\n"), path: fields[4]}
				ev.code, err = strconv.Atoi(fields[1])
				if fields[0] != "ready" || err != nil {
					ev.err = errors.New("bash session hook was changed or disabled; restart LingCell (commands are never replayed)")
				}
				s.publish(ev)
			}
		}
		if fds[1].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			s.publish(event{err: errors.New("bash control channel closed; command was not replayed")})
			return
		}
		select {
		case <-s.done:
			drain()
			return
		default:
		}
	}
}
func (s *Session) publish(ev event) {
	select {
	case s.events <- ev:
	case <-s.done:
	}
}

// Run executes one command at Bash's top level without replaying on failure.
// Cancellation interrupts the foreground process group; an acknowledged
// interrupt returns its exit code, while an unresponsive session is closed.
func (s *Session) Run(ctx context.Context, command string) (Result, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	result := Result{Command: command, Cwd: s.Cwd()}
	if strings.ContainsRune(command, 0) {
		return result, errors.New("commands cannot contain NUL")
	}
	if len(command) > 1024*1024 {
		return result, errors.New("command exceeds 1 MiB")
	}
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-s.done:
		return result, errors.New("bash session has exited")
	default:
	}
	s.mu.Lock()
	s.capture = nil
	s.truncated = false
	s.mu.Unlock()
	restore, err := s.forwardInput()
	if err != nil {
		return result, err
	}
	defer restore()
	// The short PTY line reads one NUL-framed command from fd 4. eval executes
	// it at Bash's top level, preserving shell state and supporting long input.
	dispatch := "IFS= builtin read -r -d '' __lc_command <&4; stty echo; builtin eval -- \"$__lc_command\"; __lc_status=$?; if [[ ${PROMPT_COMMAND[*]} != *'__lc_report'* ]] || ! builtin declare -F __lc_report >/dev/null; then builtin printf '%s\\0' broken 1 \"$PWD\" '' '' >&3; fi; __lc_save \"$__lc_status\"\n"
	if _, err := s.terminal.WriteString(dispatch); err != nil {
		return result, err
	}
	writeDone := make(chan error, 1)
	go func() { _, err := s.commands.WriteString(command + "\x00"); writeDone <- err }()
	select {
	case err := <-writeDone:
		if err != nil {
			return result, err
		}
	case <-s.done:
		return result, errors.New("bash exited while accepting command")
	case <-ctx.Done():
		s.Close()
		return result, ctx.Err()
	}
	var ev event
	select {
	case ev = <-s.events:
	case <-s.done:
		// pump may have received the final event immediately before process exit.
		<-s.stopped
		select {
		case ev = <-s.events:
		default:
			return result, errors.New("bash exited; command was not replayed")
		}
	case <-ctx.Done():
		s.CancelForeground()
		select {
		case ev = <-s.events:
		case <-s.done:
			return result, ctx.Err()
		case <-time.After(2 * time.Second):
			s.Close()
			return result, ctx.Err()
		}
	}
	if ev.err != nil {
		s.Close()
		return result, ev.err
	}
	s.update(ev)
	result.ExitCode, result.Cwd = ev.code, ev.cwd
	s.mu.Lock()
	result.Output, result.Truncated = string(s.capture), s.truncated
	s.mu.Unlock()
	return result, nil
}

// CancelForeground sends SIGINT to the PTY's current foreground process group.
func (s *Session) CancelForeground() {
	pgid, err := unix.IoctlGetInt(s.ttyFD, unix.TIOCGPGRP)
	if err == nil && pgid > 0 {
		_ = unix.Kill(-pgid, unix.SIGINT)
	}
}

// Resize copies the input terminal's dimensions to the PTY when available.
func (s *Session) Resize() {
	if s.input != nil {
		if size, err := unix.IoctlGetWinsize(int(s.input.Fd()), unix.TIOCGWINSZ); err == nil {
			_ = unix.IoctlSetWinsize(s.ttyFD, unix.TIOCSWINSZ, size)
		}
	}
}
func (s *Session) forwardInput() (func(), error) {
	if s.input == nil || !term.IsTerminal(int(s.input.Fd())) {
		return func() {}, nil
	}
	fd := int(s.input.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	s.Resize()
	stop, ended := make(chan struct{}), make(chan struct{})
	resize := make(chan os.Signal, 1)
	signal.Notify(resize, syscall.SIGWINCH)
	go func() {
		defer close(ended)
		buf := make([]byte, 4096)
		for {
			select {
			case <-stop:
				return
			case <-resize:
				s.Resize()
			default:
			}
			fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
			n, err := unix.Poll(fds, 30)
			if err == unix.EINTR {
				continue
			}
			if err != nil {
				return
			}
			if n > 0 && fds[0].Revents&unix.POLLIN != 0 {
				n, err := unix.Read(fd, buf)
				if err != nil || n == 0 {
					return
				}
				if _, err := s.terminal.Write(buf[:n]); err != nil {
					return
				}
			}
		}
	}()
	return func() { close(stop); <-ended; signal.Stop(resize); _ = term.Restore(fd, state) }, nil
}

// Close stops Bash, waits for its reader goroutine and releases all descriptors.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		if pgid, err := unix.IoctlGetInt(s.ttyFD, unix.TIOCGPGRP); err == nil && pgid > 0 && pgid != s.cmd.Process.Pid {
			_ = unix.Kill(-pgid, unix.SIGHUP)
		}
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGHUP)
		_ = s.commands.Close()
		select {
		case <-s.done:
		case <-time.After(250 * time.Millisecond):
			_ = s.cmd.Process.Kill()
		}
		<-s.done
		<-s.stopped
		_ = s.terminal.Close()
		_ = s.control.Close()
	})
}
