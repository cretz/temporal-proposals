// Package workflow would technically be embedded into Temporal's SDK in the
// workflow package.
package workflow

import "go.temporal.io/sdk/workflow"

type NexusCallOptions struct {
	// Required service name. This is a simple name that is mapped to the real
	// service name on the Temporal server.
	Service string

	// Required operation name.
	Operation string

	// Identifier for the Nexus operation. If not provided, this is defaulted to a
	// unique value.
	ID string

	// Optional metadata for the call.
	Metadata map[string][]string
}

// NexusFuture represents a future that will be completed when the Nexus call or
// ALO is completed. To just wait for start in the case of ALO, "Accepted" can
// be used.
type NexusFuture interface {
	workflow.Future

	// Accepted is a future that is resolved when an ALO is started. If the Nexus
	// call is not an ALO, this is resolved at the same time as this NexusFuture.
	// This future always has a nil result type.
	//
	// NOTES:
	// * What result type should we use instead of nil?
	Accepted() workflow.Future
}

// CallNexus schedules a Nexus call.
func CallNexus(ctx workflow.Context, options NexusCallOptions, input interface{}) NexusFuture {
	panic("TODO")
}
