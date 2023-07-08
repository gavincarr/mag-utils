// mag utility to export the vocab.yml dataset to anki as a CSV

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
	deckNameGrEn   = "Mastronarde Attic Greek Vocab (Greek-to-English)"
	csvCommentGrEn = "# This is an export of the MAG vocab dataset in Anki CSV format (Greek-to-English)"
	csvHeader      = "ID,Front,Back,Tags,DeckName"
	deckColumnPos  = 5
)

var (
	reCommaStar            = regexp.MustCompile(`,.*$`)
	reSemicolon            = regexp.MustCompile(`\pZ*;\pZ*`)
	reSemicolonParenthesis = regexp.MustCompile(`\pZ*;\pZ*\(`)
	reCaseMarker           = regexp.MustCompile(`^\(\+\pZ*(acc|gen|dat)\.?\)`)
	reVoiceMarker          = regexp.MustCompile(`^\([^(]*(mid|pass)\.[^)]*\)`)
	rePluralMarker         = regexp.MustCompile(`^\(pl\.\)`)

	posMap = map[string]string{
		"adj":  "adjective",
		"adv":  "adverb",
		"conj": "conjunction",
		"n":    "noun",
		"part": "particle",
		"prep": "preposition",
		"pron": "pronoun",
		"v":    "verb",
	}
)

type Word struct {
	Gr    string
	GrMP  string `yaml:"gr_mp"`
	GrPl  string `yaml:"gr_pl"`
	GrExt string `yaml:"gr_ext"`
	Id    string
	En    string
	EnExt string `yaml:"en_ext"`
	Cog   string
	Pos   string
}

type UnitVocab struct {
	Name  string
	Unit  int
	Vocab []Word
}

type CaseVoiceGloss struct {
	Case   string
	Voice  string
	Plural bool
	Marker string
	Gloss  string
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

// parsePrepGlosses parses a gloss into one or more CaseVoiceGloss records,
// breaking where a gloss includes a leading case marker (e.g. acc/gen/dat).
// where CaseVoiceGloss.Case is the bare case string ("acc", "gen", "dat"),
// and CaseVoiceGloss.Gloss is the gloss entry for that case
func parsePrepGlosses(gloss string) []CaseVoiceGloss {
	entries := reSemicolon.Split(gloss, -1)
	cglist := []CaseVoiceGloss{}
	cg := CaseVoiceGloss{}
	for i, entry := range entries {
		matches := reCaseMarker.FindStringSubmatch(entry)
		if matches == nil {
			// The first entry not having a case marker is a fatal error
			if i == 0 {
				log.Fatalf("preposition entry without initial case marker: %s",
					gloss)
			}
			// Subsequent entries without case markers just get appended to current
			cg.Gloss += "; " + entry
			continue
		}

		if cg.Case != "" {
			cglist = append(cglist, cg)
		}
		// Remove case marker from the gloss
		gloss := strings.TrimSpace(strings.Replace(entry, matches[0], "", 1))
		cg = CaseVoiceGloss{Case: matches[1], Marker: matches[0], Gloss: gloss}
	}
	if cg.Case != "" {
		cglist = append(cglist, cg)
	}
	return cglist
}

// parseVoiceGlosses parses a gloss into one or more CaseVoiceGloss records,
// breaking where a gloss includes a leading voice marker (e.g. mid/pass).
// CaseVoiceGloss.Voice is the bare voice string ("mid" or "pass"),
// and CaseVoiceGloss.Gloss is the gloss entry for that voice (including
// the introductory "(voice.)" Marker)
func parseVoiceGlosses(gloss string) []CaseVoiceGloss {
	entries := reSemicolon.Split(gloss, -1)
	cglist := []CaseVoiceGloss{}
	cg := CaseVoiceGloss{}
	for _, entry := range entries {
		matches := reVoiceMarker.FindStringSubmatch(entry)
		if matches == nil {
			// If no voice marker, just add to current
			if cg.Gloss == "" {
				cg.Gloss = entry
			} else {
				cg.Gloss += "; " + entry
			}
			continue
		}

		voice := matches[1]
		if cg.Voice == voice {
			// If we have multiple matches, just append to current
			cg.Gloss += "; " + entry
			continue
		}

		if cg.Gloss != "" {
			cglist = append(cglist, cg)
		}
		cg = CaseVoiceGloss{Voice: voice, Gloss: entry}
	}
	if cg.Gloss != "" {
		cglist = append(cglist, cg)
	}
	return cglist
}

func parsePluralGlosses(gloss string) []CaseVoiceGloss {
	entries := reSemicolon.Split(gloss, -1)
	cglist := []CaseVoiceGloss{}
	cg := CaseVoiceGloss{}
	for _, entry := range entries {
		matches := rePluralMarker.FindStringSubmatch(entry)
		if matches == nil {
			// If no plural marker, just add to current
			if cg.Gloss == "" {
				cg.Gloss = entry
			} else {
				cg.Gloss += "; " + entry
			}
			continue
		}

		if cg.Gloss != "" {
			cglist = append(cglist, cg)
		}
		cg = CaseVoiceGloss{Plural: true, Gloss: entry}
	}
	if cg.Gloss != "" {
		cglist = append(cglist, cg)
	}
	return cglist
}

// exportVocab exports vocab in Anki CSV format to wtr
func exportVocab(wtr io.Writer, vocab []UnitVocab, opts Options) error {
	cwtr := csv.NewWriter(wtr)
	count := 1
	idmap := make(map[string]struct{})

	// Output file headers
	fmt.Fprintln(wtr, csvCommentGrEn)
	fmt.Fprintln(wtr, "#separator:Comma")
	fmt.Fprintf(wtr, "#columns:%s\n", csvHeader)
	fmt.Fprintf(wtr, "#deck column:%d\n", deckColumnPos)
	fmt.Fprintln(wtr, "#html:true")

	// Output vocab entries
	for _, u := range vocab {
		if opts.Unit > 0 && u.Unit != opts.Unit {
			continue
		}

		for _, w := range u.Vocab {
			var id string
			if w.Id != "" {
				id = w.Id
			} else {
				id = reCommaStar.ReplaceAllString(w.Gr, "")
			}

			// Make sure ids are unique
			if _, exists := idmap[id]; exists {
				log.Fatal("duplicate ids found: ", id)
			}
			idmap[id] = struct{}{}
			pos, ok := posMap[w.Pos]
			if !ok {
				log.Fatalf("bad POS %q found on word %q/%q",
					w.Pos, w.Gr, w.En)
			}

			front := w.Gr
			if w.GrExt != "" {
				front += " " + w.GrExt
			}
			tags := []string{"pos::" + pos}
			tagstr := strings.Join(tags, " ")
			deck := strings.Join([]string{deckNameGrEn, u.Name}, "::")

			// For prepositions, split into per-case entries
			var glosses []CaseVoiceGloss
			if w.Pos == "prep" {
				glosses = parsePrepGlosses(w.En)
				if w.EnExt != "" {
					fmt.Fprintf(os.Stderr, "Warning: en_ext is unsupported with prepositions - skipping for %q\n", front)
				}
			} else if w.GrMP != "" {
				// If a separate middle/passive form is defined, parse
				// voice glosses
				glosses = parseVoiceGlosses(w.En)
			} else if w.GrPl != "" {
				// If a separate plural form is defined, parse plural glosses
				glosses = parsePluralGlosses(w.En)
			}
			//fmt.Fprintf(os.Stderr, "+ %s: %v\n", id, glosses)
			if len(glosses) > 1 {
				for _, cg := range glosses {
					id2 := id
					if cg.Case != "" {
						id2 = id + "-" + cg.Case
						front = w.Gr + " " + cg.Marker
						if w.GrExt != "" {
							front += " " + w.GrExt
						}
					} else if (cg.Voice == "mid" || cg.Voice == "pass") &&
						w.GrMP != "" {
						id2 = w.GrMP
						front = w.GrMP
					} else if w.GrPl != "" && cg.Plural {
						id2 = reCommaStar.ReplaceAllString(w.GrPl, "")
						front = w.GrPl
					}
					back := cg.Gloss
					back = reSemicolonParenthesis.ReplaceAllString(back, "<br>(")
					// Write entry
					err := cwtr.Write([]string{
						id2, front, back, tagstr, deck})
					if err != nil {
						return err
					}
				}
			} else {
				back := w.En
				back = reSemicolonParenthesis.ReplaceAllString(back, "<br>(")
				if w.EnExt != "" {
					back += "<br><i>" + w.EnExt + "</i>"
				}
				if w.Cog != "" {
					back += "<br>[" + w.Cog + "]"
				}
				// Write entry
				err := cwtr.Write([]string{id, front, back, tagstr, deck})
				if err != nil {
					return err
				}
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
