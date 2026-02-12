package cmd

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

func init() {
	rootCmd.Flags().String("log_level", "info", "log level")

	viper.BindPFlags(scanCmd.Flags())
}

var (
	// vars injected by goreleaser at build time
	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "sandbox-probe",
	Short: "sandbox-probe command line",
	Long: `
	Perform security enumeration for AI code assistants`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
		if err := initConfig(); err != nil {
			// log.Warn().Err(err).Msg("Error when initializing the config")
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Msgf("Root command failed with error: %s", err.Error())
		os.Exit(1)
	}
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil
	}

	log.Info().Msgf("Using config:", viper.ConfigFileUsed())
	return nil
}
