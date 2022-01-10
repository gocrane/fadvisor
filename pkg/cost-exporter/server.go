package cost_exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	"github.com/gocrane/fadvisor/pkg/util"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/gocrane/fadvisor/pkg/cost-exporter/costmodel"
)

func NewServer(model costmodel.CostModel, bind string, debugging componentbaseconfig.DebuggingConfiguration) *Server {
	return &Server{
		model:     model,
		bind:      bind,
		debugging: debugging,
		server:    &http.Server{},
	}
}

type Server struct {
	model     costmodel.CostModel
	bind      string
	server    *http.Server
	debugging componentbaseconfig.DebuggingConfiguration
}

func (s *Server) RegisterHandlers() {
	baseHandler := util.NewBaseHandler("apiserver", s.debugging)
	baseHandler.Handle("/nodes/cost", s.NodesCostHandler())
	baseHandler.Handle("/nodes/pricing", s.NodesPriceHandler())

	handler := util.BuildHandlerChain(baseHandler, nil, nil)
	s.server.Handler = handler
}

func (s *Server) Serve(stopCh <-chan struct{}) <-chan struct{} {
	// Shutdown server gracefully.
	stoppedCh := make(chan struct{})

	go func() {
		defer utilruntime.HandleCrash()
		defer close(stoppedCh)
		<-stopCh
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = s.server.Shutdown(ctx)
		cancel()
	}()

	go func() {
		defer utilruntime.HandleCrash()

		l, err := net.Listen("tcp", s.bind)
		if err != nil {
			klog.Errorf("Failed to start server: %v", err)
			os.Exit(-1)
		}
		// block until shutdown or err
		err = s.server.Serve(l)
		msg := fmt.Sprintf("Stopped listening on %s", l.Addr().String())
		select {
		case <-stopCh:
			klog.Info(msg)
		default:
			panic(fmt.Sprintf("%s due to error: %v", msg, err))
		}
	}()
	return stoppedCh
}

func (s *Server) NodesCostHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		costs, err := s.model.GetNodesCost()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		data, err := json.Marshal(costs)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
		} else {
			_, _ = w.Write(data)
		}
	})
}

func (s *Server) NodesPriceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pricing, err := s.model.GetNodesPricing()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		data, err := json.Marshal(pricing)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
		} else {
			_, _ = w.Write(data)
		}
	})
}
