package main

import (
	"fmt"
	"os/exec"
)

func main() {

	var value string

	n, err := fmt.Scan(&value)
	if err != nil || n == 0 {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(value)

	cmd := exec.Command(value)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(string(output))

	//
}
