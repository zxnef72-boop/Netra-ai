//go:build ignore
package main

import "fmt"

func main() {
	log, err := cmdBuildWinInstaller("/data/data/com.termux/files/home/netra-tank", "Test-Tank")
	fmt.Println("=== LOG ===")
	fmt.Println(log)
	fmt.Println("=== ERR ===")
	fmt.Println(err)
}
