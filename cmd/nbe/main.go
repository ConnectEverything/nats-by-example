package main

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

func main() {
	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	app = cli.App{
		Name:  "nbe",
		Usage: "CLI for using the NATS by Example repo.",
		Commands: []*cli.Command{
			&runCmd,
			&buildCmd,
			&imageCmd,
			&serveCmd,
			&generateCmd,
			&ejectCmd,
		},
	}

	imageCmd = cli.Command{
		Name:  "image",
		Usage: "Build the container image for the example.",
		Action: func(c *cli.Context) error {
			example := c.Args().First()

			repo, err := os.Getwd()
			if err != nil {
				return err
			}

			b := ImageBuilder{
				Repo:    repo,
				Example: example,
				Verbose: true,
			}

			image, err := b.Run()
			if err != nil {
				return err
			}

			fmt.Println(image)
			return nil
		},
	}

	ejectCmd = cli.Command{
		Name:  "eject",
		Usage: "Eject the example source files to a new directory.",
		Action: func(c *cli.Context) error {
			example := c.Args().Get(0)
			dir := c.Args().Get(1)

			if example == "" {
				return errors.New("example name is required")
			}

			if dir == "" {
				return errors.New("output directory must be specified")
			}

			repo, err := os.Getwd()
			if err != nil {
				return err
			}

			b := Ejecter{
				Repo:    repo,
				Example: example,
				Dir:     dir,
				Verbose: true,
			}

			return b.Run()
		},
	}

	runCmd = cli.Command{
		Name:  "run",
		Usage: "Run an example using containers.",
		Description: `To run an example, the current requirement is to clone the
repo and run the command in the root the repo.

Future versions may leverage pre-built images hosted in a registry to reduce
the runtime.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "cluster",
				Usage: "Use compose file with a NATS cluster.",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "Explicit name of the run. This maps to the Compose project name and image tag.",
				Value: "",
			},
			&cli.StringFlag{
				Name:  "image",
				Usage: "Pre-built image for this example to use.",
				Value: "",
			},
			&cli.BoolFlag{
				Name:  "keep",
				Usage: "If true, the example image is not deleted after the run.",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "up",
				Usage: "Run with docker compose up primarily for debugging.",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "Hide output of image building.",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "no-ansi",
				Usage: "If true, disable ANSI control characters.",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			cluster := c.Bool("cluster")
			name := c.String("name")
			keep := c.Bool("keep")
			image := c.String("image")
			up := c.Bool("up")
			quiet := c.Bool("quiet")
			noAnsi := c.Bool("no-ansi")

			example := c.Args().First()

			repo, err := os.Getwd()
			if err != nil {
				return err
			}

			if image == "" {
				b := ImageBuilder{
					Name:    name,
					Repo:    repo,
					Example: example,
					Verbose: !quiet,
				}

				image, err = b.Run()
				if err != nil {
					return err
				}

				if !keep {
					// Best effort.
					defer removeImage(image)
				}
			}

			r := ComposeRunner{
				Name:    name,
				Repo:    repo,
				Example: example,
				Cluster: cluster,
				Keep:    keep,
				Up:      up,
				Verbose: !quiet,
				NoAnsi:  noAnsi,
			}

			return r.Run(image)
		},
	}

	serveCmd = cli.Command{
		Name:  "serve",
		Usage: "Dev server for the docs.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "dir",
				Usage: "Directory containing the rendered HTML.",
				Value: "html",
			},
			&cli.StringFlag{
				Name:  "addr",
				Usage: "HTTP bind address.",
				Value: "localhost:8000",
			},
		},
		Action: func(c *cli.Context) error {
			addr := c.String("addr")
			dir := c.String("dir")
			return http.ListenAndServe(addr, http.FileServer(http.Dir(dir)))
		},
	}

	generateCmd = cli.Command{
		Name:  "generate",
		Usage: "Set of commands for generating various files from examples.",
		Subcommands: []*cli.Command{
			&generateRecordingCmd,
		},
	}

	generateRecordingCmd = cli.Command{
		Name:  "recording",
		Usage: "Generate execution recording for examples.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Source directory containing the examples.",
				Value: "examples",
			},
			&cli.BoolFlag{
				Name:  "recreate",
				Usage: "If true, recreate all previously generated files.",
			},
			&cli.BoolFlag{
				Name:  "exit-on-error",
				Usage: "If true, exits on the first error that occurs.",
			},
		},
		Action: func(c *cli.Context) error {
			repo, err := os.Getwd()
			if err != nil {
				return err
			}

			source := c.String("source")
			recreate := c.Bool("recreate")
			exitOnError := c.Bool("exit-on-error")

			path := c.Args().First()

			var matches map[string]struct{}
			var useMatch bool
			if path != "" {
				ms, err := filepath.Glob(path)
				if err != nil {
					return err
				}
				if len(ms) == 0 {
					return fmt.Errorf("glob has no matches")
				}
				matches = make(map[string]struct{})
				for _, m := range ms {
					matches[m] = struct{}{}
				}
				useMatch = true
			}

			root, err := parseExamples(source)
			if err != nil {
				return err
			}

			// Enumerate all the client implementations.
			for _, c := range root.Categories {
				for _, e := range c.Examples {
					for _, i := range e.Clients {
						if _, ok := matches[i.Path]; ok || !useMatch {
							log.Printf("%s: recording", i.Path)
							if err := generateRecording(repo, i.Path, recreate); err != nil {
								if exitOnError {
									return fmt.Errorf("%s: %s", i.Path, err)
								} else {
									log.Printf("%s: %s", i.Path, err)
								}
							}
						}
					}
				}
			}

			return nil
		},
	}

	buildCmd = cli.Command{
		Name:  "build",
		Usage: "Takes the examples and builds documentation from it.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Source directory containing the examples.",
				Value: "examples",
			},
			&cli.StringFlag{
				Name:  "static",
				Usage: "Directory containing static files that will be copied in.",
				Value: "static",
			},
			&cli.StringFlag{
				Name:  "output",
				Usage: "Directory the HTML files will be written to. Note, this will delete the existing directory if present.",
				Value: "html",
			},
		},
		Action: func(c *cli.Context) error {
			source := c.String("source")
			output := c.String("output")
			static := c.String("static")

			root, err := parseExamples(source)
			if err != nil {
				return err
			}

			if _, err := os.Stat(output); os.IsNotExist(err) {
				if err := os.MkdirAll(output, 0755); err != nil {
					return err
				}
			} else {
				entries, err := os.ReadDir(output)
				if err != nil {
					return err
				}
				for _, e := range entries {
					if err := os.RemoveAll(filepath.Join(output, e.Name())); err != nil {
						return err
					}
				}
			}

			entries, err := fs.ReadDir(os.DirFS(static), ".")
			if err != nil {
				return err
			}

			for _, e := range entries {
				b, err := ioutil.ReadFile(filepath.Join(static, e.Name()))
				if err != nil {
					return err
				}
				err = createFile(filepath.Join(output, e.Name()), b)
				if err != nil {
					return err
				}
			}

			return generateDocs(root, output)
		},
	}
)
