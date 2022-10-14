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

**From Alice's POV**

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

**From Bob's POV**

Note, this is for Temporal+Nexus Workflow-as-ALO. Generic Nexus handlers may do something different.

1. Bob is running a Temporal+Nexus server and a Temporal+Nexus worker that:
   1. Has a Nexus handler registered to handle Nexus calls. Technically underneath, it's a low-level Nexus handler that
      just starts the workflow defined in the next point for each request, but to Bob it was just a simple one-liner or
      config or something else really high level.
   1. Has an implicit Temporal workflow registered to handle Nexus requests. This is a generic workflow that accepts an
      activity to invoke on Nexus request and handles the callback URL sending.
   1. Has a high-level Nexus handler that, via helpers, knows to just map the ALO Alice will call 1:1 with a Temporal
      workflow. This is basically the activity the generic workflow invokes. To Bob, this is a very simple one-liner or
      config or whatever.
1. Upon receipt of Alice's Nexus call request in Bob's Nexus server, the request is sent to a polling Nexus worker.
1. Upon receive of Alice's Nexus call request in Bob's Nexus worker/handler, the high-level handler is looked up based
   on the request and the low-level handler starts the generic workflow which starts the activity to invoke the
   high-level handler.
1. Bob's high-level Nexus handler (which was just configured, no code):
   1. Uses a Temporal client to get-or-start the workflow Bob wants to run for this Nexus call. Note, this is a
      get-or-start for idempotency.
   1. Sends a signal to the calling internal-request-handler workflow that the ALO was started.
   1. Waits for the Temporal workflow to complete, heartbeating all the while
   1. Returns from the handler (and in turn from the activity) with the completion
1. Bob's low-level request workflow that is running the activity that is running the Nexus handler:
   1. Upon first receipt of a "ALO started" signal, the Nexus request is marked completed with the ALO info.
      1. TBD on how a workflow relays this to the underlying Nexus handler waiting for start. This is the problem sync
         updates will solve, but in the meantime use one of the hacks.
   1. Wait for the activity to complete.
   1. Upon activity completion of the ALO response (or error), invoke Alice's callback as an activity from inside this
      workflow before completing the workflow.

While this may seem convoluted, it's important for flexibility. But it's simplified for the common 1:1 Temporal workflow
use case. For example, Bob's code might look something like:

```go
func StartWorker(ctx context.Context, client client.Client) error {
  worker := temporalnexus.NewWorker(client, temporalnexus.WorkerOptions{})
  // Of course "Name" could be implied
  worker.RegisterALOWorkflow(BobWorkflow, worker.ALOWorkflowOptions{Name: "bob-alo-call"})
  return worker.Start()
}

func BobWorkflow(ctx workflow.Context, someParam string) (string, error) {
  return strings.ToUpper(someParam), nil
}
```