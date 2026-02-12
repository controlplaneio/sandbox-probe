package cmd

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/controlplaneio/sandbox-probe/pkg/probes"
	"github.com/controlplaneio/sandbox-probe/pkg/tasks"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the environment for security enumerations",
	Long:  "Scan the given environment and reports security enumerations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return scan()
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringSlice("tasks", []string{}, "Additional tasks to run.")
	scanCmd.Flags().StringSlice("tasksets", []string{"baseline"}, "Group of tasks to select: baseline, ps, all")
	scanCmd.Flags().String("output_path", "report.json", "path to report the output")

	viper.BindPFlags(scanCmd.Flags())
}

func loadtasks() ([]tasks.Task, error) {
	var allTasks []tasks.Task
	taskSetsTasks, err := tasks.GetTaskSetsTasks(viper.GetStringSlice("tasksets"))
	if err != nil {
		return nil, err
	}
	individualTasks, err := tasks.GetTasksByName(viper.GetStringSlice("tasks"))
	if err != nil {
		return nil, err
	}
	allTasks = append(allTasks, taskSetsTasks...)
	allTasks = append(allTasks, individualTasks...)
	loadedTasks := delete_duplicate_tasks(allTasks)
	if len(loadedTasks) == 0 {
		return nil, fmt.Errorf("There should be at least one task to run")
	}
	return loadedTasks, nil
}

func delete_duplicate_tasks(inpTasks []tasks.Task) []tasks.Task {
	taskCheck := map[string]struct{}{}
	var deduplicatedTasks []tasks.Task

	for _, task := range inpTasks {
		if _, ok := taskCheck[task.GetName()]; !ok {
			deduplicatedTasks = append(deduplicatedTasks, task)
			taskCheck[task.GetName()] = struct{}{}
		}
	}

	return deduplicatedTasks
}

func scan() error {
	log.Info().Msg("Starting sandbox-probe scan")

	loadedTasks, err := loadtasks()
	if err != nil {
		return err
	}

	p, err := probes.NewProbe(
		probes.WithName("sandbox-probe"),
		probes.WithTasks(loadedTasks),
	)

	if err != nil {
		return err
	}

	if err := p.Run(); err != nil {
		return err
	}

	log.Info().Msg("Probe execution completed successfully")
	log.Info().Msg("Creating the report")

	// Create the report
	log.Info().Msg("Creating report")
	report := &reportv1.Report{
		Version:   "1.0.0",
		Timestamp: timestamppb.New(time.Now()),
		ProbeBinary: &reportv1.ProbeBinary{
			GoVersion:     runtime.Version(),
			Os:            runtime.GOOS,
			Arch:          runtime.GOARCH,
			Static:        false,
			BinaryVersion: version,
			Commit:        commit,
			BuildDate:     date,
		},
		Findings: p.Findings,
	}

	// Convert to JSON using protojson for better formatting
	jsonBytes, err := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: false,
	}.Marshal(report)
	if err != nil {
		log.Fatal().Err(err).Msg("Error marshaling report to JSON")
	}

	fmt.Println(string(jsonBytes))

	// Write report to file
	if err := os.WriteFile(viper.GetString("output_path"), jsonBytes, 0644); err != nil {
		log.Fatal().Err(err).Msg("Error writing report to file")
	}

	log.Info().Str("file", viper.GetString("output_path")).Msg("Report written successfully")
	log.Info().Msg("Sandbox probe completed")

	return nil
}
