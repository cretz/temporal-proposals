package temporalnexus

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	updated_sdk_client "go.temporal.io/sdk-updated/client"
	updated_sdk_converter "go.temporal.io/sdk-updated/converter"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type WorkerOptions struct {
	Nexus    NexusWorkerOptions
	Temporal worker.Options
}

// Combined Temporal and Nexus worker
type Worker struct {
	Nexus    *NexusWorker
	Temporal worker.Worker
}

func NewWorker(client updated_sdk_client.Client, options WorkerOptions) (*Worker, error) {
	panic("TODO")
}

func (w *Worker) RegisterALOWorkflow(
	workflow interface{},
	aloOptions RegisterALOWorkflowOptions,
	workflowOptions workflow.RegisterOptions,
) error {
	panic("TODO")
}

type NexusWorkerOptions struct {
}

// Worker for handling Nexus requests
type NexusWorker struct {
	*nexus.Worker
}

func NewNexusWorker(client updated_sdk_client.Client, options NexusWorkerOptions) (*NexusWorker, error) {
	panic("TODO")
}

type RegisterALOOptions struct {
	Service   string
	Operation string
}

type ALOContext struct {
	context.Context
	Request       *nexus.Request
	Client        updated_sdk_client.Client
	DataConverter converter.DataConverter
	Input         *updated_sdk_converter.PassthroughPayload
}

// Safe to call many times
func (a *ALOContext) MarkStarted(info *nexus.ALOInfo) {
	panic("TODO")
}

// TODO(cretz): Support details?
func (a *ALOContext) Heartbeat() {
	panic("TODO")
}

// TODO(cretz): Doc that this must be thread safe and will be called immediately
// if called after MarkStarted and a cancellation has already been requested at
// least once. Otherwise it could be called immediately on MarkStarted.
func (a *ALOContext) SetCancelCallback(func(ctx context.Context) error) {
	panic("TODO")
}

// ALO should be idempotent
func (n *NexusWorker) RegisterALO(
	alo func(*ALOContext) (*updated_sdk_converter.PassthroughPayload, error),
	options RegisterALOOptions,
) error {
	panic("TODO")
}

type WorkflowStarterOptions struct {
}

type RegisterALOWorkflowOptions struct {
	Service   string
	Operation string
	// Note, if any ID is present, it becomes the prefix of the real ALO ID
	// TODO(cretz): Disable all forms of ID reuse?
	StartOptions client.StartWorkflowOptions

	// Starter func(ctx context.Context, options RegisterALOWorkflowOptions, workflow interface{}, args ...interface{}) (WorkflowRun, error)
}

func (n *NexusWorker) RegisterALOWorkflow(workflow interface{}, options RegisterALOWorkflowOptions) error {
	// An ALO workflow is a workflow
	panic("TODO")
}

type ALOWorkflow struct {
	Workflow interface{}
	Options  RegisterALOWorkflowOptions
	// Defaults to 1s
	HeartbeatFrequency time.Duration
}

func (a *ALOWorkflow) WorkflowALO(ctx *ALOContext) (*updated_sdk_converter.PassthroughPayload, error) {
	// Copy start options and set ID as the request ID
	// TODO(cretz): Ok for ALO IDs to be reused like workflow IDs?
	startOptions := a.Options.StartOptions
	startOptions.ID += ctx.Request.RequestID

	var args []interface{}
	if ctx.Input != nil {
		args = []interface{}{ctx.Input}
	}

	// Get or start the workflow
	run, err := ctx.Client.ExecuteWorkflow(ctx, startOptions, a.Workflow, args...)
	if err != nil {
		return nil, err
	}

	// Set cancel callback
	ctx.SetCancelCallback(func(cancelCtx context.Context) error {
		// TODO(cretz): Should we use the run ID too?
		return ctx.Client.CancelWorkflow(cancelCtx, run.GetID(), "")
	})

	// Say we're now started. No status or metadata needed.
	ctx.MarkStarted(&nexus.ALOInfo{ID: ctx.Request.RequestID})

	// Wait for completion async. We don't use the given context because that
	respCh := make(chan *updated_sdk_converter.PassthroughPayload, 1)
	errCh := make(chan error, 1)
	go func() {
		var respVal updated_sdk_converter.PassthroughPayload
		if err := run.Get(ctx, &respVal); err != nil {
			errCh <- err
		} else {
			respCh <- &respVal
		}
	}()

	// Start heartbeat ticker
	heartbeatFreq := a.HeartbeatFrequency
	if heartbeatFreq == 0 {
		heartbeatFreq = 1 * time.Second
	}
	heartbeatTicker := time.NewTicker(heartbeatFreq)

	// Wait for heartbeat, context completion, or workflow completion
	for {
		select {
		case <-heartbeatTicker.C:
			ctx.Heartbeat()
		case <-ctx.Done():
			return nil, ctx.Err()
		case resp := <-respCh:
			return resp, nil
		case err := <-errCh:
			return nil, err
		}
	}
}
