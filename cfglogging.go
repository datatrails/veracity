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
	if logLevel == "TEST" {
		cmd.log = &logger.WrappedLogger{
			SugaredLogger: zap.NewNop().Sugar(),
		}
	} else {
		logger.New(logLevel, logger.WithConsole())
		cmd.log = logger.Sugar
	}

	return nil
}
