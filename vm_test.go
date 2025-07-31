// vm_test.go

package janet

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestExecuteString tests the ExecuteString function.
func TestExecuteString(t *testing.T) {
	vm, err := NewVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	tests := []struct {
		input              string
		expectedResult     string
		expectedErrPattern string
	}{
		////////////////////////////////
		// tests that should pass
		//
		// (arithmetic)
		{
			input:          `(+ 1 2 3)`,
			expectedResult: `6`,
		},
		{
			input:          `(/ 42 10.0)`,
			expectedResult: `4.2`,
		},
		{
			input:          `(/ 1 0)`,
			expectedResult: `inf`,
		},
		// (integers and floats)
		{
			input:          `100`,
			expectedResult: `100`,
		},
		{
			input:          `3.14`,
			expectedResult: `3.14`,
		},
		{
			input:          `-2.718`,
			expectedResult: `-2.718`,
		},
		{
			input:          `10.000000`,
			expectedResult: `10`,
		},
		// (function declaration and call)
		{
			input:          `(defn add [x y] (+ x y))`,
			expectedResult: `<function add>`,
		},
		{
			input:          `(add 1 2)`,
			expectedResult: `3`,
		},
		// (tuples)
		{
			input:          `'(1 2 3)`,
			expectedResult: `(1 2 3)`,
		},
		{
			input:          `'(1 2 (3 4) 5)`,
			expectedResult: `(1 2 (3 4) 5)`,
		},
		// (nil)
		{
			input:          `nil`,
			expectedResult: `nil`,
		},

		////////////////////////////////
		// expected errors
		//
		// (malformed expressions)
		{
			input:              `(malformed expression`,
			expectedErrPattern: `unexpected end of source`,
		},
		// (unknown symbols)
		{
			input:              `(no-such-func 1 2 3)`,
			expectedErrPattern: `unknown symbol`,
		},
	}

	for _, test := range tests {
		output, err := vm.ExecuteString(context.TODO(), test.input)
		if err != nil {
			if !strings.Contains(err.Error(), test.expectedErrPattern) {
				t.Errorf("Expected error pattern '%s', got '%s'", test.expectedErrPattern, err)
			}
		} else {
			if output != test.expectedResult {
				t.Errorf("Expected '%s', got '%s'", test.expectedResult, output)
			}
		}
	}

	// (intentional) timedout execution
	timedoutCtx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	if _, err := vm.ExecuteString(timedoutCtx, `(os/sleep 5)`); err != nil {
		if !strings.Contains(err.Error(), `context deadline exceeded`) {
			t.Errorf("Expected timeout error, got '%s'", err)
		}
	} else {
		t.Errorf("Should have failed with context timeout error")
	}
}

// TestExecuteStringWithOutput tests the ExecuteStringWithOutput function.
func TestExecuteStringWithOutput(t *testing.T) {
	vm, err := NewVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	tests := []struct {
		input          string
		expectedResult string
		expectedStdout string
		expectedStderr string
		expectedErr    string
	}{
		// FIXME: fails often with `exit status 2`
		{
			input:          `(print "hello to stdout") (eprint "hello to stderr") "hello"`,
			expectedResult: `hello`,
			expectedStdout: "hello to stdout\n",
			expectedStderr: "hello to stderr\n",
		},
		{
			input:          `(error "intentional")`,
			expectedResult: "",
			expectedStdout: "",
			expectedStderr: `error: intentional
  in thunk pc=1
`,
			expectedErr: "intentional",
		},
	}

	for _, test := range tests {
		result, stdout, stderr, err := vm.ExecuteStringWithOutput(context.TODO(), test.input)

		if err != nil {
			if test.expectedErr == "" {
				t.Errorf("Unexpected error: %v", err)
			} else if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("Expected error containing '%s', got '%s'", test.expectedErr, err.Error())
			}
		} else if test.expectedErr != "" {
			t.Errorf("Expected error '%s', but got none", test.expectedErr)
		}

		if result != test.expectedResult {
			t.Errorf("Input: %s\nExpected result: '%s', got: '%s'", test.input, test.expectedResult, result)
		}

		if stdout != test.expectedStdout {
			t.Errorf("Input: %s\nExpected stdout: '%s', got: '%s'", test.input, test.expectedStdout, stdout)
		}

		if stderr != test.expectedStderr {
			t.Errorf("Input: %s\nExpected stderr: '%s', got: '%s'", test.input, test.expectedStderr, stderr)
		}
	}

	// (intentional) timedout execution
	timedoutCtx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	if _, _, _, err := vm.ExecuteStringWithOutput(timedoutCtx, `(os/sleep 5)`); err != nil {
		if !strings.Contains(err.Error(), `context deadline exceeded`) {
			t.Errorf("Expected timeout error, got '%s'", err)
		}
	} else {
		t.Errorf("Should have failed with context timeout error")
	}
}
