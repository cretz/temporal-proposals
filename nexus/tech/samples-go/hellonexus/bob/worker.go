package main

import (
	"context"
	"fmt"

	updated_sdk_client "go.temporal.io/sdk-updated/client"
	"go.temporal.io/sdk-updated/worker/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

func RunWorker(ctx context.Context, client updated_sdk_client.Client) error {
	worker, err := temporalnexus.NewWorker(client, temporalnexus.WorkerOptions{})
	if err != nil {
		return err
	}
	// TODO(cretz): More...
}

func GreetingWorkflow(ctx workflow.Context, name string) (string, error) {
	return fmt.Sprintf("Hello, %v!", name), nil
}
