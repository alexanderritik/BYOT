// Build: go build -o ../byot-fail .
// Upload byot-fail with runtime "go" (default command runs ./artifact).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("BYOT fail fixture — simulating a broken production check")
	fmt.Fprintln(os.Stderr, "ERROR: assertion failed (expected pass, got fail)")
	os.Exit(1)
}
