package cmd

import (
	"encoding/json"
	"fmt"

	tasks "github.com/controlplaneio/sandbox-probe/pkg/tasks/baseline"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var probeCmd = &cobra.Command{
	Hidden: true,
	Use:    "probe",
	Short:  "hidden: A subcommand for any actions we need outside of the main process",
	Long:   "Congratulations, you've found our hidden probe subcommand.\nThis feature exists to conduct actions in a separate process to the main scan.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return probe()
	},
}

func init() {
	rootCmd.AddCommand(probeCmd)

	probeCmd.Flags().StringSlice("tasks", []string{}, "Additional tasks to run.")
	probeCmd.Flags().StringSlice("tasksets", []string{"baseline"}, "Group of tasks to select: baseline, ps, all")
	probeCmd.Flags().String("output_path", "report.json", "path to report the output")
	probeCmd.Flags().StringSlice("tags", []string{""}, "Metadata tags tp be appended to the report")

	viper.BindPFlags(probeCmd.Flags())
}

func probe() error {
	// log.Info().Msg("Starting sandbox-probe probe")

	ld, err := tasks.ProbeLandlockSelfDepth()
	if err != nil {
		return err
	}
	res := tasks.ProbeSubCmdData{
		LockdownDepth: ld,
	}
	out, err := json.Marshal(res)
	if err != nil {
		return err
	}
	// fmt.Fprintf(os.Stderr, "%s", string(out))
	fmt.Printf("%s", string(out))

	return nil
}
