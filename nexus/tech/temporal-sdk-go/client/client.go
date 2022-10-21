package client

import (
	"context"

	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/client"
)

type Client interface {
	client.Client

	Nexus() nexus.Client
}

type NexusResponse[T any] struct {
	*nexus.Response
	Output T
}

func CallNexus[I any, O any](
	ctx context.Context,
	client client.Client,
	req *nexus.Request,
	param I,
) (*NexusResponse[O], error) {
	panic("TODO")
}

func StartNexusALO[I any, O any](
	ctx context.Context,
	client client.Client,
	req *nexus.Request,
	param I,
) (NexusALOHandle[O], error) {
	panic("TODO")
}

func GetNexusALO[O any](
	ctx context.Context,
	client client.Client,
	ref *nexus.ALORef,
) (NexusALOHandle[O], error) {
	panic("TODO")
}

type NexusALOHandle[O any] interface {
	GetInfo(context.Context) (*nexus.ALOInfo, error)
	Cancel(context.Context) error
	Wait(context.Context) (*NexusResponse[O], error)
}
