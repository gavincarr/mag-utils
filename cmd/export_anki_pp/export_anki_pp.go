// mag utility to export the pp.yml dataset to anki as a CSV

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	flags "github.com/jessevdk/go-flags"
	yaml "gopkg.in/yaml.v3"
)

const (
	deckNameGrEn   = "Mastronarde Attic Greek Principal Parts (Greek-to-English)"
	csvCommentGrEn = "# This is an export of the MAG principal parts dataset in Anki CSV format (Greek-to-English)"
	csvHeader      = "ID,Front,Back,Tags,DeckName"
	deckColumnPos  = 5
)

var (
	reAlternates = regexp.MustCompile(`(\()?(\p{Greek}+)\pZ+(or|and)\pZ+(\p{Greek}+)(\))?`)
	reSpace      = regexp.MustCompile(`\pZ+`)
)

type Parts struct {
	Present string `yaml:"pr"`
	Future  string `yaml:"fu"`
	Aorist  string `yaml:"ao"`
	Perfect string `yaml:"pf"`
	PerfMid string `yaml:"pm"`
	AorPass string `yaml:"ap"`
}

type UnitPP struct {
	Name string
	Unit int
	PP   []Parts
}

// Options
type Options struct {
	Verbose bool   `short:"v" long:"verbose" description:"display verbose output"`
	Unit    int    `short:"u" long:"unit" description:"export only this unit number"`
	Num     int    `short:"n" long:"num" description:"export only the first N principal parts (2-6)" default:"6"`
	Outfile string `short:"o" long:"outfile" description:"path to output filename (use stdout if not set)"`
	Args    struct {
		Filename string `description:"pp yml dataset to read" default:"pp.yml"`
	} `positional-args:"yes"`
}

func exportSingleEntry(cwtr *csv.Writer, u UnitPP, id, label, ppstr, conj string, n int) error {
	labeltag := reSpace.ReplaceAllString(strings.ToLower(label), "_")
	tagstr := "pp::" + labeltag
	deck := strings.Join([]string{deckNameGrEn, u.Name}, "::")

	nstr := ""
	if n > 0 {
		nstr = fmt.Sprintf(" #%d", n)
	}
	meaning := ""
	switch conj {
	case "and":
		meaning = " (diff. meaning)"
	case "or":
		meaning = " (same meaning)"
	}
	back := fmt.Sprintf("%s%s of %s%s", label, nstr, id, meaning)

	err := cwtr.Write([]string{ppstr, ppstr, back, tagstr, deck})
	if err != nil {
		return err
	}

	return nil
}

func exportEntry(cwtr *csv.Writer, u UnitPP, id, label, ppstr string) error {
	matches := reAlternates.FindStringSubmatch(ppstr)
	if matches == nil {
		return exportSingleEntry(cwtr, u, id, label, ppstr, "", 0)
	}

	paren1 := matches[1]
	part1 := matches[2]
	conj := matches[3]
	part2 := matches[4]
	paren2 := matches[5]

	if paren1 != "" {
		if paren2 != "" {
			log.Fatal("missing paren2 in alternate:", ppstr)
		}
		part1 = "(" + part1 + ")"
		part2 = "(" + part2 + ")"
	}

	err := exportSingleEntry(cwtr, u, id, label, part1, conj, 1)
	if err != nil {
		return err
	}
	err = exportSingleEntry(cwtr, u, id, label, part2, conj, 2)
	if err != nil {
		return err
	}

	return nil
}

// exportPP exports principal parts in Anki CSV format to wtr
func exportPP(wtr io.Writer, upp []UnitPP, opts Options) error {
	cwtr := csv.NewWriter(wtr)
	idmap := make(map[string]struct{})

	// Output file headers
	fmt.Fprintln(wtr, csvCommentGrEn)
	fmt.Fprintln(wtr, "#separator:Comma")
	fmt.Fprintf(wtr, "#columns:%s\n", csvHeader)
	fmt.Fprintf(wtr, "#deck column:%d\n", deckColumnPos)
	fmt.Fprintln(wtr, "#html:false")

	// Output pp entries
	for _, u := range upp {
		for _, pp := range u.PP {
			if opts.Unit > 0 && u.Unit != opts.Unit {
				continue
			}

			// Make sure ids are unique
			id := pp.Present
			if _, exists := idmap[id]; exists {
				log.Fatal("duplicate ids found: ", id)
			}
			idmap[id] = struct{}{}

			// Export entries for each principal part
			if pp.Future != "" {
				exportEntry(cwtr, u, id, "Future", pp.Future)
			}
			if opts.Num >= 3 && pp.Aorist != "" {
				exportEntry(cwtr, u, id, "Aorist", pp.Aorist)
			}
			if opts.Num >= 4 && pp.Perfect != "" {
				exportEntry(cwtr, u, id, "Perfect", pp.Perfect)
			}
			if opts.Num >= 5 && pp.PerfMid != "" {
				exportEntry(cwtr, u, id, "Perfect Middle", pp.PerfMid)
			}
			if opts.Num == 6 && pp.AorPass != "" {
				exportEntry(cwtr, u, id, "Aorist Passive", pp.AorPass)
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

	var pp []UnitPP
	err = yaml.Unmarshal(data, &pp)
	if err != nil {
		return err
	}

	stats := make(map[string]int)
	err = exportPP(wtr, pp, opts)
	if err != nil {
		return err
	}

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
	if opts.Num < 2 || opts.Num > 6 {
		log.Fatal("Error: invalid -n/--num value")
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
