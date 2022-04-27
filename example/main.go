package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/ieee0824/funnel"
)

func main() {
	f := funnel.New(1, context.TODO())
	var wg sync.WaitGroup

	for _, job := range []*funnel.Job{
		{
			Command: "ls",
			Options: []string{
				"-lh",
			},
		},
		{
			Command: "sh",
			Options: []string{
				"-c",
				"sleep 10; ls",
			},
		},
	} {
		wg.Add(1)
		go func(job *funnel.Job, wg *sync.WaitGroup) {
			defer wg.Done()
			out, err := f.Request(job)
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Println(out)
		}(job, &wg)
	}

	f.Wg().Wait()
	wg.Wait()
}
