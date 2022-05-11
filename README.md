# funnel

This is a library that controls the number of concurrent OS command executions.

# example

```Go
    const maxProcess = 1
	f := funnel.New(maxProcess, context.TODO())
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

	// wait all request completed
	// this wait group is used to stop safely
	f.Wg().Wait()
	// wait receive
	wg.Wait()
```