package debug

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
)

var debugSb strings.Builder
var logMutex sync.Mutex

func GetStackInfo(lastStackIdx int) (string, string) {
	var stack, stackSimple = "", ""
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack = fmt.Sprintf("%s\n", buf[:n])
	stackIdx := lastStackIdx // print the last stack
	lines := strings.Split(stack, "\n")
	if stackIdx*2 <= len(lines) {
		stackSimple = lines[stackIdx*2-1] + " " + lines[stackIdx*2]
	}
	return stack, stackSimple
}

func Log(args ...any) {
	logMutex.Lock()
	defer logMutex.Unlock()
	debugSb.WriteString(fmt.Sprint(args...))
	debugSb.WriteString("\n")
}

func LogWithStack(args ...any) {
	Log(args...)
	logStackTrace()
}

func logStackTrace() {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	debugSb.WriteString(fmt.Sprintf("\n%s\n", buf[:n]))
}

func PrintStackTrace() {
	buf := make([]byte, 4096)
	stackSize := runtime.Stack(buf, false)
	fmt.Printf("%s\n", buf[:stackSize])
}
func PrintAllStackTrace() {
	buf := make([]byte, 1<<20)
	stackSize := runtime.Stack(buf, true)
	fmt.Printf("%s\n", buf[:stackSize])
}

func FlushLog() {
	logs := debugSb.String()
	if logs != "" {
		println(logs)
		debugSb.Reset()
	}
}
