package cmd

import (
	"fmt"

	tasks "github.com/controlplaneio/sandbox-probe/pkg/tasks/baseline"
	"github.com/spf13/cobra"
)

// seedCmd plants the IPC decoys the shell seeder cannot: bash has no way to
// bind() a Unix socket, and the probe is the one artifact already present on
// every runner. It reads the same registry it scans, so a decoy can never land
// somewhere the probe does not look.
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Soft-plant socket and process decoys at the probe's own IPC targets",
	Long: `Bind a decoy Unix socket at every seedable socket target in the registry, and
start a decoy process for every process target, so a sandbox blocking IPC or
process visibility becomes a provable result rather than "nothing was there".

Soft: a target something already owns is left untouched and counted as skipped,
so a real docker.sock is never shadowed, and no running process is ever adopted.
What was planted is recorded, and "cleanup" removes exactly that. A decoy
process also exits on its own after a fixed timeout, so a cleanup that never
runs is not a permanent leak.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := tasks.SeedTargets(tasks.ListTargets(), tasks.DefaultSeedRecordPath())
		fmt.Printf("seed: planted %d, skipped %d (already present / unwritable)\n", res.Planted, res.Skipped)
		return err
	},
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove the decoys a previous seed recorded",
	Long: `Remove every artifact the recorded seeding pass created, and nothing else.
Safe to run twice, and safe after a crashed run: an artifact already gone, one
that is no longer the socket that was planted, or a recorded pid that no longer
holds the command name it was seeded under, is left alone — unsignalled.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		removed, err := tasks.CleanupSeeded(tasks.DefaultSeedRecordPath())
		fmt.Printf("cleanup: removed %d seeded artifact(s)\n", removed)
		return err
	},
}

func init() {
	rootCmd.AddCommand(seedCmd, cleanupCmd)
}
