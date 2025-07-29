# janet-go

This is a Go library that provides a wrapper around the [Janet](https://janet-lang.org/) programming language.

It allows you to embed a Janet VM in your Go programs and run Janet codes.

## Installation

```bash
go get -u github.com/meinside/janet-go
```

## Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/meinside/janet-go"
)

func main() {
	vm, err := janet.NewVM()
	if err != nil {
		log.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	// Execute a simple expression
	output, err := vm.ExecuteString("(+ 1 2 3)")
	if err != nil {
		log.Fatalf("Failed to execute Janet code: %v", err)
	}
	fmt.Println(output) // Output: 6

	// Define a function
	_, err = vm.ExecuteString("(defn add [x y] (+ x y))")
	if err != nil {
		log.Fatalf("Failed to execute Janet code: %v", err)
	}

	// and call that function
	output, err = vm.ExecuteString("(add 10 20)")
	if err != nil {
		log.Fatalf("Failed to execute Janet code: %v", err)
	}
	fmt.Println(output) // Output: 30

	// Execute a malformed expression (that will lead to an error)
	_, err = vm.ExecuteString("(malformed expression")
	if err != nil {
		fmt.Println(err) // Output: unexpected end of source, ( opened at line 1, column 1
	}
}
```

## Note

### Amalgamation

`amalgamated/janet.h` and `amalgamated/janet.c` are generated from the [source code](https://github.com/janet-lang/janet) with `amalgamate.sh`.

They need to be updated when there is a new release of Janet.

`amalgamate.sh` is also used by `//go:generate` in `janet.go` file.

For clearing files compiled by cgo, run `go clean -cache`.

## License

MIT

