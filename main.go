package main

import "github.com/bariiss/coprem/cmd"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	_ = version
	_ = commit
	_ = date
	cmd.Execute()
}
