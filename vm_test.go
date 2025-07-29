// vm_test.go

package janet

import (
	"strings"
	"testing"
)

// TestVM tests the Janet VM with a few simple expressions.
func TestVM(t *testing.T) {
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
		output, err := vm.ExecuteString(test.input)
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
}
