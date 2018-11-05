package main

import (
	"os"
	"fmt"
	goflag "flag"

	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/util/logs"

	"github.com/wu8685/lpv-res-predicate/cmd/predicate-server/app"
	_ "github.com/wu8685/lpv-res-predicate/pkg/handlers/health"
	_ "github.com/wu8685/lpv-res-predicate/pkg/handlers/predicate"
)

func main() {
	command := app.NewPredicateServerCommand()

	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
