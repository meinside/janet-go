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
		input       string
		expected    string
		expectedErr string
	}{
		// tests that should pass
		{
			input:    "(+ 1 2 3)",
			expected: "6",
		},
		{
			input:    "(/ 42 10.0)",
			expected: "4.200000",
		},
		{
			input:    "(defn add [x y] (+ x y))",
			expected: "<function add>",
		},
		{
			input:    "(add 1 2)",
			expected: "3",
		},
		{
			input:    "nil",
			expected: "nil",
		},
		{
			input:    "'(1 2 3)",
			expected: "(1 2 3)",
		},
		{
			input:    "'(1 2 (3 4) 5)",
			expected: "(1 2 (3 4) 5)",
		},

		// expected errors
		{
			input:       "(malformed expression",
			expectedErr: "unexpected end of source",
		},
	}

	for _, test := range tests {
		output, err := vm.ExecuteString(test.input)
		if err != nil {
			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("Expected error '%s', got '%s'", test.expectedErr, err)
			}
		} else {
			if output != test.expected {
				t.Errorf("Expected '%s', got '%s'", test.expected, output)
			}
		}
	}
}
