package cmd

import (
	"fmt"
	"time"

	tasks "github.com/controlplaneio/sandbox-probe/v6/pkg/tasks/baseline"
	"github.com/spf13/cobra"
)

// seedCmd plants the IPC decoys the shell seeder cannot: bash has no way to
// bind() a Unix socket, and the probe is the one artifact already present on
// every runner. It reads the same registry it scans, so a decoy can never land
// somewhere the probe does not look.
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Soft-plant socket, named-pipe and process decoys at the probe's own IPC targets",
	Long: `Bind a decoy Unix socket at every seedable socket target in the registry, serve
a decoy named pipe for every Windows pipe target, and start a decoy process for
every process target, so a sandbox blocking IPC or process visibility becomes a
provable result rather than "nothing was there".

Soft: a target something already owns is left untouched and counted as skipped,
so a real docker.sock or docker_engine pipe is never shadowed, and no running
process is ever adopted.
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
that is no longer the socket that was planted, a recorded pid that no longer
holds the command name it was seeded under, or a pipe server that is no longer
the process seeding spawned, is left alone — unsignalled.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		removed, err := tasks.CleanupSeeded(tasks.DefaultSeedRecordPath())
		fmt.Printf("cleanup: removed %d seeded artifact(s)\n", removed)
		return err
	},
}

// servePipeCmd is how a Windows pipe decoy stays alive: a pipe exists only
// while a server holds it open, so "seed" spawns the probe again in this mode,
// one process per decoy, each exiting on its own after the given lifetime.
// Hidden because it is an implementation detail of "seed", not something to run
// by hand.
var servePipeCmd = &cobra.Command{
	Use:    "serve-pipe <name> <lifetime>",
	Short:  "Serve one decoy named pipe until the lifetime expires (used by seed)",
	Hidden: true,
	Args:   cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return err
		}
		return tasks.ServePipe(args[0], d)
	},
}

func init() {
	rootCmd.AddCommand(seedCmd, cleanupCmd, servePipeCmd)
}
