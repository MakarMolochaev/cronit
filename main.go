package main

import (
    "fmt"
    "os"
	"internal/storage"
	"os/exec"
)

func main() {
    arguments = os.Args[1:]

	if(arguments[0] == "start") {
		command = arguments[1]
		exec.Command("sh", "-c", arguments[1])
	} else if(arguments[0] == "add") {

	}
}