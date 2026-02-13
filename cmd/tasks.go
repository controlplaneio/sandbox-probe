package cmd

import (
	"fmt"

	"github.com/controlplaneio/sandbox-probe/pkg/tasks"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var taskCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Task related command",
	Long:  "Perform functions like listing the available tasks and tasksets",
}

var listTaskCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks command",
	Long:  "Report the available tasks and tasksets",
	RunE: func(cmd *cobra.Command, args []string) error {
		return list()
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(listTaskCmd)

	viper.BindPFlags(taskCmd.Flags())
}

func list() error {
	tasksNames := tasks.GetAllTasksNames()
	tsks, err := tasks.GetTasksByName(tasksNames)
	if err != nil {
		return err
	}

	if len(tsks) != len(tasksNames) {
		fmt.Errorf("Mismatch between task names and tasks")
	}

	// Find max task name length for alignment
	maxLen := 0
	for _, name := range tasksNames {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	for idx, t := range tsks {
		name := tasksNames[idx]
		// Pad task name to align descriptions
		fmt.Printf("\033[1;34m%-*s\033[0m : \033[0;37m%s\033[0m\n", maxLen, name, t.GetDescription())
	}

	return nil
}
