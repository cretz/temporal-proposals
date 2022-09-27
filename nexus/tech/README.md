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