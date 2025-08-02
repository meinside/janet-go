// vm_test.go

package janet

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestExecutions tests the Execute function.
func TestExecutions(t *testing.T) {
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
		result, stdout, stderr, err := vm.Execute(context.TODO(), test.input)

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

// TestTimedoutExecutions tests the Execute function which times out.
func TestTimedoutExecutions(t *testing.T) {
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	// (intentional) timedout execution
	timedoutCtx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	if _, _, _, err := vm.Execute(timedoutCtx, `(os/sleep 3)`); err != nil {
		if !strings.Contains(err.Error(), `context deadline exceeded`) {
			t.Errorf("Expected timeout error, got '%s'", err)
		}
	} else {
		t.Errorf("Should have failed with context timeout error")
	}
}

// TestParseJanetString tests the ParseJanetString function.
func TestParseJanetString(t *testing.T) {
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	tests := []struct {
		input    string
		expected any
	}{
		{
			input:    `"hello, world!"`,
			expected: "hello, world!",
		},
		{
			input:    `123`,
			expected: float64(123),
		},
		{
			input:    `3.14`,
			expected: float64(3.14),
		},
		{
			input:    `nil`,
			expected: nil,
		},
		{
			input:    `true`,
			expected: true,
		},
		{
			input:    `false`,
			expected: false,
		},
		{
			input:    `'(1 2 "three")`,
			expected: []any{float64(1), float64(2), "three"},
		},
		{
			input:    `@["a" "b" "c"]`,
			expected: []any{"a", "b", "c"},
		},
		{
			input:    `@{:a 1 :b 2}`,
			expected: map[any]any{"a": float64(1), "b": float64(2)},
		},
		{
			input:    `@{:a 1 :b @{:c 3}}`,
			expected: map[any]any{"a": float64(1), "b": map[any]any{"c": float64(3)}},
		},
	}

	for _, test := range tests {
		value, err := vm.ParseJanetString(context.TODO(), test.input)
		if err != nil {
			t.Errorf("ParseJanetString failed for input '%s': %v", test.input, err)
		}

		if !reflect.DeepEqual(value, test.expected) {
			t.Errorf("Input: %s\nExpected: '%v', got: '%v'", test.input, test.expected, value)
		}
	}
}
