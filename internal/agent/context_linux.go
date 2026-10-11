package agent

import (
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// runtimeContext contains program-generated facts, never environment values,
// account names, process lists or configuration contents. It is refreshed for
// each task; live system state still requires a confirmed command.
type runtimeContext struct {
	OS           string   `json:"os"`
	Kernel       string   `json:"kernel"`
	Architecture string   `json:"architecture"`
	Privilege    string   `json:"privilege"`
	Cwd          string   `json:"cwd"`
	ProjectRoot  string   `json:"project_root"`
	Tools        []string `json:"tools"`
}

func newRuntimeContext(cwd, root string) runtimeContext {
	kernel := "unknown"
	var uts unix.Utsname
	if err := unix.Uname(&uts); err == nil {
		kernel = unix.ByteSliceToString(uts.Release[:])
	}
	privilege := "non_root"
	if os.Geteuid() == 0 {
		privilege = "root"
	}
	var names []string
	for _, tool := range Tools() {
		names = append(names, tool.Function.Name)
	}
	return runtimeContext{OS: runtime.GOOS, Kernel: kernel, Architecture: runtime.GOARCH, Privilege: privilege, Cwd: cwd, ProjectRoot: root, Tools: names}
}
