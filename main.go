package main

import (
	"flag"
	"sync"

	"github.com/golang/glog"
	"github.com/rockstardevs/parser/ofx"
)

var (
	numWorkers = flag.Int("num_workers", 3, "number of workers to process files concurrently.")
	jobsBuffer = flag.Int("jobs_buffer_size", 10, "numbers of jobs to buffer at a time.")
)

// processFile processes an individual input file.
func processFile(index int, jobs <-chan string, wg *sync.WaitGroup) {
	glog.Infof("started worker %d", index)
	defer wg.Done()
	for filename := range jobs {
		glog.Infof("worker %d: processing %s", index, filename)
		document, err := ofx.NewDocumentFromXML(filename)
		if err != nil {
			glog.Errorf("worker %d: error processing %s - %s", index, filename, err)
		}
		// TODO: do something with the parsed document.
		glog.Infof("%v", document)
	}
	glog.Infof("shutting down worker %d", index)
}

func main() {
	flag.Parse()

	// Start workers
	var wg sync.WaitGroup
	jobsChan := make(chan string, *jobsBuffer)
	for i := 0; i <= *numWorkers; i++ {
		go processFile(i, jobsChan, &wg)
		wg.Add(1)
	}

	// Distribute files for processing.
	for _, filename := range flag.Args() {
		jobsChan <- filename
	}
	close(jobsChan)

	// Block for all files to be processed.
	wg.Wait()
}
