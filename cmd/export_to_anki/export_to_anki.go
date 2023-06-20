// mag utility to export the vocab.yml dataset to anki as a CSV

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	flags "github.com/jessevdk/go-flags"
	yaml "gopkg.in/yaml.v3"
)

const (
	deckNameGrEn  = "Mastronarde Attic Greek Greek-to-English"
	csvHeader     = "Front,Back,DeckName"
	deckColumnPos = 3
)

type Word struct {
	Gr  string
	En  string
	Cog string
	Pos string
}

type UnitVocab struct {
	Name  string
	Unit  int
	Vocab []Word
}

// Options
type Options struct {
	Verbose bool   `short:"v" long:"verbose" description:"display verbose output"`
	Unit    int    `short:"u" long:"unit" description:"export only this unit number"`
	Count   int    `short:"c" long:"count" description:"export only this many entries"`
	Outfile string `short:"o" long:"outfile" description:"path to output filename (use stdout if not set)"`
	Args    struct {
		Filename string `description:"vocab yml dataset to read" default:"vocab.yml"`
	} `positional-args:"yes"`
}

// exportVocab exports vocab in Anki CSV format to wtr
func exportVocab(wtr io.Writer, vocab []UnitVocab, opts Options) error {
	cwtr := csv.NewWriter(wtr)
	count := 1

	// Output file headers
	fmt.Fprintln(wtr, "#separator:Comma")
	fmt.Fprintf(wtr, "#columns:%s\n", csvHeader)
	fmt.Fprintf(wtr, "#deck column:%d\n", deckColumnPos)

	// Output vocab entries
	for _, u := range vocab {
		if opts.Unit > 0 && u.Unit != opts.Unit {
			continue
		}

		for _, w := range u.Vocab {
			deck := strings.Join([]string{deckNameGrEn, u.Name}, "::")
			err := cwtr.Write([]string{w.Gr, w.En, w.Cog, w.Pos, deck})
			if err != nil {
				return err
			}

			count++
			if opts.Count > 0 && count > opts.Count {
				break
			}
		}
	}

	cwtr.Flush()
	if err := cwtr.Error(); err != nil {
		return err
	}

	return nil
}

func RunCLI(wtr io.Writer, opts Options) error {
	dataset := opts.Args.Filename
	data, err := os.ReadFile(dataset)
	if err != nil {
		return err
	}

	var vocab []UnitVocab
	err = yaml.Unmarshal(data, &vocab)
	if err != nil {
		return err
	}

	stats := make(map[string]int)
	err = exportVocab(wtr, vocab, opts)
	if err != nil {
		return err
	}
	//stats["errors"] = errors

	if len(stats) > 0 {
		jstats, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintln(wtr, string(jstats))
	}

	return nil
}

func main() {
	log.SetFlags(0)
	// Parse default options are HelpFlag | PrintErrors | PassDoubleDash
	var opts Options
	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		if flags.WroteHelp(err) {
			os.Exit(0)
		}

		// Does PrintErrors work? Is it not set?
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err.Error())
		parser.WriteHelp(os.Stderr)
		os.Exit(2)
	}

	wtr := os.Stdout
	if opts.Outfile != "" {
		wtr, err = os.Create(opts.Outfile)
		if err != nil {
			log.Fatal("opening outfile: ", err)
		}
	}
	err = RunCLI(wtr, opts)
	if err != nil {
		log.Fatal(err)
	}
}
