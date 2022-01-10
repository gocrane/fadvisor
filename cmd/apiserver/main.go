package main

import (
	"fmt"
	"os"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/gocrane/fadvisor/cmd/cost-exporter/app"
)

// apiserver main.
func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	ctx := genericapiserver.SetupSignalContext()

	if err := app.NewExporterCommand(ctx).Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
