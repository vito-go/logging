package unilogrpc

import (
	"errors"
	"net/rpc"
)

type Unilog struct {
	register    func(*UnilogRegisterReq) (*int64, error)
	serviceName string
}

type UnilogRegisterReq struct {
	APPName string
	Host    string // ip:port
	CodeInt int64
}

type UnilogCli struct {
	rpcCli      *rpc.Client
	serviceName string
}
type UnilogServer interface {
	Register(*UnilogRegisterReq) (*int64, error)
}

func RegisterUnilogServer(server *rpc.Server, s UnilogServer) error {
	receiver := &Unilog{
		register:    s.Register,
		serviceName: "Unilog",
	}
	return server.RegisterName(receiver.serviceName, receiver)
}
func NewUnilogCli(rpcCli *rpc.Client) *UnilogCli {
	return &UnilogCli{rpcCli: rpcCli, serviceName: "Unilog"}
}
func (h *Unilog) Register(in *UnilogRegisterReq, resp *int64) error {
	if h.register == nil {
		return errors.New("nil func")
	}
	out, err := h.register(in)
	if err != nil {
		return err
	}
	*resp = *out
	return nil
}

func (r *UnilogCli) Register(arg *UnilogRegisterReq) (*int64, error) {
	if arg == nil {
		return nil, errors.New("nil arg")
	}
	var resp = new(int64)
	err := r.rpcCli.Call(r.serviceName+".Register", arg, resp)
	return resp, err
}
