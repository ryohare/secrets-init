package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"secrets-init/pkg/secrets"
	"secrets-init/pkg/secrets/aws"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/ini.v1"
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
				Usage: "supported secrets manager provider ['aws']",
				Value: "aws",
			},
			&cli.StringFlag{
				Name:     "ini, i",
				Usage:    "ini file to read and annotate with secrets",
				Required: false,
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

	// get provider
	var provider secrets.Provider
	var err error
	if c.String("provider") == "aws" {
		provider, err = aws.NewAwsSecretsProvider()
	}
	if err != nil {
		log.WithField("provider", c.String("provider")).WithError(err).Error("failed to initialize secrets provider")
	}
	// Launch main command
	err = run(nil, provider, c.Args().Slice())
	if err != nil {
		log.WithError(err).Error("failed to run")
	}

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

	iniMode := false
	if strings.Contains(pathStr, ".ini") {
		iniMode = true
	}

	var err error
	var env []string
	var cfg *ini.File
	sectionToKeyMap := make(map[string]string)
	if iniMode {
		cfg, err = ini.Load(pathStr)

		if err != nil {
			log.WithError(err).Error("Could not read specified ini file")
			os.Exit(5)
		}

		for _, s := range cfg.SectionStrings() {
			sec, err := cfg.GetSection(s)
			if err != nil {
				log.WithError(err).Error("Could not get section")
				continue
			}

			for _, k := range sec.KeyStrings() {
				key, _ := sec.GetKey(k)
				//fmt.Printf("%s=%s\n", key.Name(), key.String())
				env = append(env, fmt.Sprintf("%s=%s", key.Name(), key.String()))
				sectionToKeyMap[key.Name()] = sec.Name()
			}
		}
	} else {
		env = os.Environ()
	}

	// set environment variables
	var secs []secrets.Secret
	if provider != nil {
		secs, err = provider.ResolveSecrets(nil, env)
		if err != nil {
			log.WithError(err).Error("failed to resolve secrets")
		}
	} else {
		log.Warn("no secrets provider available; using environment without resolving secrets")
	}

	if iniMode {
		for _, s := range secs {
			if s.Format == secrets.KeyValueFormat {
				log.Errorf("Secret %s is not plaintext format. Cannot load", s.Arn)
				continue
			}
			section, err := cfg.GetSection(sectionToKeyMap[s.ArnVarName])
			if err != nil {
				log.WithError(err).Error("Failed to get section to write out secret")
				continue
			}
			key, err := section.GetKey(s.ArnVarName)
			if err != nil {
				log.WithError(err).Error("Failed to get key to write out secret")
				continue
			}
			key.SetValue(s.KeyValues[0].Value)
		}
		cfg.SaveTo(pathStr)
	} else {

		// env file writing mode
		f, err := os.OpenFile(pathStr, os.O_CREATE|os.O_WRONLY, 0644)

		if err != nil {
			log.Errorf("Could not open output file because %s", err.Error())
			os.Exit(7)
		}
		defer f.Close()

		fileTemplate := `#!/bin/sh`

		for _, e := range secs {
			for _, s := range e.GetKeyValueStrings() {
				fileTemplate = fmt.Sprintf("%s\nexport %s;", fileTemplate, s)
			}
		}

		f.WriteString(fileTemplate)

	}
	return nil
}

func cleanQuit(cancel context.CancelFunc, wg *sync.WaitGroup, code int) {
	// signal zombie goroutine to stop and wait for it to release waitgroup
	cancel()
	wg.Wait()

	os.Exit(code)
}
