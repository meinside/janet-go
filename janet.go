//go:generate bash ./amalgamate.sh

// Package janet provides an interface to the Janet virtual machine.
package janet

/*
#cgo CFLAGS: -I./vendor/janet/src/include -I./vendor/janet/src/conf -I./vendor/janet/src/core -I./amalgamated
#cgo LDFLAGS: -lm -lpthread -ldl
#include "amalgamated/janet.c"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// VM represents a Janet virtual machine instance.
type VM struct {
	env *C.JanetTable
}

// NewVM initializes a new Janet VM.
func NewVM() (*VM, error) {
	C.janet_init()
	env := C.janet_core_env(nil)
	if env == nil {
		return nil, fmt.Errorf("failed to create janet environment")
	}
	return &VM{env: env}, nil
}

// Close deinitializes the Janet VM.
func (vm *VM) Close() {
	C.janet_deinit()
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
		num := C.janet_unwrap_number(value)
		if num == C.double(int(num)) {
			return fmt.Sprintf("%d", int(num))
		}
		return fmt.Sprintf("%f", num)
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
func (vm *VM) ExecuteString(code string) (string, error) {
	cCode := C.CString(code)
	defer C.free(unsafe.Pointer(cCode))
	var result C.Janet
	ret := C.janet_dostring(vm.env, cCode, nil, &result)

	if ret != C.JANET_SIGNAL_OK {
		var buffer C.JanetBuffer
		C.janet_buffer_init(&buffer, 0)
		C.janet_to_string_b(&buffer, result)
		errOutput := C.GoStringN((*C.char)(unsafe.Pointer(buffer.data)), C.int(buffer.count))
		C.janet_buffer_deinit(&buffer)
		return "", errors.New(errOutput)
	}

	return janetValueToString(result), nil
}
