package cmd

import (
	"encoding/json"
	"fmt"

	tasks "github.com/controlplaneio/sandbox-probe/pkg/tasks/baseline"
	"github.com/spf13/cobra"
)

// listTargetsCmd emits the probe's own sensitive-path registry as JSON so an
// external seeder can plant decoys at exactly the paths the probe checks —
// keeping seeding from drifting out of sync with what is measured.
var listTargetsCmd = &cobra.Command{
	Use:   "list-targets",
	Short: "List the sensitive-path targets the probe checks (JSON)",
	Long: `Emit the probe's target registry as JSON, scoped to the running OS. Each
entry is classified so a seeder knows what to do with it: "kind" says how it is
seeded, and only home-scoped regular files are marked "seedable": true. IPC
entries also carry "category" and "evidence" — why they are on the list.
Consumed by scripts/seed-decoys.sh.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		b, err := json.MarshalIndent(tasks.ListTargets(), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listTargetsCmd)
}
