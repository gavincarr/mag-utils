// mag-utils utility to load and validate the given mag pp.yml dataset

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"

	flags "github.com/jessevdk/go-flags"
	yaml "gopkg.in/yaml.v3"
)

var (
	reEntry = regexp.MustCompile(`^\(?-?\p{Greek}+( (or|and) \p{Greek}+)?(\pZ+\(stem \p{Greek}+-\))?\)?$`)
)

type Record struct {
	Pr string
	Fu string
	Ao string
	Pf string
	Pm string
	Ap string
}

type UnitPP struct {
	Name string
	Unit int
	PP   []Record
}

// Options
type Options struct {
	Verbose bool `short:"v" long:"verbose" description:"display verbose output"`
	Unit    int  `short:"u" long:"unit" description:"lint only this unit number"`
	Args    struct {
		Filename string `description:"principal parts yml dataset to read" default:"pp.yml"`
	} `positional-args:"yes"`
}

func checkWord(word, pptype, label string) error {
	/*
		fmt.Fprintf(os.Stderr, "+ %s:\n", word)
		for _, r := range word {
			fmt.Fprintf(os.Stderr, "  - %x %c\n", r, r)
		}
	*/
	if !reEntry.MatchString(word) {
		return errors.New(fmt.Sprintf("Bad %q entry found%s: %q",
			pptype, label, word))
	}
	return nil
}

func LintRecord(wtr io.Writer, rec Record, label string) int {
	errors := 0
	if rec.Pr != "" {
		err := checkWord(rec.Pr, "pr", label)
		if err != nil {
			fmt.Fprintln(wtr, err.Error())
			errors++
		}
	}
	if rec.Fu != "" {
		err := checkWord(rec.Fu, "fu", label)
		if err != nil {
			fmt.Fprintln(wtr, err.Error())
			errors++
		}
	}
	if rec.Ao != "" {
		err := checkWord(rec.Ao, "ao", label)
		if err != nil {
			fmt.Fprintln(wtr, err.Error())
			errors++
		}
	}
	if rec.Pf != "" {
		err := checkWord(rec.Pf, "pf", label)
		if err != nil {
			fmt.Fprintln(wtr, err.Error())
			errors++
		}
	}
	if rec.Pm != "" {
		err := checkWord(rec.Pm, "pm", label)
		if err != nil {
			fmt.Fprintln(wtr, err.Error())
			errors++
		}
	}
	if rec.Ap != "" {
		err := checkWord(rec.Ap, "ap", label)
		if err != nil {
			fmt.Fprintln(wtr, err.Error())
			errors++
		}
	}
	return errors
}

// LintPP runs a series of checks on pp, and outputs
// any errors to stdout
func LintPP(wtr io.Writer, opts Options, pp []UnitPP, stats *map[string]int) int {
	errors := 0
	if len(pp) == 0 {
		fmt.Fprintln(wtr, "Empty pp list!")
		errors++
		return errors
	}

	for _, u := range pp {
		if opts.Unit > 0 && u.Unit != opts.Unit {
			continue
		}

		(*stats)["units"]++
		var label string
		if u.Name != "" {
			label = fmt.Sprintf(" for unit %q", u.Name)
		} else if u.Unit >= 3 {
			label = fmt.Sprintf(" for unit %d", u.Unit)
		}
		if u.Name == "" {
			fmt.Fprintf(wtr, "Empty unit 'name' field found%s\n", label)
			errors++
		}
		if u.Unit == 0 {
			fmt.Fprintf(wtr, "Empty unit 'unit' field found%s\n", label)
			errors++
		} else if u.Unit < 5 || u.Unit > 42 {
			fmt.Fprintf(wtr, "Invalid unit 'unit' field found%s: %d\n",
				label, u.Unit)
			errors++
		}
		if len(u.PP) == 0 {
			fmt.Fprintf(wtr, "Empty unit 'pp' list found%s\n", label)
			errors++
			continue
		}
		if label == "" {
			continue
		}

		for _, rec := range u.PP {
			(*stats)["records"]++
			errors += LintRecord(wtr, rec, label)
		}
	}

	return errors
}

func RunCLI(wtr io.Writer, opts Options) error {
	dataset := opts.Args.Filename
	data, err := os.ReadFile(dataset)
	if err != nil {
		return err
	}

	var pp []UnitPP
	err = yaml.Unmarshal(data, &pp)
	if err != nil {
		return err
	}

	stats := make(map[string]int)
	errors := LintPP(wtr, opts, pp, &stats)
	stats["errors"] = errors

	jstats, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(wtr, string(jstats))

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

	err = RunCLI(os.Stdout, opts)
	if err != nil {
		log.Fatal(err)
	}
}
