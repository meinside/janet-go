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

static int32_t janet_struct_len(JanetStruct st) {
    return janet_struct_head(st)->length;
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

// vmExecRequest is used to send a execution job to the VM handler goroutine.
type vmExecRequest struct {
	expression   string // janet expression
	responseChan chan vmExecResponse
}

// vmExecResponse is used to receive the execution result from the VM handler.
type vmExecResponse struct {
	evaluated string // evaluated janet expression
	stdout    string
	stderr    string
	err       error
}

// vmParseRequest is used to send a parse job to the VM handler goroutine.
type vmParseRequest struct {
	expression   string // janet expression
	responseChan chan vmParseResponse
}

// vmParseResponse is used to receive the parsed result from the VM handler.
type vmParseResponse struct {
	value any // parsed value (janet expression => go value)
	err   error
}

// VM represents a Janet virtual machine instance.
type VM struct {
	execChan     chan vmExecRequest  // for executing janet expression
	parseChan    chan vmParseRequest // for parsing janet expression
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

	execChan := make(chan vmExecRequest)
	parseChan := make(chan vmParseRequest)
	shutdownChan := make(chan struct{})

	vm = &VM{
		execChan:     execChan,
		parseChan:    parseChan,
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
			case req := <-execChan:
				handleExecRequest(env, req)
			case req := <-parseChan:
				handleParseRequest(env, req)
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

// handleExecRequest executes the janet expression within the dedicated VM thread.
// This function should only be called from the VM handler goroutine.
func handleExecRequest(
	env *C.JanetTable,
	req vmExecRequest,
) {
	var janetResult C.Janet
	var ret C.int

	cCode := C.CString(req.expression)
	defer C.free(unsafe.Pointer(cCode))

	// create pipes for stdout and stderr
	var stdoutPipe [2]C.int
	var stderrPipe [2]C.int
	if C.pipe(&stdoutPipe[0]) != 0 {
		req.responseChan <- vmExecResponse{err: errors.New("failed to create stdout pipe")}
		return
	}
	if C.pipe(&stderrPipe[0]) != 0 {
		// close opened pipes that were opened above
		C.close(stdoutPipe[0])
		C.close(stdoutPipe[1])

		req.responseChan <- vmExecResponse{err: errors.New("failed to create stderr pipe")}
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
		req.responseChan <- vmExecResponse{
			stdout: outBuf.String(),
			stderr: errBuf.String(),
			err:    errors.New(errOutput),
		}
		return
	}

	req.responseChan <- vmExecResponse{
		evaluated: janetValueToString(janetResult),
		stdout:    outBuf.String(),
		stderr:    errBuf.String(),
		err:       nil,
	}
}

// handleParseRequest parses the janet string within the dedicated VM thread.
func handleParseRequest(
	env *C.JanetTable,
	req vmParseRequest,
) {
	var janetResult C.Janet
	var ret C.int

	cCode := C.CString(req.expression)
	defer C.free(unsafe.Pointer(cCode))

	// run janet code
	ret = C.janet_dostring(env, cCode, nil, &janetResult)

	if ret != C.JANET_SIGNAL_OK {
		var buffer C.JanetBuffer
		C.janet_buffer_init(&buffer, 0)
		C.janet_to_string_b(&buffer, janetResult)
		errOutput := C.GoStringN((*C.char)(unsafe.Pointer(buffer.data)), C.int(buffer.count))
		C.janet_buffer_deinit(&buffer)
		req.responseChan <- vmParseResponse{
			err: errors.New(errOutput),
		}
		return
	}

	req.responseChan <- vmParseResponse{
		value: parseJanetValueToGo(janetResult),
		err:   nil,
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
	case C.JANET_KEYWORD:
		return ":" + C.GoString((*C.char)(unsafe.Pointer(C.janet_unwrap_keyword(value))))
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

// parseJanetValueToGo converts a Janet value to its Go value.
func parseJanetValueToGo(value C.Janet) any {
	switch C.janet_type(value) {
	case C.JANET_NIL:
		return nil
	case C.JANET_BOOLEAN:
		return C.janet_unwrap_boolean(value) != 0
	case C.JANET_NUMBER:
		return float64(C.janet_unwrap_number(value))
	case C.JANET_STRING:
		return C.GoString((*C.char)(unsafe.Pointer(C.janet_unwrap_string(value))))
	case C.JANET_SYMBOL:
		return C.GoString((*C.char)(unsafe.Pointer(C.janet_unwrap_symbol(value))))
	case C.JANET_KEYWORD:
		return ":" + C.GoString((*C.char)(unsafe.Pointer(C.janet_unwrap_keyword(value))))
	case C.JANET_TUPLE, C.JANET_ARRAY:
		var data *C.Janet
		var length C.int32_t
		C.janet_indexed_view(value, &data, &length)
		slice := make([]any, length)
		for i := C.int32_t(0); i < length; i++ {
			elem := *(*C.Janet)(unsafe.Pointer(uintptr(unsafe.Pointer(data)) + uintptr(i)*unsafe.Sizeof(*data)))
			slice[i] = parseJanetValueToGo(elem)
		}
		return slice
	case C.JANET_TABLE:
		table := C.janet_unwrap_table(value)
		result := make(map[any]any)
		for i := C.int32_t(0); i < table.capacity; i++ {
			currentKV := (*C.JanetKV)(unsafe.Pointer(uintptr(unsafe.Pointer(table.data)) + uintptr(i)*unsafe.Sizeof(*table.data)))
			if C.janet_checktype(currentKV.key, C.JANET_NIL) == 0 {
				key := parseJanetValueToGo(currentKV.key)
				val := parseJanetValueToGo(currentKV.value)
				result[key] = val
			}
		}
		return result
	case C.JANET_STRUCT:
		kv := C.janet_unwrap_struct(value)
		length := C.janet_struct_len(kv)
		result := make(map[any]any)
		for i := range length {
			currentKV := (*C.JanetKV)(unsafe.Pointer(uintptr(unsafe.Pointer(kv)) + uintptr(i)*unsafe.Sizeof(*kv)))
			if C.janet_checktype(currentKV.key, C.JANET_NIL) == 0 {
				key := parseJanetValueToGo(currentKV.key)
				val := parseJanetValueToGo(currentKV.value)
				result[key] = val
			}
		}
		return result
	default:
		// For other complex types, fallback to string representation
		return janetValueToString(value)
	}
}

// Execute executes a `janetExpression` and returns the evaluated result, along with any output to stdout and stderr.
func (vm *VM) Execute(
	ctx context.Context,
	janetExpression string,
) (
	evaluated string,
	stdout string,
	stderr string,
	err error,
) {
	responseChan := make(chan vmExecResponse, 1)
	req := vmExecRequest{
		expression:   janetExpression,
		responseChan: responseChan,
	}

	select {
	case vm.execChan <- req:
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

// ParseToValue parses a `janetExpression` containing janet data into a Go value.
func (vm *VM) ParseToValue(
	ctx context.Context,
	janetExpression string,
) (
	value any,
	err error,
) {
	responseChan := make(chan vmParseResponse, 1)
	req := vmParseRequest{
		expression:   janetExpression,
		responseChan: responseChan,
	}

	select {
	case vm.parseChan <- req:
		// request sent
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case res := <-responseChan:
		return res.value, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
