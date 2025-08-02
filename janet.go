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
	"runtime"
	"sync"
	"unsafe"
)

// shared VM
var _sharedVM *VM

// vmRequest is used to send a job to the VM handler goroutine.
type vmRequest struct {
	code         string
	responseChan chan vmResponse
}

// vmResponse is used to receive the result from the VM handler.
type vmResponse struct {
	evaluated string
	stdout    string
	stderr    string
	err       error
}

// VM represents a Janet virtual machine instance.
type VM struct {
	requestChan  chan vmRequest
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

// SharedVM initializes and returns a new shared Janet VM.
// It starts a dedicated OS-thread-locked goroutine to handle all CGo calls
// sequentially, ensuring thread safety.
func SharedVM() (vm *VM, err error) {
	if _sharedVM != nil {
		return _sharedVM, nil
	}

	initDone := make(chan error, 1)

	requestChan := make(chan vmRequest)
	shutdownChan := make(chan struct{})

	vm = &VM{
		requestChan:  requestChan,
		shutdownChan: shutdownChan,
	}
	vm.wg.Add(1)

	// The dedicated VM handler goroutine
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer vm.wg.Done()

		C.janet_init()
		defer C.janet_deinit()

		env := C.janet_core_env(nil)
		if env == nil {
			initDone <- errors.New("failed to create janet environment")
			return
		}
		close(initDone) // Signal successful initialization

		// Main loop to process requests
		for {
			select {
			case req := <-requestChan:
				handleVMRequest(env, req)
			case <-shutdownChan:
				return
			}
		}
	}()

	// Wait for initialization to complete
	err = <-initDone
	if err != nil {
		vm.wg.Wait() // Ensure the goroutine has exited
		return nil, err
	}

	_sharedVM = vm
	return _sharedVM, nil
}

// handleVMRequest runs the janet code within the dedicated VM thread.
// This function should only be called from the VM handler goroutine.
func handleVMRequest(
	env *C.JanetTable,
	req vmRequest,
) {
	var janetResult C.Janet
	var ret C.int

	cCode := C.CString(req.code)
	defer C.free(unsafe.Pointer(cCode))

	// create pipes for stdout and stderr
	var stdoutPipe [2]C.int
	var stderrPipe [2]C.int
	if C.pipe(&stdoutPipe[0]) != 0 {
		req.responseChan <- vmResponse{err: errors.New("failed to create stdout pipe")}
		return
	}
	if C.pipe(&stderrPipe[0]) != 0 {
		req.responseChan <- vmResponse{err: errors.New("failed to create stderr pipe")}
		return
	}

	// redirect stdout and stderr
	originalStdoutFd := C.redirectStdout(stdoutPipe[1])
	originalStderrFd := C.redirectStderr(stderrPipe[1])

	// run janet code
	ret = C.janet_dostring(env, cCode, nil, &janetResult)

	// restore stdout and stderr
	C.restoreStdout(originalStdoutFd)
	C.restoreStderr(originalStderrFd)

	// read all output from pipes
	var outBuf, errBuf bytes.Buffer
	buf := make([]byte, 1024)
	for {
		n, _ := C.read(stdoutPipe[0], unsafe.Pointer(&buf[0]), 1024)
		if n <= 0 {
			break
		}
		outBuf.Write(buf[:n])
	}
	for {
		n, _ := C.read(stderrPipe[0], unsafe.Pointer(&buf[0]), 1024)
		if n <= 0 {
			break
		}
		errBuf.Write(buf[:n])
	}
	C.close(stdoutPipe[0])
	C.close(stderrPipe[0])

	// and return the result
	if ret != C.JANET_SIGNAL_OK {
		var buffer C.JanetBuffer
		C.janet_buffer_init(&buffer, 0)
		C.janet_to_string_b(&buffer, janetResult)
		errOutput := C.GoStringN((*C.char)(unsafe.Pointer(buffer.data)), C.int(buffer.count))
		C.janet_buffer_deinit(&buffer)
		req.responseChan <- vmResponse{
			stdout: outBuf.String(),
			stderr: errBuf.String(),
			err:    errors.New(errOutput),
		}
		return
	}

	req.responseChan <- vmResponse{
		evaluated: janetValueToString(janetResult),
		stdout:    outBuf.String(),
		stderr:    errBuf.String(),
		err:       nil,
	}
}

// Close deinitializes the Janet VM.
func (vm *VM) Close() {
	if _sharedVM != nil {
		close(vm.shutdownChan)
		vm.wg.Wait()
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

// Execute executes a Janet `code` and returns the evaluated result, along with any output to stdout and stderr.
func (vm *VM) Execute(
	ctx context.Context,
	code string,
) (
	evaluated string,
	stdout string,
	stderr string,
	err error,
) {
	responseChan := make(chan vmResponse, 1)
	req := vmRequest{
		code:         code,
		responseChan: responseChan,
	}

	select {
	case vm.requestChan <- req:
		// request sent
	case <-ctx.Done():
		return "", "", "", ctx.Err()
	}

	select {
	case res := <-responseChan:
		return res.evaluated, res.stdout, res.stderr, res.err
	case <-ctx.Done():
		return "", "", "", ctx.Err()
	}
}
