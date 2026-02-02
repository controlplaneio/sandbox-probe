package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
	"github.com/controlplaneio/sandbox-probe/pkg/probes"
	"github.com/controlplaneio/sandbox-probe/pkg/tasks"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupLogging() (*os.File, error) {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	logFileName := filepath.Join(logsDir, fmt.Sprintf("sandbox-probe-%s.log", timestamp))
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Configure zerolog to write to both console and file
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	multiWriter := io.MultiWriter(consoleWriter, logFile)

	// Set global logger
	zlog.Logger = zerolog.New(multiWriter).With().
		Timestamp().
		Caller().
		Logger()

	// Set global log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	zlog.Info().Str("log_file", logFileName).Msg("Logging initialized")

	return logFile, nil
}

func main() {
	// Set up logging
	logFile, err := setupLogging()
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	zlog.Info().Msg("Starting sandbox-probe")

	p, err := probes.NewProbe(
		probes.WithName("baseline"),
		probes.WithTasks(tasks.GetBaselineTasks()),
		// probes.WithTasks([]tasks.Task{
		// 	tasks.NewNetworkTask()}),
	)

	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to create probe")
	}

	if err := p.Run(); err != nil {
		zlog.Fatal().Err(err).Msg("Error running probe")
	}

	zlog.Info().Msg("Probe execution completed successfully")

	// Create the report
	zlog.Info().Msg("Creating report")
	report := &reportv1.Report{
		Version:   "1.0.0",
		Timestamp: timestamppb.New(time.Now()),
		ProbeBinary: &reportv1.ProbeBinary{
			GoVersion: runtime.Version(),
			Os:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			Static:    false, // TODO: detect if binary is statically linked
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
		zlog.Fatal().Err(err).Msg("Error marshaling report to JSON")
	}

	fmt.Println(string(jsonBytes))

	// Write report to file
	if err := os.WriteFile("report.json", jsonBytes, 0644); err != nil {
		zlog.Fatal().Err(err).Msg("Error writing report to file")
	}

	zlog.Info().Str("file", "report.json").Msg("Report written successfully")
	zlog.Info().Msg("Sandbox probe completed")
}
