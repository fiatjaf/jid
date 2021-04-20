package yiq

import (
	"io"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/nsf/termbox-go"
)

const (
	DefaultY     int    = 1
	FilterPrompt string = "[yq]> "
)

type Engine struct {
	json          string
	query         *Query
	args          []string
	term          *Terminal
	autocomplete  string
	candidates    []string
	candidatemode bool
	candidateidx  int
	contentOffset int
	cursorOffsetX int
}

func NewEngine(s io.Reader, args []string, initialquery string) *Engine {
	j, err := ioutil.ReadAll(s)
	if err != nil {
		return &Engine{}
	}
	e := &Engine{
		json:          string(j),
		term:          NewTerminal(FilterPrompt, DefaultY),
		query:         NewQuery([]rune(initialquery)),
		args:          args,
		autocomplete:  "",
		candidates:    []string{},
		candidatemode: false,
		candidateidx:  0,
		contentOffset: 0,
		cursorOffsetX: len(initialquery),
	}
	return e
}

type EngineResult struct {
	Content string
	Qs      string
	Err     error
}

func (e *Engine) Run() *EngineResult {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	termbox.SetInputMode(termbox.InputAlt)

	var contents []string = []string{""}

	for {
		e.candidates = []string{}
		e.autocomplete = ""
		contents = e.getContents(contents)
		e.makeCandidates()
		e.setCandidateData()

		ta := &TerminalDrawAttributes{
			Query:           e.query.StringGet(),
			CursorOffsetX:   e.cursorOffsetX,
			Contents:        contents,
			CandidateIndex:  e.candidateidx,
			ContentsOffsetY: e.contentOffset,
			Complete:        e.autocomplete,
			Candidates:      e.candidates,
		}

		e.term.draw(ta)

		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case 0:
				if ev.Mod == termbox.ModAlt { // alt+something key shortcuts
					switch ev.Ch {
					case 'f':
						// move one word forward
						filter := e.query.StringGet()
						if len(filter) > e.cursorOffsetX {
							n := strings.IndexAny(filter[e.cursorOffsetX:], "[.")
							if n == -1 {
								e.cursorOffsetX = len(filter)
							} else {
								e.cursorOffsetX += n + 1
							}
						}
					case 'b':
						// move one word backwards
						filter := e.query.StringGet()
						if 0 < e.cursorOffsetX {
							n := strings.LastIndexAny(filter[:e.cursorOffsetX], "[.")
							if n == -1 {
								e.cursorOffsetX = 0
							} else {
								e.cursorOffsetX = n
							}
						}
					}
				} else {
					e.inputChar(ev.Ch)
				}
			case termbox.KeySpace:
				e.inputChar(32)
			case termbox.KeyBackspace, termbox.KeyBackspace2:
				// delete previous char
				if e.cursorOffsetX > 0 {
					_ = e.query.Delete(e.cursorOffsetX - 1)
					e.cursorOffsetX -= 1
				}
			case termbox.KeyDelete:
				e.query.Delete(e.cursorOffsetX) // delete next char
			case termbox.KeyCtrlU:
				e.query.Clear()
			case termbox.KeyTab:
				e.tabAction()
			case termbox.KeyArrowLeft:
				// move cursor left
				if e.cursorOffsetX > 0 {
					e.cursorOffsetX -= 1
				}
			case termbox.KeyArrowRight:
				// move cursor right
				if len(e.query.Get()) > e.cursorOffsetX {
					e.cursorOffsetX += 1
				}
			case termbox.KeyHome, termbox.KeyCtrlA: // move to start of line
				e.cursorOffsetX = 0
			case termbox.KeyEnd, termbox.KeyCtrlE: // move to end of line
				e.cursorOffsetX = len(e.query.Get())
			case termbox.KeyCtrlW:
				// delete the word before the cursor
				e.deleteWordBackward()
			case termbox.KeyCtrlK, termbox.KeyArrowUp:
				e.scrollToAbove()
			case termbox.KeyCtrlJ, termbox.KeyArrowDown:
				e.scrollToBelow()
			case termbox.KeyCtrlN, termbox.KeyPgdn:
				_, h := termbox.Size()
				e.scrollPageDown(len(contents), h)
			case termbox.KeyCtrlP, termbox.KeyPgup:
				_, h := termbox.Size()
				e.scrollPageUp(h)
			case termbox.KeyEsc:
				e.candidatemode = false
			case termbox.KeyEnter:
				if !e.candidatemode {
					cc, err := yqrun(e.query.StringGet(), e.json, e.args)

					return &EngineResult{
						Content: cc,
						Qs:      e.query.StringGet(),
						Err:     err,
					}
				}
				e.confirmCandidate()
			case termbox.KeyCtrlC:
				return &EngineResult{}
			default:
			}
		case termbox.EventError:
			panic(ev.Err)
			break
		default:
		}
	}
}

func (e *Engine) getContents(prevContents []string) []string {
	cc, err := yqrun(e.query.StringGet(), e.json, e.args)
	if err == nil {
		return strings.Split("\n"+cc, "\n")
	} else {
		errorline := []string{cc}
		if newline := strings.Index(cc, "\n"); newline != -1 {
			errorline = []string{cc[0:newline]}
		}

		return append(
			errorline,
			prevContents[1:]...,
		)
	}
}

var complicatedKeyRegex = regexp.MustCompile(`\d|\W`)

func (e *Engine) makeCandidates() {
	filter := e.query.StringGet()
	if strings.IndexAny(strings.TrimLeft(filter, " "), "|{} @(),") == -1 {
		// try to find suggestions since it seems to be a simple jq filter
		validUntilNow, next := e.query.StringSplitLastKeyword()
		if validUntilNow == "" {
			validUntilNow = "."
		}
		keys, err := yqrun(validUntilNow+" | keys", e.json, []string{"-c"})
		if err == nil {
			candidates := strings.Split(keys[1:len(keys)-1], ",")
			if len(candidates[0]) > 0 && candidates[0][0] == '"' {
				// only suggest if keys are strings
				for _, cand := range candidates {
					// filter out candidates with the wrong prefix
					if strings.HasPrefix(cand, `"`+next) {
						cand = cand[1 : len(cand)-1] // remove quotes
						if complicatedKeyRegex.FindStringIndex(cand) != nil {
							// superquote ["<value>"] complicated keys
							cand = `["` + cand + `"]`
						}
						e.candidates = append(e.candidates, cand)
					}
				}

				// if there's only one candidate, let it be our autocomplete suggestion
				if len(e.candidates) == 1 {
					e.autocomplete = e.candidates[0][len(next):]
				}
			}
		}
	}
}

func (e *Engine) setCandidateData() {
	ncandidates := len(e.candidates)
	if ncandidates == 1 {
		e.candidatemode = false
	} else if ncandidates > 1 {
		if e.candidateidx >= ncandidates {
			e.candidateidx = 0
		}
	} else {
		e.candidatemode = false
	}
	if !e.candidatemode {
		e.candidateidx = 0
		e.candidates = []string{}
	}
}

func (e *Engine) confirmCandidate() {
	filter, _ := e.query.StringSplitLastKeyword()
	if e.candidates[e.candidateidx][0] != '[' {
		filter += "."
	}
	filter += e.candidates[e.candidateidx]
	e.query.StringSet(filter)
	e.cursorOffsetX = len(filter)
	e.candidatemode = false
}

func (e *Engine) scrollToBelow() {
	e.contentOffset++
}
func (e *Engine) scrollToAbove() {
	if o := e.contentOffset - 1; o >= 0 {
		e.contentOffset = o
	}
}
func (e *Engine) scrollPageDown(rownum int, height int) {
	co := rownum - 1
	if o := rownum - e.contentOffset; o > height {
		co = e.contentOffset + (height - DefaultY)
	}
	e.contentOffset = co
}
func (e *Engine) scrollPageUp(height int) {
	co := 0
	if o := e.contentOffset - (height - DefaultY); o > 0 {
		co = o
	}
	e.contentOffset = co
}
func (e *Engine) deleteWordBackward() {
	if k, _ := e.query.StringPopKeyword(); k != "" && !strings.Contains(k, "[") {
		_ = e.query.StringAdd(".")
	}
	e.cursorOffsetX = len(e.query.Get())
}
func (e *Engine) tabAction() {
	if !e.candidatemode {
		e.candidatemode = true
		if e.query.StringGet() == "" {
			e.query.StringAdd(".")
			e.cursorOffsetX = 1
		} else if e.autocomplete != "" {
			valid, next := e.query.StringSplitLastKeyword()
			filter := valid + "." + next + e.autocomplete
			e.query.StringSet(filter)
			e.cursorOffsetX = len(filter)
		}
	} else {
		e.candidateidx = e.candidateidx + 1
	}
}
func (e *Engine) inputChar(ch rune) {
	b := len(e.query.Get())
	q := e.query.StringInsert(string(ch), e.cursorOffsetX)
	if b < len(q) {
		e.cursorOffsetX += 1
	}
}
