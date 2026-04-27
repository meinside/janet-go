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
	"context"
	"fmt"
	"log"

	"github.com/meinside/janet-go"
)

func main() {
	ver := janet.Version()
	log.Printf("Janet version: %s", ver)

	vm, err := janet.SharedVM()
	if err != nil {
		log.Fatalf("Failed to create Janet VM: %v", err)
	}
	defer vm.Close()

	ctx := context.Background()

	// Execute a simple expression
	res, err := vm.Execute(ctx, "(+ 1 2 3)")
	if err != nil {
		log.Fatalf("Failed to execute Janet code: %v", err)
	}
	fmt.Println(res.Evaluated) // Output: 6

	// Define a function
	if _, err := vm.Execute(ctx, "(defn add [x y] (+ x y))"); err != nil {
		log.Fatalf("Failed to execute Janet code: %v", err)
	}

	// and call that function
	res, err = vm.Execute(ctx, "(add 10 20)")
	if err != nil {
		log.Fatalf("Failed to execute Janet code: %v", err)
	}
	fmt.Println(res.Evaluated) // Output: 30

	// Execute a malformed expression (that will lead to an error)
	if _, err := vm.Execute(ctx, "(malformed expression"); err != nil {
		fmt.Println(err) // Output: unexpected end of source, ( opened at line 1, column 1
	}
}
```

## Concurrency

`SharedVM()` returns a process-wide singleton `VM` backed by a dedicated
OS-thread-locked goroutine. All `Execute` and `ParseToValue` calls are
forwarded to that goroutine and serialized internally, so the VM is safe
to use from multiple goroutines without external synchronization — calls
are queued and processed one at a time.

`Execute` and `ParseToValue` accept a `context.Context`. If the context is
cancelled (or its deadline elapses) before the request is dispatched or
before the result arrives, the call returns `ctx.Err()`. Note that
cancellation does **not** interrupt Janet evaluation already in progress;
the work continues on the VM goroutine until it finishes.

`Close()` is idempotent. After closing the shared VM, a subsequent call
to `SharedVM()` will start a fresh VM.

## Note

### Amalgamation

`amalgamated/janet.c`, `amalgamated/janet.h`, and `amalgamated/janetconf.h` are generated from the [source code](https://github.com/janet-lang/janet) with `amalgamate.sh`.

They need to be updated when there is a new release of Janet.

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.

