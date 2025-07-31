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
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	tests := []struct {
		input string

		expectedEvaluated  string
		expectedStdout     string
		expectedStderr     string
		expectedErrPattern string
	}{
		////////////////////////////////
		// tests that should pass
		//
		// (arithmetic)
		{
			input:             `(+ 1 2 3)`,
			expectedEvaluated: `6`,
		},
		{
			input:             `(/ 42 10.0)`,
			expectedEvaluated: `4.2`,
		},
		{
			input:             `(/ 1 0)`,
			expectedEvaluated: `inf`,
		},
		// (integers and floats)
		{
			input:             `100`,
			expectedEvaluated: `100`,
		},
		{
			input:             `3.14`,
			expectedEvaluated: `3.14`,
		},
		{
			input:             `-2.718`,
			expectedEvaluated: `-2.718`,
		},
		{
			input:             `10.000000`,
			expectedEvaluated: `10`,
		},
		// (function declaration and call)
		{
			input:             `(defn add [x y] (+ x y))`,
			expectedEvaluated: `<function add>`,
		},
		{
			input:             `(add 1 2)`,
			expectedEvaluated: `3`,
		},
		// (tuples)
		{
			input:             `'(1 2 3)`,
			expectedEvaluated: `(1 2 3)`,
		},
		{
			input:             `'(1 2 (3 4) 5)`,
			expectedEvaluated: `(1 2 (3 4) 5)`,
		},
		// (nil)
		{
			input:             `nil`,
			expectedEvaluated: `nil`,
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

		// (standard out/err)
		{
			input:             `(print "hello to stdout") (eprint "hello to stderr") "hello"`,
			expectedEvaluated: `hello`,
			expectedStdout:    "hello to stdout\n",
			expectedStderr:    "hello to stderr\n",
		},
		{
			input: `(error "intentional")`,
			expectedStderr: `error: intentional
  in thunk pc=1
`,
			expectedErrPattern: "intentional",
		},
	}

	for _, test := range tests {
		result, stdout, stderr, err := vm.ExecuteString(context.TODO(), test.input)

		if err != nil {
			if test.expectedErrPattern == "" {
				t.Errorf("Unexpected error: %v", err)
			} else if !strings.Contains(err.Error(), test.expectedErrPattern) {
				t.Errorf("Expected error containing '%s', got '%s'", test.expectedErrPattern, err.Error())
			}
		} else if test.expectedErrPattern != "" {
			t.Errorf("Expected error containing '%s', but got none", test.expectedErrPattern)
		}

		if test.expectedEvaluated != "" && result != test.expectedEvaluated {
			t.Errorf("Input: %s\nExpected result: '%s', got: '%s'", test.input, test.expectedEvaluated, result)
		}

		if test.expectedStdout != "" && stdout != test.expectedStdout {
			t.Errorf("Input: %s\nExpected stdout: '%s', got: '%s'", test.input, test.expectedStdout, stdout)
		}

		if test.expectedStderr != "" && stderr != test.expectedStderr {
			t.Errorf("Input: %s\nExpected stderr: '%s', got: '%s'", test.input, test.expectedStderr, stderr)
		}
	}
}

// TestTimedoutExecuteString tests the ExecuteString function which times out.
func TestTimedoutExecuteString(t *testing.T) {
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	// (intentional) timedout execution
	timedoutCtx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	if _, _, _, err := vm.ExecuteString(timedoutCtx, `(os/sleep 3)`); err != nil {
		if !strings.Contains(err.Error(), `context deadline exceeded`) {
			t.Errorf("Expected timeout error, got '%s'", err)
		}
	} else {
		t.Errorf("Should have failed with context timeout error")
	}
}
