package main

import (
	"fmt"

	"github.com/bioflowy/bioflowy/pkg/jobs"
)

func main() {
	ps, err := jobs.LoadProcesses("pkg/jobs/test.yaml")
	if err != nil {
		fmt.Printf("%v", err)
	}
	ps.ExecuteAll()
	fmt.Printf("%v", ps)
}
