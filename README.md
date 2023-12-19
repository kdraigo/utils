# kdraigo/utils

*** Providing functionality for managing asynchronous tasks. ***


# Concurrent Task Management in Go

This Go package provides functionality for managing concurrent tasks using futures and promises. It includes interfaces for handling asynchronous tasks, cancellation, timeouts, and event callbacks.

## Installation

```bash
go get -u github.com/kdraigo/utils
```

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/kdraigo/utils/pkg/sync"
)

func main() {
	// Create a context for the task.
	ctx := context.Background()

	// Define an asynchronous task function.
	asyncTask := func(promise sync.Promise[int]) {
		// Simulate a time-consuming task.
		time.Sleep(2 * time.Second)

		// Send the result through the promise.
		err := promise.Send(42)
		if err != nil {
			fmt.Println("Error sending data:", err)
		}
	}

	// Run the asynchronous task.
	future := sync.AsyncTask(ctx, 1, asyncTask)

	// Wait for the task to complete and get the result.
	result := future.Get()
	fmt.Println("Task result:", result)
}
```


```go

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/kdraigo/utils/pkg/sync"
)

func main() {
	// Create a context with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Define an asynchronous task function.
	asyncTask := func(promise sync.Promise[int]) {
		// Simulate a time-consuming task.
		time.Sleep(5 * time.Second)

		// Send the result through the promise.
		err := promise.Send(42)
		if err != nil {
			fmt.Println("Error sending data:", err)
		}
	}

	// Run the asynchronous task with a timeout.
	future := sync.AsyncTaskWithTimeOut(ctx, 1, 3*time.Second, asyncTask)

	// Wait for the task to complete and get the result.
	result := future.Get()
	fmt.Println("Task result:", result)
}

```
