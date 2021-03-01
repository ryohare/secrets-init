package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"secrets-init/pkg/secrets"
	"secrets-init/pkg/secrets/aws"
	"secrets-init/pkg/secrets/google"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	// Version contains the current version.
	version = "dev"
	// BuildDate contains a string with the build date.
	date = "unknown"
	// commit
	commit = "dirty"
	// buildBy
	builtBy = "local"
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "provider, p",
				Usage: "supported secrets manager provider ['aws', 'google']",
				Value: "aws",
			},
		},
		Name:    "secrets-init",
		Usage:   "enrich environment variables with secrets from secret manager",
		Action:  mainCmd,
		Version: version,
	}
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("secrets-init %s\n", version)
		fmt.Printf("  build date: %s\n", date)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built by: %s\n", builtBy)
		fmt.Printf("  built with: %s\n", runtime.Version())
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func mainCmd(c *cli.Context) error {
	// Routine to reap zombies (it's the job of init)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	// get provider
	var provider secrets.Provider
	var err error
	if c.String("provider") == "aws" {
		provider, err = aws.NewAwsSecretsProvider()
	} else if c.String("provider") == "google" {
		provider, err = google.NewGoogleSecretsProvider(ctx)
	}
	if err != nil {
		log.WithField("provider", c.String("provider")).WithError(err).Error("failed to initialize secrets provider")
	}
	// Launch main command
	var mainRC int
	err = run(ctx, provider, c.Args().Slice())
	if err != nil {
		log.WithError(err).Error("failed to run")
		mainRC = 1
	}

	// Wait removeZombies goroutine
	cleanQuit(cancel, &wg, mainRC)
	return nil
}

// run passed command
func run(ctx context.Context, provider secrets.Provider, filepathSlice []string) error {
	if len(filepathSlice) == 0 {
		log.Warn("no file path specified")
		return nil
	}

	// split command and arguments
	pathStr := filepathSlice[0]

	var err error
	var env []string
	// set environment variables
	if provider != nil {
		env, err = provider.ResolveSecrets(ctx, os.Environ())
		if err != nil {
			log.WithError(err).Error("failed to resolve secrets")
		}
	} else {
		log.Warn("no secrets provider available; using environment without resolving secrets")
		env = os.Environ()
	}

	// write the envs into a soureable script
	f, err := os.OpenFile(pathStr, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		log.Errorf("Could not open output file because %s", err.Error())
		os.Exit(7)
	}
	defer f.Close()

	fileTemplate := `#!/bin/sh`

	for _, e := range env {
		fileTemplate = fmt.Sprintf("%s\nexport %s;", fileTemplate, e)
	}

	// add the self-destruction
	// fileTemplate = fmt.Sprintf("%s\nrm -- \"$0\"\n", fileTemplate)

	f.WriteString(fileTemplate)

	return nil
}

func cleanQuit(cancel context.CancelFunc, wg *sync.WaitGroup, code int) {
	// signal zombie goroutine to stop and wait for it to release waitgroup
	cancel()
	wg.Wait()

	os.Exit(code)
}
