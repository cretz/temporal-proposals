package nexus

import (
	"context"
)

type WorkerOptions struct {
}

type Worker struct {
}

func NewWorker(client *Client, options WorkerOptions) (*Worker, error) {
	panic("TODO")
}

func Run(ctx context.Context) error {
	panic("TODO")
}

type RegisterHandlerOptions struct {
	Service   string
	Operation string
	Handler   Handler
}

func (w *Worker) RegisterHandler(options RegisterHandlerOptions) error {
	panic("TODO")
}

type RegisterALOHandlerOptions struct {
	Service   string
	Operation string
	Handler   ALOHandler
}

func (w *Worker) RegisterALOHandler(options RegisterALOHandlerOptions) error {
	return (&ALOHandlers{options}).Register(w)
}

type ALOHandlers struct {
	Options RegisterALOHandlerOptions
}

func (a *ALOHandlers) Register(w *Worker) error {
	err := w.RegisterHandler(RegisterHandlerOptions{
		Service:   a.Options.Service,
		Operation: a.Options.Operation,
		Handler:   HandlerFunc(a.StartALO),
	})
	if err != nil {
		return err
	}
	err = w.RegisterHandler(RegisterHandlerOptions{
		Service:   a.Options.Service,
		Operation: a.Options.Operation + "/get",
		Handler:   HandlerFunc(a.GetALOInfo),
	})
	if err != nil {
		return err
	}
	err = w.RegisterHandler(RegisterHandlerOptions{
		Service:   a.Options.Service,
		Operation: a.Options.Operation + "/cancel",
		Handler:   HandlerFunc(a.CancelALO),
	})
	if err != nil {
		return err
	}
	return w.RegisterHandler(RegisterHandlerOptions{
		Service:   a.Options.Service,
		Operation: a.Options.Operation + "/wait",
		Handler:   HandlerFunc(a.WaitALO),
	})
}

func (a *ALOHandlers) StartALO(ctx context.Context, req *Request) (*Response, error) {
	info, err := a.Options.Handler.StartALO(ctx, req)
	if err != nil {
		return nil, err
	}
	b, err := info.MarshalBinary()
	if err != nil {
		return nil, err
	}
	// TODO(cretz): Is it ok that there is no response metadata that can be set?
	return &Response{Output: b, ALOInfo: true}, nil
}

func (a *ALOHandlers) GetALOInfo(ctx context.Context, req *Request) (*Response, error) {
	var ref ALORef
	if err := ref.UnmarshalFromRequest(req); err != nil {
		return nil, err
	}
	info, err := a.Options.Handler.GetALOInfo(ctx, &ref)
	if err != nil {
		return nil, err
	}
	b, err := info.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &Response{Output: b, ALOInfo: true}, nil
}

func (a *ALOHandlers) CancelALO(ctx context.Context, req *Request) (*Response, error) {
	var ref ALORef
	if err := ref.UnmarshalFromRequest(req); err != nil {
		return nil, err
	}
	if err := a.Options.Handler.CancelALO(ctx, &ref); err != nil {
		return nil, err
	}
	return &Response{}, nil
}

func (a *ALOHandlers) WaitALO(ctx context.Context, req *Request) (*Response, error) {
	var ref ALORef
	if err := ref.UnmarshalFromRequest(req); err != nil {
		return nil, err
	}
	return a.Options.Handler.WaitALO(ctx, &ref)
}
