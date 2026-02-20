package main

import "github.com/Digni/ding-ding/cmd"

var Version = "dev"

func main() {
	cmd.Version = Version
	cmd.Execute()
}
