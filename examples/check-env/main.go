// check-env verifies injection without displaying secret values or networking.
package main

import (
	"fmt"
	"os"
)

func main() {
	if _, ok := os.LookupEnv("EXAMPLE_SERVICE_TOKEN"); !ok {
		fmt.Fprintln(os.Stderr, "Example configuration is missing.")
		os.Exit(1)
	}
	fmt.Println("Example configuration is available.")
}
