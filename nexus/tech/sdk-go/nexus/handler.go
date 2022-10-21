package nexus

import (
	"context"
	"fmt"

	"github.com/nexus-rpc/sdk-go/nexus/backend/backendpb"
	"google.golang.org/protobuf/proto"
)

type Request struct {
	RequestID    string
	Service      string
	Operation    string
	Input        []byte
	Metadata     map[string][]string
	HTTPCallback *HTTPCallback
}

type HTTPCallback struct {
	URL string
}

type Response struct {
	Output   []byte
	ALOInfo  bool
	Metadata map[string][]string
}

type ResponseFailure struct {
	Code     uint32
	Output   []byte
	Metadata map[string][]string
}

func (r *ResponseFailure) Error() string {
	return fmt.Sprintf("Nexus request failure with code %v", r.Code)
}

type Handler interface {
	ServeNexus(context.Context, *Request) (*Response, error)
}

type HandlerFunc func(context.Context, *Request) (*Response, error)

func (h HandlerFunc) ServeNexus(ctx context.Context, req *Request) (*Response, error) {
	return h(ctx, req)
}

type ALORef struct {
	Service   string
	Operation string
	ID        string
}

func (a *ALORef) UnmarshalFromRequest(req *Request) error {
	var pb backendpb.AloRef
	if err := proto.Unmarshal(req.Input, &pb); err != nil {
		return err
	}
	a.Service = req.Service
	a.Operation = req.Operation
	a.ID = pb.Id
	return nil
}

type ALOInfo struct {
	ID       string
	Status   ALOStatus
	Metadata map[string]string
}

func (a *ALOInfo) MarshalBinary() ([]byte, error) { return proto.Marshal(a.toProto()) }
func (a *ALOInfo) UnmarshalBinary(data []byte) error {
	var pb backendpb.AloInfo
	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}
	a.fromProto(&pb)
	return nil
}

type ALOStatus int

const (
	ALOStatusUnspecified ALOStatus = iota
	ALOStatusRunning
	ALOStatusCompleted
)

type ALOHandler interface {
	// TODO(cretz): What if ALO completed so fast and they want to return eagerly? Do we support that?
	StartALO(context.Context, *Request) (*ALOInfo, error)
	GetALOInfo(context.Context, *ALORef) (*ALOInfo, error)
	CancelALO(context.Context, *ALORef) error
	WaitALO(context.Context, *ALORef) (*Response, error)
}

// Proto helpers

func (r *Request) toProto() *backendpb.CallRequest {
	pb := &backendpb.CallRequest{
		RequestId: r.RequestID,
		Service:   r.Service,
		Operation: r.Operation,
		Input:     r.Input,
		Metadata:  callMetadataToProto(r.Metadata),
	}
	if r.HTTPCallback != nil {
		pb.AloCompletionCallback = &backendpb.CallRequest_CompletionCallback{
			Callback: &backendpb.CallRequest_CompletionCallback_Http_{Http: &backendpb.CallRequest_CompletionCallback_Http{
				Url: r.HTTPCallback.URL,
			}},
		}
	}
	return pb
}

func (r *Response) fromProto(resp *backendpb.CallResponse) *ResponseFailure {
	if f := resp.GetFailure(); f != nil {
		return &ResponseFailure{
			Code:     f.Code,
			Output:   f.Output,
			Metadata: callMetadataFromProto(resp.Metadata),
		}
	}
	// TODO(cretz): Be more defensive about what is and isn't present
	r.Output = resp.GetSuccess().Output
	r.Metadata = callMetadataFromProto(resp.Metadata)
	r.ALOInfo = resp.GetSuccess().ResponseType == backendpb.CallResponse_Success_ALO_INFO
	return nil
}

func (a *ALOInfo) toProto() *backendpb.AloInfo {
	return &backendpb.AloInfo{Id: a.ID, Status: backendpb.AloInfo_Status(int32(a.Status)), Metadata: a.Metadata}
}

func (a *ALOInfo) fromProto(info *backendpb.AloInfo) {
	a.ID = info.Id
	a.Status = ALOStatus(info.Status)
	a.Metadata = info.Metadata
}

func callMetadataToProto(m map[string][]string) map[string]*backendpb.MetadataValues {
	if len(m) == 0 {
		return nil
	}
	ret := make(map[string]*backendpb.MetadataValues, len(m))
	for k, v := range m {
		ret[k] = &backendpb.MetadataValues{Values: v}
	}
	return ret
}

func callMetadataFromProto(m map[string]*backendpb.MetadataValues) map[string][]string {
	if len(m) == 0 {
		return nil
	}
	ret := make(map[string][]string, len(m))
	for k, v := range m {
		ret[k] = v.Values
	}
	return ret
}
