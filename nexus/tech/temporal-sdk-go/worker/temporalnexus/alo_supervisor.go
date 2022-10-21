package temporalnexus

import (
	"context"
	"fmt"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	commonpb "go.temporal.io/api/common/v1"
	failurepb "go.temporal.io/api/failure/v1"
	updated_sdk_client "go.temporal.io/sdk-updated/client"
	updated_sdk_converter "go.temporal.io/sdk-updated/converter"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

const (
	supervisorALOStartedSignal = "alo-started"
	supervisorCancelALOSignal  = "cancel-alo"
	supervisorGetALOInfoQuery  = "get-alo-info"
)

type supervisedALOOptions struct {
	Request          *nexus.Request
	HeartbeatTimeout time.Duration
}

type supervisedALOResponse struct {
	Success *updated_sdk_converter.PassthroughPayload
}

func aloSupervisorWorkflow(ctx workflow.Context, opts *supervisedALOOptions) (*updated_sdk_converter.PassthroughPayload, error) {
	// Query for ALO info. This is only present when started.
	var aloInfo *nexus.ALOInfo
	queryErr := workflow.SetQueryHandler(ctx, supervisorGetALOInfoQuery, func() (*nexus.ALOInfo, error) {
		return aloInfo, nil
	})
	if queryErr != nil {
		return nil, queryErr
	}

	// Build activity options
	// TODO(cretz): Where do these options come from? Can make them configurable,
	// but in what ways?
	workflowInfo := workflow.GetInfo(ctx)
	if workflowInfo.WorkflowExecutionTimeout == 0 {
		return nil, fmt.Errorf("supervisor workflow requires execution timeout")
	}
	actCtx, actCancel := workflow.WithCancel(ctx)
	actCtx = workflow.WithActivityOptions(actCtx, workflow.ActivityOptions{
		// TODO(cretz): Do we care how long the ALO activity takes to start on each
		// retry? Probably not...
		// ScheduleToStartTimeout: ...,

		// TODO(cretz): We expect activity to be able to run as long as the entire
		// workflow, but is this acceptable?
		StartToCloseTimeout: workflowInfo.WorkflowExecutionTimeout,

		// TODO(cretz): Default this to a pretty low number
		HeartbeatTimeout: opts.HeartbeatTimeout,

		// We send user cancel as activity cancel, so we wait
		WaitForCancellation: true,
	})

	// Start the activity
	var a *aloSupervisorActivities
	fut := workflow.ExecuteActivity(ctx, a.RunALO, opts)

	// Prepare the primary selector
	var success *updated_sdk_converter.PassthroughPayload
	var failure error
	sel := workflow.NewSelector(ctx)

	// Wait for activity completion
	sel.AddFuture(fut, func(fut workflow.Future) { failure = fut.Get(ctx, success) })

	// Wait for activity to report started. We're ok receiving this signal many
	// times (like for activity retries).
	sel.AddReceive(
		workflow.GetSignalChannel(ctx, supervisorALOStartedSignal),
		func(ch workflow.ReceiveChannel, _ bool) { ch.ReceiveAsync(aloInfo) },
	)

	// Single-use timer for failing if the activity hasn't started quickly
	// TODO(cretz): Configurable
	const aloStartTimeout = 30 * time.Second
	sel.AddFuture(workflow.NewTimer(ctx, aloStartTimeout), func(fut workflow.Future) {
		_ = fut.Get(ctx, nil)
		if aloInfo == nil {
			failure = fmt.Errorf("ALO did not start within %v", aloStartTimeout)
		}
	})

	// Wait for user cancel request. We're ok receiving this signal many times and
	// we will trigger an activity cancel each time, but of course it only applies
	// the first time.
	sel.AddReceive(
		workflow.GetSignalChannel(ctx, supervisorCancelALOSignal),
		func(ch workflow.ReceiveChannel, more bool) {
			ch.ReceiveAsync(nil)
			actCancel()
		},
	)

	// Run continuously until error or success
	for success == nil && failure == nil {
		sel.Select(ctx)
	}

	// Invoke callback if on request
	if opts.Request.HTTPCallback != nil {
		// TODO(cretz): Make these configurable
		cbCtx := workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
			ScheduleToCloseTimeout: 20 * time.Second,
		})
		// TODO(cretz): Local activity acceptable?
		cbOpts := &invokeCallbackOptions{Callback: opts.Request.HTTPCallback}
		if success != nil {
			cbOpts.Success = success.Payload
		} else {
			// TODO(cretz): Convert
			// cbOpts.Failure = temporal.GetDefaultFailureConverter().ErrorToFailure(failure)
		}
		if cbErr := workflow.ExecuteLocalActivity(cbCtx, a.InvokeCallback, cbOpts).Get(ctx, nil); cbErr != nil {
			workflow.GetLogger(ctx).Warn("Failed invoking callback", "error", cbErr)
		}
	}

	return success, failure
}

type serviceOperation struct {
	service   string
	operation string
}

type invokeCallbackOptions struct {
	Callback *nexus.HTTPCallback
	Success  *commonpb.Payload
	Failure  *failurepb.Failure
}

type aloSupervisorActivities struct {
	client        updated_sdk_client.Client
	dataConverter converter.DataConverter
	alos          map[serviceOperation]func(*ALOContext) (*updated_sdk_converter.PassthroughPayload, error)
}

func (a *aloSupervisorActivities) RunALO(
	ctx context.Context,
	opts *supervisedALOOptions,
) (*updated_sdk_converter.PassthroughPayload, error) {
	// TODO(cretz): Build ALO context and invoke ALO that matches operation
	panic("TODO")
}

func (a *aloSupervisorActivities) InvokeCallback(
	ctx context.Context,
	opts *invokeCallbackOptions,
) error {
	// TODO(cretz): Make HTTP callback
	panic("TODO")
}
