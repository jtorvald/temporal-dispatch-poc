package main

import (
	"context"
	"flag"
	"github.com/jtorvald/temporal-dispatch-poc/api"
	"github.com/jtorvald/temporal-dispatch-poc/workflows"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// turned off go:generate schema-generate -p schema -i ../../schema/dispatch-workflow.schema.json -o ../../schema/workflow_schema_generated.go
func main(){

	var apiAddr, temporalAddr, queue string

	flag.StringVar(&apiAddr, "api", "localhost:8888", "interface and port to have the API listen on (default: localhost:8888)")
	flag.StringVar(&temporalAddr, "temporal", "localhost:7233", "host and port that temporal is listening on (default: localhost:7233)")
	flag.StringVar(&queue, "queue", "dispatch", "the temporal queue to work with (default: dispatch")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	go func(){
		if err := run(ctx, apiAddr, temporalAddr, queue); err != nil {
			panic(err)
		}
	}()

	// Handle interrupt signals
	ch := make(chan os.Signal, 5)

	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-ch:
		switch sig {
		// INFO: Ctrl + T, proess info
		// PIP: failed write to file descriptor
		// TERM: Sent by kill <pid>
		case os.Interrupt: // ctrl+c
			fallthrough
		case syscall.SIGQUIT: // Ctrl + \, core dumps

			log.Println("")
			log.Println("Got interrupt....")

		default:
			log.Println("Unknown signal: ", sig)
		}
	}

	cancel()

	// give server time to shut down properly
	time.Sleep(4 * time.Second)
}

func run(ctx context.Context, apiAddr, temporalAddr, queue string) error {

	client, err := workflows.NewWorkflowStarter(temporalAddr, queue)
	if err != nil {
		return err
	}
	// start api and listen on port 8888 in the background
	api.ListenAndServe(ctx, apiAddr, client)

	// start the background worker
	go client.StartWorkflowWorker(ctx)
	log.Println("Listening to Temporal server: ", temporalAddr, " on queue: ", queue)

	return nil
}
