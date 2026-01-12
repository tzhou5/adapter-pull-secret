package main

import (
	"context"
	"flag"

	logger "github.com/openshift-online/ocm-service-common/pkg/ocmlogger"
	"github.com/spf13/cobra"
	"gitlab.cee.redhat.com/service/hyperfleet/mvp/cmd/pull-secret/jobs"

	// This package is used to make the runtime maxprocs cGroup aware rather than
	// using the number of available machine cores. This is necessary for containerized
	// applications so the Go scheduler doesnt overcommit compute and cause the container
	// to be throttled: https://github.com/golang/go/issues/33803
	//
	// This will be a noop in non-containerized environments and also obeys the GOMAXPROCS
	// env override.
	_ "go.uber.org/automaxprocs"
)

func init() {
	logger.SetTrimList([]string{"pull-secret", "pkg"})
	_ = logger.SetLogLevel(logger.OCM_LOG_LEVEL_DEFAULT) //nolint:errcheck // safe to ignore
}

func main() {
	ctx := context.Background()
	ulog := logger.NewOCMLogger(ctx)

	// This is needed to make `glog` believe that the flags have already been parsed, otherwise
	// every log messages is prefixed by an error message stating the the flags haven't been
	// parsed.
	_ = flag.CommandLine.Parse([]string{}) //nolint:errcheck // Parse flags, error can be safely ignored

	// Always log to stderr by default
	if err := flag.Set("logtostderr", "true"); err != nil {
		ulog.Contextual().Info("Unable to set logtostderr to true")
	}

	rootCmd := &cobra.Command{
		Use:  "pull-secret",
		Long: "pull-secret job runner",
	}

	// Add job command
	jobCmd := jobs.NewJobCommand(ctx)
	rootCmd.AddCommand(jobCmd)

	if err := rootCmd.Execute(); err != nil {
		ulog.Contextual().Fatal(err, "error running command")
	}
}
