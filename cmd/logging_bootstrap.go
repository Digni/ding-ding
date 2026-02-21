package cmd

import (
	"fmt"
	"io"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/logging"
)

var commandLoggingBootstrap = func(cfg config.LoggingConfig, role logging.Role) error {
	logging.Bootstrap(cfg, role)
	return nil
}

func initializeCommandLogging(errWriter io.Writer, cfg config.LoggingConfig, role logging.Role) {
	if err := commandLoggingBootstrap(cfg, role); err != nil {
		fmt.Fprintf(errWriter, "warning: unable to initialize persistent logging for %s role: %v; continuing without file logging\n", role, err)
	}
}
