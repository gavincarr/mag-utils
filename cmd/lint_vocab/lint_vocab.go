// mag utility to load and validate the vocab.yml dataset

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"

	flags "github.com/jessevdk/go-flags"
	yaml "gopkg.in/yaml.v3"
)

const (
	defaultDataset = "vocab.yml"
)

var (
	rePos = regexp.MustCompile(`^(n|v|adj|adv|pron|prep|conj|part)$`)
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
	Verbose bool `short:"v" long:"verbose" description:"display verbose output"`
	Unit    int  `short:"u" long:"unit" description:"lint only this unit number"`
	Args    struct {
		Filename string
	} `positional-args:"yes"`
}

func LintWord(wtr io.Writer, w Word, label string, i int) int {
	errors := 0
	if w.Gr == "" {
		fmt.Fprintf(wtr, "Empty 'gr' field found%s, word %d\n",
			label, i)
		errors++
	}
	if w.En == "" {
		fmt.Fprintf(wtr, "Empty 'en' field found%s, word %d\n",
			label, i)
		errors++
	}
	if w.Pos == "" {
		fmt.Fprintf(wtr, "Empty 'pos' field found%s, word %d\n",
			label, i)
		errors++
	} else if !rePos.MatchString(w.Pos) {
		fmt.Fprintf(wtr, "Invalid 'pos' value found%s, word %d: %q\n",
			label, i, w.Pos)
		errors++
	}
	return errors
}

// LintVocab runs a series of checks on vocab, and outputs
// any errors to stdout
func LintVocab(wtr io.Writer, opts Options, vocab []UnitVocab, stats *map[string]int) int {
	errors := 0
	if len(vocab) == 0 {
		fmt.Fprintln(wtr, "Empty vocab list!")
		errors++
		return errors
	}

	for _, u := range vocab {
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
		} else if u.Unit < 3 || u.Unit > 42 {
			fmt.Fprintf(wtr, "Invalid unit 'unit' field found%s: %d\n",
				label, u.Unit)
			errors++
		}
		if len(u.Vocab) == 0 {
			fmt.Fprintf(wtr, "Empty unit 'vocab' list found%s\n", label)
			errors++
			continue
		}
		if label == "" {
			continue
		}

		for i, w := range u.Vocab {
			(*stats)["words"]++
			errors += LintWord(wtr, w, label, i)
		}
	}

	return errors
}

func RunCLI(wtr io.Writer, opts Options) error {
	dataset := defaultDataset
	if opts.Args.Filename != "" {
		dataset = opts.Args.Filename
	}
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
	errors := LintVocab(wtr, opts, vocab, &stats)
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
