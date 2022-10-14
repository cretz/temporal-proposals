# Nexus Technical Proposal

In addition to normal documentation, some files have areas with `NOTES` for some discussion about design decisions.

### Included

* [api/nexus/backend/v1](api/nexus/backend/v1) - Service that a Nexus backend must implement to support Nexus
  invocations and workers.
* [api/temporal/api](api/temporal/api) - Updates to Temporal workflow commands and events to support Nexus.
* [sdk-go](sdk-go) - The Nexus Go SDK. This is expected to have everything needed for both callers and worker
  implementers to use Nexus. There is no Temporal code in here.
* [temporal-sdk-go](temporal-sdk-go) - Alterations/additions to the Temporal Go SDK to support Nexus callers from
  Temporal workflows and helpers for Nexus workers to do Temporal things like starting a workflow.

### Not Included

* Nexus frontend
  * This is an exposed HTTP and/or gRPC gateway (often on the same server as the backend) in front of these raw backend
    APIs to provide a friendlier invocation interface and possible service definitions.
* Nexus CLI
  * A friendlier interface to Nexus backends for registering/listing services, making invocations, etc.
* Nexus client
  * Programmatic client in the Nexus SDK for making Nexus calls

### Sequence - Temporal Workflow to Temporal Workflow Invocations

![Temporal Workflow to Temporal Workflow](./diagrams/temporal-to-temporal-workflow.png)

How it works in detail:

#### From Alice's POV

1. Alice, inside her workflow calls `ExecuteNexus` which is meant to _start_ a Nexus call and return a future.
1. Alice's Temporal server receives this schedule Nexus command and:
   1. Looks up the actual endpoint from the given string endpoint name via some user-definable config mapping (it's
      Bob's Nexus server).
   1. Persists the Nexus schedule event.
   1. Makes a call to the looked up Nexus gRPC endpoint which is Bob's. This would likely set a callback on the call to
      Alice's Temporal server (server impl detail, but probably easier than long-polling which is also an option).
   1. Persists the results of this call as a Nexus completed event (which may be a completed value or an ALO reference)
1. Alice's workflow receives the completed event about the completed Nexus call (which doesn't mean a completed ALO if
   it is an ALO, just "started").
   1. Technically at this time Alice could wait on the "started" part of that `ExecuteNexus` future and it'd be
      resolved. For non-ALOs this is the same as waiting on the regular future and it's resolved with the result. For
      ALOs the started future is resolved with the ALO info and the regular future is not resolved yet. The SDK has
      enough information to differentiate.
1. Assuming ALO henceforth, once complete, Bob's Nexus server invokes to configured callback telling Alice's server that
   the ALO is complete.
1. Alice's server persists the completion event and workflow is notified.
1. Alice, listening on the primary future of `ExecuteNexus` now has that resolved with the result.

Alice's code might look something like:

```go
func AliceWorkflow(ctx workflow.Context) error {
  // ...

  // The context+options pattern just for consistency. Obviously it's not the
  // best way to do these things in Go compared to explicit options on the call.
  ctx = workflow.WithNexusOptions(ctx, workflow.NexusOptions{Service: "bob-service"})
  fut := workflow.ExecuteNexus(ctx, "bob-alo-call", "some parameter")
  var response string
  if err := fut.Get(ctx, &response); err != nil {
    return err
  }
  // Do stuff with response...
}
```

#### From Bob's POV

Note, this is for Temporal+Nexus Workflow-as-ALO. Generic Nexus handlers may do something different.

Nexus worker-side definitions for the purposes of this flow (not their actual names because they are confusing):

* Nexus Request Handler - implicit, low-level handler that accepts a Nexus request and starts the Nexus Handler Workflow
  and waits until it has said "started" or "completed".
* Nexus Handler Workflow - implicit workflow wrapping all Nexus calls (invokes Nexus Handler Activity and invokes
  callback upon ALO completion)
* Nexus Handler Activity - implicit activity that just invokes the actual Nexus Handler
* Nexus Handler - code Bob could write, but we have defaults for 1:1 starting workflows

Flow:

1. Upon receipt of Alice's Nexus request in Bob's Nexus server, the request is sent to a polling Nexus worker.
1. Upon receipt of Alice's Nexus request in Bob's Nexus Request Handler, a lookup is done to see which Nexus Handler
   this corresponds to, and then a Nexus Handler Workflow is started to handle that request.
1. Upon receipt of Alice's Nexus request in Bob's Nexus Handler Workflow, an activity is started to handle that request.
1. Upon receipt of Alice's Nexus request in Bob's Nexus Handler Activity, Bob's Nexus handler is invoked to run the
   request.
1. Bob's Nexus Handler (usually not hand-coded, but could be) uses a Temporal client to get-or-start whatever Temporal
   workflow corresponds to the request. Once started, the parent workflow is signalled with this "ALO info".
   1. To be idempotent, it's a get-or-start and the parent workflow is smart enough to discard duplicate signals
1. Bob's Nexus Handler Workflow receives this signal and notifies Bob's Nexus Request Handler that the ALO has started
   1. This is a use case for sync update, but for now can use one of the hacky req/resp approaches to signal out from
      workflow.
1. Bob's Nexus Request Handler completes to Alice with the started ALO info.
1. Bob's Nexus Handler is waiting on Temporal workflow completion via the client, heartbeating all the while.
1. Bob's Nexus Handler gets Temporal workflow completion and completes itself which completes Bob's Nexus Handler
   Activity.
1. Bob's Nexus Workflow gets the activity completion and invokes Alice's ALO completion callback via another activity
   and then completes itself.

While this may seem convoluted, it's important for flexibility. But it's simplified for the common 1:1 Temporal workflow
use case. For example, Bob's code might look something like:

```go
func StartWorker(ctx context.Context, client client.Client) error {
  worker := temporalnexus.NewWorker(client, temporalnexus.WorkerOptions{})
  // This is sugar over registering an ALO handler that starts a workflow. Of
  // course "Name" could be implied.
  worker.RegisterALOWorkflow(BobWorkflow, worker.ALOWorkflowOptions{Name: "bob-alo-call"})
  return worker.Start()
}

func BobWorkflow(ctx workflow.Context, someParam string) (string, error) {
  return strings.ToUpper(someParam), nil
}
```

With this flexibility the following can be customized:

* Instead of just calling `RegisterALOWorkflow` which is just a helper for a workflow-starting handler, Bob could
  `RegisterALOHandler` which gives him a Temporal client he can do whatever he wants with and wait as long as he wants.
  (heartbeating is done in background and a friendly call is exposed for telling Alice that the ALO has "started").
* Instead of registering an ALO handler, Bob could just `RegisterNexusHandler` directly. Maybe Bob doesn't want to start
  anything from Temporal at all? Or maybe Bob wants to expose a query to that workflow.

We can also make it easy to expose queries, signals, etc as Nexus calls. We make it easy to expose workflows as ALOs,
have custom ALOs, or just have custom Nexus calls. It's all sugar/helpers all the way down to the Nexus handler.