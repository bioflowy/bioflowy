package main

import (
	"github.com/bioflowy/bioflowy/pkg/jobs"
	"os"
)

func main() {
	ps, err := jobs.LoadProcesses(os.Args[1])
	if err != nil {
		panic(err)
	}
	err = ps.Execute()
	if err != nil {
		panic(err)
	}
}
