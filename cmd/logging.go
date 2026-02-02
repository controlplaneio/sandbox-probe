package cmd

import (
	"os"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func setupLogging() {
	// Set global logger
	zlog.Logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Caller().
		Logger()

	// Set global log level
	zerolog.SetGlobalLevel(stringToLogLevel(viper.GetString("log_level")))
}

func stringToLogLevel(level string) zerolog.Level {
	switch level {
	case "info":
		return zerolog.InfoLevel
	default:
		return zerolog.InfoLevel
	}
}
