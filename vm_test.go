// vm_test.go

package janet

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestVersion tests the Version function.
func TestVersion(t *testing.T) {
	if ver := Version(); len(ver) <= 0 {
		t.Errorf("Failed to get Janet version")
	}
}

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
		// (keywords)
		{
			input:             `:hello`,
			expectedEvaluated: `:hello`,
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
		res, err := vm.Execute(context.Background(), test.input)

		if err != nil {
			if test.expectedErrPattern == "" {
				t.Errorf("Unexpected error: %v", err)
			} else if !strings.Contains(err.Error(), test.expectedErrPattern) {
				t.Errorf("Expected error containing '%s', got '%s'", test.expectedErrPattern, err.Error())
			}
		} else if test.expectedErrPattern != "" {
			t.Errorf("Expected error containing '%s', but got none", test.expectedErrPattern)
		}

		if test.expectedEvaluated != "" && res.Evaluated != test.expectedEvaluated {
			t.Errorf("Input: %s\nExpected result: '%s', got: '%s'", test.input, test.expectedEvaluated, res.Evaluated)
		}

		if test.expectedStdout != "" && res.Stdout != test.expectedStdout {
			t.Errorf("Input: %s\nExpected stdout: '%s', got: '%s'", test.input, test.expectedStdout, res.Stdout)
		}

		if test.expectedStderr != "" && res.Stderr != test.expectedStderr {
			t.Errorf("Input: %s\nExpected stderr: '%s', got: '%s'", test.input, test.expectedStderr, res.Stderr)
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
	timedoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if _, err := vm.Execute(timedoutCtx, `(os/sleep 3)`); err != nil {
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
			input: `'(1 2 "three")`,
			expected: []any{
				float64(1),
				float64(2),
				"three",
			},
		},
		{
			input: `@["a" "b" "c"]`,
			expected: []any{
				"a",
				"b",
				"c",
			},
		},
		{
			input: `@{:a 1 :b 2}`,
			expected: map[any]any{
				":a": float64(1),
				":b": float64(2),
			},
		},
		{
			input: `@{:a 1 :b @{:c 3}}`,
			expected: map[any]any{
				":a": float64(1),
				":b": map[any]any{
					":c": float64(3),
				},
			},
		},
	}

	for _, test := range tests {
		value, err := vm.ParseToValue(context.Background(), test.input)
		if err != nil {
			t.Errorf("ParseJanetString failed for input '%s': %v", test.input, err)
		}

		if !reflect.DeepEqual(value, test.expected) {
			t.Errorf("Input: %s\nExpected: '%v', got: '%v'", test.input, test.expected, value)
		}
	}
}

// TestLargeStdout verifies that output exceeding a single pipe buffer
// (~64 KiB on Linux) does not deadlock the VM. Regression test for the
// concurrent pipe-drain fix.
func TestLargeStdout(t *testing.T) {
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	const lines = 2000 // ~128 KiB of output, well past the 64 KiB pipe buffer
	expr := fmt.Sprintf(`(loop [i :range [0 %d]] (print "line-" i))`, lines)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := vm.Execute(ctx, expr)
	if err != nil {
		t.Fatalf("Execute failed on large output: %v", err)
	}

	got := strings.Count(res.Stdout, "\n")
	if got != lines {
		t.Errorf("Expected %d newlines in stdout, got %d (stdout len=%d)", lines, got, len(res.Stdout))
	}
}

// TestDoubleClose verifies Close() is idempotent.
func TestDoubleClose(t *testing.T) {
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	vm.Close()
	vm.Close() // must not panic
}

// TestReopenAfterClose verifies that SharedVM() works again after Close.
func TestReopenAfterClose(t *testing.T) {
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	vm.Close()

	vm2, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to re-create Janet VM after Close: %v", err)
	}
	defer vm2.Close()

	res, err := vm2.Execute(context.Background(), `(+ 1 2)`)
	if err != nil {
		t.Fatalf("Execute failed on reopened VM: %v", err)
	}
	if res.Evaluated != "3" {
		t.Errorf("Expected '3', got '%s'", res.Evaluated)
	}
}

// TestConcurrentSharedVM verifies that concurrent SharedVM() callers all
// receive the same (single) VM instance.
func TestConcurrentSharedVM(t *testing.T) {
	const n = 16
	var wg sync.WaitGroup
	vms := make([]*VM, n)
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			v, err := SharedVM()
			if err != nil {
				t.Errorf("SharedVM error: %v", err)
				return
			}
			vms[i] = v
		}(i)
	}
	wg.Wait()

	first := vms[0]
	defer first.Close()
	if first == nil {
		t.Fatal("SharedVM returned nil")
	}
	for i, v := range vms {
		if v != first {
			t.Errorf("vm[%d] is a different instance than vm[0]", i)
		}
	}
}

// TestStatePersistence verifies that definitions persist across Execute calls.
func TestStatePersistence(t *testing.T) {
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	ctx := context.Background()

	if _, err := vm.Execute(ctx, `(def magic 42)`); err != nil {
		t.Fatalf("def failed: %v", err)
	}
	res, err := vm.Execute(ctx, `(* magic 2)`)
	if err != nil {
		t.Fatalf("use-of-def failed: %v", err)
	}
	if res.Evaluated != "84" {
		t.Errorf("Expected '84', got '%s'", res.Evaluated)
	}
}

// TestEmptyCollections covers empty array/tuple/table/struct parsing.
func TestEmptyCollections(t *testing.T) {
	vm, err := SharedVM()
	if err != nil {
		t.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	tests := []struct {
		input    string
		expected any
	}{
		{`'()`, []any{}},
		{`@[]`, []any{}},
		{`@{}`, map[any]any{}},
		{`{}`, map[any]any{}},
	}
	for _, tt := range tests {
		got, err := vm.ParseToValue(context.Background(), tt.input)
		if err != nil {
			t.Errorf("ParseToValue(%q) error: %v", tt.input, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("ParseToValue(%q) = %#v, want %#v", tt.input, got, tt.expected)
		}
	}
}
