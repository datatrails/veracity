package veracity

import (
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

// cfgLogging establishes the logger
// call this once at the start of the command
func cfgLogging(cmd *CmdCtx, cCtx *cli.Context) error {
	logLevel := cCtx.String("loglevel")
	if logLevel == "" {
		logLevel = "INFO"
	}
	// This special case allows our integration tests to disable logging
	// configuration all together. This is necessary due to the approach of
	// instancing the entire veracity command, including its logging
	// initialisation, in the integration tests. In cases where the test needs
	// to be threaded, this can cause a data race due to the way our logging
	// package deals with its global process state.
	if logLevel == "NOOP" {
		cmd.log = &logger.WrappedLogger{
			SugaredLogger: zap.NewNop().Sugar(),
		}
	} else {
		logger.New(logLevel, logger.WithConsole())
		cmd.log = logger.Sugar
	}

	return nil
}
