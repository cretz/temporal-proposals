package nexus

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/nexus-rpc/sdk-go/nexus/backend/backendpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type DialOptions struct {
	Target string
	TLS    *tls.Config

	GRPCDialOptions []grpc.DialOption

	// Will close if interface implements io.Closer. Default is grpc.DialContext.
	GRPCDialer func(ctx context.Context, target string, opts ...grpc.DialOption) (grpc.ClientConnInterface, error)
}

type Client interface {
	Call(context.Context, *Request) (*Response, error)

	StartALO(context.Context, *Request) (*ALORef, *ALOInfo, error)
	GetALOInfo(context.Context, *ALORef) (*ALOInfo, error)
	CancelALO(context.Context, *ALORef) error
	WaitALO(context.Context, *ALORef) (*Response, error)

	// May return nil
	BackendService() backendpb.BackendServiceClient

	Close() error
}

type grpcClient struct {
	// May be nil
	conn           grpc.ClientConnInterface
	backendService backendpb.BackendServiceClient
}

func Dial(ctx context.Context, options DialOptions) (Client, error) {
	// Build gRPC options
	// TODO(cretz): Support HTTP one day?
	var grpcOptions []grpc.DialOption
	if options.TLS != nil {
		grpcOptions = append(grpcOptions, grpc.WithTransportCredentials(credentials.NewTLS(options.TLS)))
	} else {
		grpcOptions = append(grpcOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	grpcOptions = append(grpcOptions, options.GRPCDialOptions...)

	// Dial
	var conn grpc.ClientConnInterface
	var err error
	if options.GRPCDialer == nil {
		conn, err = grpc.DialContext(ctx, options.Target, grpcOptions...)
	} else {
		conn, err = options.GRPCDialer(ctx, options.Target, grpcOptions...)
	}
	if err != nil {
		return nil, err
	}
	return &grpcClient{conn: conn, backendService: backendpb.NewBackendServiceClient(conn)}, nil
}

// TODO(cretz): Document Close is a noop
func NewClientFromGRPC(ctx context.Context, backendService backendpb.BackendServiceClient) Client {
	return &grpcClient{backendService: backendService}
}

func (g *grpcClient) Call(ctx context.Context, req *Request) (*Response, error) {
	pbResp, err := g.backendService.Call(ctx, req.toProto())
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := resp.fromProto(pbResp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (g *grpcClient) StartALO(ctx context.Context, req *Request) (*ALORef, *ALOInfo, error) {
	resp, err := g.Call(ctx, req)
	if err != nil {
		return nil, nil, err
	} else if !resp.ALOInfo {
		return nil, nil, fmt.Errorf("expected response to be ALO, but it is not")
	}
	// TODO(cretz): Is it ok that we don't relay response metadata?
	var info ALOInfo
	if err := info.UnmarshalBinary(resp.Output); err != nil {
		return nil, nil, err
	}
	return &ALORef{Service: req.Service, Operation: req.Operation, ID: info.ID}, &info, nil
}

func (g *grpcClient) GetALOInfo(ctx context.Context, ref *ALORef) (*ALOInfo, error) {
	refBytes, err := proto.Marshal(&backendpb.AloRef{Id: ref.ID})
	if err != nil {
		return nil, err
	}
	resp, err := g.Call(ctx, &Request{
		RequestID: uuid.NewString(),
		Service:   ref.Service,
		Operation: ref.Operation + "/get",
		Input:     refBytes,
	})
	if err != nil {
		return nil, err
	}
	var info ALOInfo
	if err := info.UnmarshalBinary(resp.Output); err != nil {
		return nil, err
	}
	return &info, nil
}

func (g *grpcClient) CancelALO(ctx context.Context, ref *ALORef) error {
	refBytes, err := proto.Marshal(&backendpb.AloRef{Id: ref.ID})
	if err != nil {
		return err
	}
	_, err = g.Call(ctx, &Request{
		RequestID: uuid.NewString(),
		Service:   ref.Service,
		Operation: ref.Operation + "/cancel",
		Input:     refBytes,
	})
	return err
}

func (g *grpcClient) WaitALO(ctx context.Context, ref *ALORef) (*Response, error) {
	refBytes, err := proto.Marshal(&backendpb.AloRef{Id: ref.ID})
	if err != nil {
		return nil, err
	}
	return g.Call(ctx, &Request{
		RequestID: uuid.NewString(),
		Service:   ref.Service,
		Operation: ref.Operation + "/wait",
		Input:     refBytes,
	})
}

func (g *grpcClient) BackendService() backendpb.BackendServiceClient {
	return g.backendService
}

func (g *grpcClient) Close() error {
	if closer, _ := g.conn.(io.Closer); closer != nil {
		return closer.Close()
	}
	return nil
}
