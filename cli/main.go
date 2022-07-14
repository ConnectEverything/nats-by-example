package main

import (
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
		Name:  "natsbyexample",
		Usage: "CLI for managing the NATS by Example repo.",
		Commands: []*cli.Command{
			&buildCmd,
			&serveCmd,
			&generateCmd,
		},
	}

	serveCmd = cli.Command{
		Name:  "serve",
		Usage: "Dev server for the docs.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Output directory containing the rendered HTML.",
				Value: "output",
			},
			&cli.StringFlag{
				Name:  "addr",
				Usage: "HTTP bind address.",
				Value: "localhost:8000",
			},
		},
		Action: func(c *cli.Context) error {
			addr := c.String("addr")
			source := c.String("source")
			fmt.Printf("Serving Go by Example at http://%s\n", addr)
			return http.ListenAndServe(addr, http.FileServer(http.Dir(source)))
		},
	}

	generateCmd = cli.Command{
		Name:  "generate",
		Usage: "Set of commands for generating various files from examples.",
		Subcommands: []*cli.Command{
			&generateDockerfileCmd,
			&generateComposefileCmd,
			&generateOutputCmd,
		},
	}

	generateDockerfileCmd = cli.Command{
		Name:  "dockerfile",
		Usage: "[Re]generate Dockerfiles for examples that do not have custom ones.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Source directory containing the examples.",
				Value: "examples",
			},
			&cli.BoolFlag{
				Name:  "recreate",
				Usage: "If true, recreate all previously generated Dockerfiles.",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			source := c.String("source")
			recreate := c.Bool("recreate")

			root, err := parseExamples(source)
			if err != nil {
				return err
			}

			// Enumerate all the example implementations.
			for _, c := range root.Categories {
				for _, e := range c.Examples {
					for _, i := range e.Implementations {
						if err := createDockerfile(i.Language, i.Path, recreate); err != nil {
							log.Printf("%s: %s", i.Path, err)
						}
					}
				}
			}

			return nil
		},
	}

	generateComposefileCmd = cli.Command{
		Name:  "composefile",
		Usage: "Generate Compose files for examples that do not have custom ones.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Source directory containing the examples.",
				Value: "examples",
			},
			&cli.BoolFlag{
				Name:  "recreate",
				Usage: "If true, recreate all previously generated Compose files.",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			source := c.String("source")
			recreate := c.Bool("recreate")

			root, err := parseExamples(source)
			if err != nil {
				return err
			}

			// Enumerate all the example implementations.
			for _, c := range root.Categories {
				for _, e := range c.Examples {
					for _, i := range e.Implementations {
						if err := createComposefile(i.Language, i.Path, recreate); err != nil {
							log.Printf("%s: %s", i.Path, err)
						}
					}
				}
			}

			return nil
		},
	}

	generateOutputCmd = cli.Command{
		Name:  "output",
		Usage: "Generate execution output for examples.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Source directory containing the examples.",
				Value: "examples",
			},
			&cli.BoolFlag{
				Name:  "recreate",
				Usage: "If true, recreate all previously generated output files.",
			},
		},
		Action: func(c *cli.Context) error {
			source := c.String("source")
			recreate := c.Bool("recreate")

			root, err := parseExamples(source)
			if err != nil {
				return err
			}

			// Enumerate all the example implementations.
			for _, c := range root.Categories {
				for _, e := range c.Examples {
					for _, i := range e.Implementations {
						if err := generateOutput(i.Path, recreate); err != nil {
							log.Printf("%s: %s", i.Path, err)
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
				Usage: "Directory containing static files.",
				Value: "static",
			},
			&cli.StringFlag{
				Name:  "output",
				Usage: "Directory the built files will be written to. Note, this will delete the existing directory if present.",
				Value: "output",
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

			os.RemoveAll(output)
			os.MkdirAll(output, 0755)

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
