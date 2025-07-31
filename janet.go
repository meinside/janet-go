// janet.go

// Package janet provides an interface to the Janet virtual machine.
package janet

/*
#cgo CFLAGS: -I./amalgamated
#cgo LDFLAGS: -lm -lpthread -ldl
#include "amalgamated/janet.c"
#include <stdio.h>
#include <unistd.h>
#include <fcntl.h>

static int redirectStdout(int pipe_fd) {
	fflush(stdout);
	int original_fd = dup(STDOUT_FILENO);
	dup2(pipe_fd, STDOUT_FILENO);
	close(pipe_fd);
	return original_fd;
}

static int redirectStderr(int pipe_fd) {
	fflush(stderr);
	int original_fd = dup(STDERR_FILENO);
	dup2(pipe_fd, STDERR_FILENO);
	close(pipe_fd);
	return original_fd;
}

static void restoreStdout(int original_fd) {
	fflush(stdout);
	if (original_fd != -1) {
		dup2(original_fd, STDOUT_FILENO);
		close(original_fd);
	}
}

static void restoreStderr(int original_fd) {
    fflush(stderr);
    if (original_fd != -1) {
        dup2(original_fd, STDERR_FILENO);
        close(original_fd);
    }
}
*/
import "C"

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"unsafe"
)

// shared VM
var _sharedVM *VM

// VM represents a Janet virtual machine instance.
type VM struct {
	mu sync.Mutex

	env *C.JanetTable
}

// SharedVM initializes and returns a new shared Janet VM.
func SharedVM() (vm *VM, err error) {
	if _sharedVM == nil {
		// initialize,
		C.janet_init()

		// and create a new VM
		if env := C.janet_core_env(nil); env == nil {
			err = errors.New("failed to create janet environment")
		} else {
			_sharedVM = &VM{env: env}
		}
	}
	return _sharedVM, err
}

// Close deinitializes the Janet VM.
func (vm *VM) Close() {
	if _sharedVM != nil {
		C.janet_deinit()
		_sharedVM = nil
	}
}

// janetValueToString converts a Janet value to its string representation.
func janetValueToString(value C.Janet) string {
	switch C.janet_type(value) {
	case C.JANET_NIL:
		return "nil"
	case C.JANET_BOOLEAN:
		if C.janet_unwrap_boolean(value) != 0 {
			return "true"
		}
		return "false"
	case C.JANET_NUMBER:
		var buffer C.JanetBuffer
		C.janet_buffer_init(&buffer, 0)
		C.janet_to_string_b(&buffer, value)
		output := C.GoStringN((*C.char)(unsafe.Pointer(buffer.data)), C.int(buffer.count))
		C.janet_buffer_deinit(&buffer)
		return output
	case C.JANET_STRING:
		return C.GoString((*C.char)(unsafe.Pointer(C.janet_unwrap_string(value))))
	case C.JANET_SYMBOL:
		return C.GoString((*C.char)(unsafe.Pointer(C.janet_unwrap_symbol(value))))
	case C.JANET_TUPLE:
		var data *C.Janet
		var length C.int32_t
		C.janet_indexed_view(value, &data, &length)

		var result string
		for i := C.int32_t(0); i < length; i++ {
			elem := *(*C.Janet)(unsafe.Pointer(uintptr(unsafe.Pointer(data)) + uintptr(i)*unsafe.Sizeof(*data)))
			result += janetValueToString(elem)
			if i < length-1 {
				result += " "
			}
		}
		return "(" + result + ")"
	default:
		var buffer C.JanetBuffer
		C.janet_buffer_init(&buffer, 0)
		C.janet_to_string_b(&buffer, value)
		output := C.GoStringN((*C.char)(unsafe.Pointer(buffer.data)), C.int(buffer.count))
		C.janet_buffer_deinit(&buffer)
		return output
	}
}

// ExecuteString executes a Janet string and returns the result.
func (vm *VM) ExecuteString(
	ctx context.Context,
	code string,
) (string, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// suppress error output (redirect to /dev/null)
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0755)
	defer func() { _ = devNull.Close() }()
	originalStderrFd := C.redirectStderr(C.int(devNull.Fd()))

	// restore stderr
	defer C.restoreStderr(originalStderrFd)

	type result struct {
		value string
		err   error
	}

	// result channel
	resultCh := make(chan result, 1)

	// FIXME: need a way of stopping/interrupting `C.janet_dostring()`
	go func() {
		// for preventing `exit status 2`
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// run janet code,
		cCode := C.CString(code)
		defer C.free(unsafe.Pointer(cCode))
		var janetResult C.Janet
		ret := C.janet_dostring(
			vm.env,
			cCode,
			nil,
			&janetResult,
		)

		// and return the result
		if ret != C.JANET_SIGNAL_OK {
			var buffer C.JanetBuffer
			C.janet_buffer_init(&buffer, 0)
			C.janet_to_string_b(&buffer, janetResult)
			errOutput := C.GoStringN((*C.char)(unsafe.Pointer(buffer.data)), C.int(buffer.count))
			C.janet_buffer_deinit(&buffer)
			resultCh <- result{value: "", err: errors.New(errOutput)}
			return
		}
		resultCh <- result{value: janetValueToString(janetResult), err: nil}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resultCh:
		return res.value, res.err
	}
}

// ExecuteStringWithOutput executes a Janet string and returns the result, along with any output to stdout and stderr.
func (vm *VM) ExecuteStringWithOutput(
	ctx context.Context,
	code string,
) (
	evaluated string,
	stdout string,
	stderr string,
	err error,
) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	type result struct {
		evaluated string
		stdout    string
		stderr    string
		err       error
	}

	resultCh := make(chan result, 1)

	// FIXME: need a way of stopping/interrupting `C.janet_dostring()`
	go func() {
		// for preventing `exit status 2`
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// create pipes for stdout and stderr
		var stdoutPipe [2]C.int
		var stderrPipe [2]C.int
		if C.pipe(&stdoutPipe[0]) != 0 {
			resultCh <- result{err: errors.New("failed to create stdout pipe")}
			return
		}
		if C.pipe(&stderrPipe[0]) != 0 {
			resultCh <- result{err: errors.New("failed to create stderr pipe")}
			return
		}

		// read from stdout/stderr pipes
		var outBuf, errBuf bytes.Buffer
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			// for preventing `exit status 2`
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()

			defer wg.Done()
			buf := make([]byte, 1024)
			for {
				n, _ := C.read(stdoutPipe[0], unsafe.Pointer(&buf[0]), 1024)
				if n <= 0 {
					break
				}
				outBuf.Write(buf[:n])
			}
			C.close(stdoutPipe[0])
		}()
		go func() {
			// for preventing `exit status 2`
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()

			defer wg.Done()
			buf := make([]byte, 1024)
			for {
				n, _ := C.read(stderrPipe[0], unsafe.Pointer(&buf[0]), 1024)
				if n <= 0 {
					break
				}
				errBuf.Write(buf[:n])
			}
			C.close(stderrPipe[0])
		}()

		// redirect stdout and stderr
		originalStdoutFd := C.redirectStdout(stdoutPipe[1])
		originalStderrFd := C.redirectStderr(stderrPipe[1])

		// run janet code,
		cCode := C.CString(code)
		defer C.free(unsafe.Pointer(cCode))
		var janetResult C.Janet
		ret := C.janet_dostring(
			vm.env,
			cCode,
			nil,
			&janetResult,
		)

		// restore stdout and stderr
		C.restoreStdout(originalStdoutFd)
		C.restoreStderr(originalStderrFd)

		// close write ends of pipes
		C.close(stdoutPipe[1])
		C.close(stderrPipe[1])

		wg.Wait()

		// and return the result
		if ret != C.JANET_SIGNAL_OK {
			var buffer C.JanetBuffer
			C.janet_buffer_init(&buffer, 0)
			C.janet_to_string_b(&buffer, janetResult)
			errOutput := C.GoStringN((*C.char)(unsafe.Pointer(buffer.data)), C.int(buffer.count))
			C.janet_buffer_deinit(&buffer)
			resultCh <- result{
				stdout: outBuf.String(),
				stderr: errBuf.String(),
				err:    errors.New(errOutput),
			}
			return
		}

		resultCh <- result{
			evaluated: janetValueToString(janetResult),
			stdout:    outBuf.String(),
			stderr:    errBuf.String(),
			err:       nil,
		}
	}()

	select {
	case <-ctx.Done():
		return "", "", "", ctx.Err()
	case res := <-resultCh:
		return res.evaluated, res.stdout, res.stderr, res.err
	}
}
