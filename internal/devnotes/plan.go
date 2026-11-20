package devnotes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/migara0/docent/internal/library"
)

// Plan notes: one living note per task. The front matter holds the stages and steps (a short, structured list) and Docent draws it as a status board.
// The Markdown below the front matter is ordinary text (goal, decisions, notes).
//
//	status: in-progress            # planning, in-progress, done or dropped
//	now: Writing the cursor logic  # optional: the text shown as "Now" (otherwise the step marked now). Cleared when a step changes.
//	next: Validate bad cursors     # optional: the text shown as "Next" (otherwise the next step still to do). Cleared when a step changes.
//	stages:
//	  - name: Design
//	    steps:
//	      - {step: Contract, state: done}
//	waiting:
//	  - {what: Kafka topic created, who: DevOps, since: 2026-11-09, blocks: Release}
//
// Docent only writes a plan when you ask it to (the `docent plan` commands, or ticking a step in the page), and then only the keys above plus `updated`.

// Step and plan-status values.
const (
	StepTodo    = "todo"
	StepNow     = "now"
	StepDone    = "done"
	StepBlocked = "blocked"
	StepSkipped = "skipped"
)

var (
	stepStates   = []string{StepTodo, StepNow, StepDone, StepBlocked, StepSkipped}
	planStatuses = []string{"planning", "in-progress", "done", "dropped"}
)

// Step is one piece of work inside a stage.
type Step struct {
	Step  string `yaml:"step" json:"step"`
	State string `yaml:"state,omitempty" json:"state"`
}

// UnmarshalYAML accepts a bare string (a step still to do) as well as {step, state}.
func (s *Step) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		s.Step, s.State = n.Value, StepTodo
		return nil
	}
	type plain Step
	var p plain
	if err := n.Decode(&p); err != nil {
		return err
	}
	*s = Step(p)
	if s.State == "" {
		s.State = StepTodo
	}
	return nil
}

// Stage is a main stage of the work, with its steps.
type Stage struct {
	Name  string `yaml:"name" json:"name"`
	State string `yaml:"state,omitempty" json:"-"` // only for a stage without steps
	Steps []Step `yaml:"steps,omitempty" json:"-"`
}

// Wait is something other people have to do before work can go on.
type Wait struct {
	What   string `yaml:"what" json:"what"`
	Who    string `yaml:"who,omitempty" json:"who,omitempty"`
	Since  string `yaml:"since,omitempty" json:"since,omitempty"`
	Blocks string `yaml:"blocks,omitempty" json:"blocks,omitempty"` // the stage it holds up
}

// PlanDoc is the plan as written in the front matter.
type PlanDoc struct {
	Status  string  `yaml:"status"`
	Now     string  `yaml:"now"`
	Next    string  `yaml:"next"`
	Stages  []Stage `yaml:"stages"`
	Waiting []Wait  `yaml:"waiting"`
}

// ───────── the view: what the board shows ─────────

// StageView is a stage with its state worked out.
type StageView struct {
	Name    string `json:"name"`
	State   string `json:"state"` // done, active, todo or blocked
	Current bool   `json:"current,omitempty"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Steps   []Step `json:"steps"`
	Waiting int    `json:"waiting,omitempty"` // external tasks holding this stage up
}

// WaitView is an external task with its age.
type WaitView struct {
	Wait
	Days int `json:"days"` // days since it was raised; -1 when there is no date
}

// PlanView is what the UI draws.
type PlanView struct {
	Status  string      `json:"status"`
	Done    int         `json:"done"`
	Total   int         `json:"total"`
	Percent int         `json:"percent"`
	Now     string      `json:"now"`
	Next    string      `json:"next"`
	Stages  []StageView `json:"stages"`
	Waiting []WaitView  `json:"waiting"`
	Updated string      `json:"updated,omitempty"`
	Days    int         `json:"days"` // days since the plan was last updated; -1 when unknown
}

func countsAsWork(state string) bool { return state != StepSkipped }

// stageState works out a stage's state from its steps (or from its own state when it has none).
func stageState(st Stage, held bool) (state string, done, total int) {
	steps := st.Steps
	if len(steps) == 0 {
		steps = []Step{{Step: st.Name, State: st.State}}
	}
	active, blocked := false, false
	for _, s := range steps {
		switch s.State {
		case StepSkipped:
			continue
		case StepDone:
			done++
		case StepNow:
			active = true
		case StepBlocked:
			blocked = true
		}
		total++
	}
	switch {
	case total == 0 || done == total:
		return "done", done, total
	case blocked || held:
		return "blocked", done, total
	case active || done > 0:
		return "active", done, total
	}
	return "todo", done, total
}

func daysSince(day string, today time.Time) int {
	t, err := time.ParseInLocation("2006-01-02", day, today.Location())
	if err != nil {
		return -1
	}
	d := int(today.Sub(t).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return d
}

// View works the board out of the plan. updated is the plan note's `updated` date.
func (p *PlanDoc) View(updated string, today time.Time) *PlanView {
	v := &PlanView{Status: p.Status, Now: strings.TrimSpace(p.Now), Next: strings.TrimSpace(p.Next), Stages: []StageView{}, Waiting: []WaitView{}, Updated: updated, Days: -1}
	if v.Status == "" {
		v.Status = "planning"
	}
	if updated != "" {
		v.Days = daysSince(updated, today)
	}
	held := map[string]int{}
	for _, w := range p.Waiting {
		v.Waiting = append(v.Waiting, WaitView{Wait: w, Days: daysSince(w.Since, today)})
		if w.Blocks != "" {
			held[strings.ToLower(w.Blocks)]++
		}
	}
	var nowSteps []string
	var todo []string
	currentSet := false
	for _, st := range p.Stages {
		sv := StageView{Name: st.Name, Waiting: held[strings.ToLower(st.Name)], Steps: []Step{}}
		sv.Steps = append(sv.Steps, st.Steps...)
		sv.State, sv.Done, sv.Total = stageState(st, sv.Waiting > 0)
		if sv.State != "done" && !currentSet {
			sv.Current, currentSet = true, true
		}
		v.Done += sv.Done
		v.Total += sv.Total
		for _, s := range st.Steps {
			switch s.State {
			case StepNow:
				nowSteps = append(nowSteps, s.Step)
			case StepTodo, "":
				todo = append(todo, s.Step)
			}
		}
		if len(st.Steps) == 0 && countsAsWork(st.State) {
			switch st.State {
			case StepNow:
				nowSteps = append(nowSteps, st.Name)
			case StepTodo, "":
				todo = append(todo, st.Name)
			}
		}
		v.Stages = append(v.Stages, sv)
	}
	if v.Total > 0 {
		v.Percent = v.Done * 100 / v.Total
	}
	if v.Now == "" {
		v.Now = strings.Join(nowSteps, " · ")
	}
	if v.Next == "" && len(todo) > 0 {
		v.Next = todo[0]
	}
	return v
}

// ───────── reading ─────────

// splitFrontMatter returns the lines of the front matter (without the --- lines) and the rest of the file. ok is false when there is none.
func splitFrontMatter(data []byte) (fm []string, rest string, ok bool) {
	text := string(data)
	nl := "\n"
	lines := strings.Split(text, nl)
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return nil, text, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			return lines[1:i], strings.Join(lines[i+1:], nl), true
		}
	}
	return nil, text, false
}

// ParsePlan reads the plan from a note's Markdown. updated is the note's `updated` (or `date`) front matter value.
func ParsePlan(data []byte) (doc *PlanDoc, updated string, err error) {
	fm, _, ok := splitFrontMatter(data)
	if !ok {
		return nil, "", fmt.Errorf("the note has no front matter")
	}
	raw := strings.Join(fm, "\n")
	doc = &PlanDoc{}
	if err := yaml.Unmarshal([]byte(raw), doc); err != nil {
		return nil, "", fmt.Errorf("the plan in the front matter is not valid: %w", err)
	}
	var dates struct {
		Date    string `yaml:"date"`
		Updated string `yaml:"updated"`
	}
	_ = yaml.Unmarshal([]byte(raw), &dates)
	updated = dates.Updated
	if updated == "" {
		updated = dates.Date
	}
	return doc, updated, nil
}

// PlanProblems checks a plan's front matter.
func PlanProblems(p *PlanDoc) []string {
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	if p.Status != "" && !slices.Contains(planStatuses, p.Status) {
		add("status %q must be one of %s", p.Status, strings.Join(planStatuses, ", "))
	}
	if len(p.Stages) == 0 {
		add("a plan needs stages (Design, Plan, Develop, Testing, Release ...)")
	}
	seen := map[string]bool{}
	allDone, any := true, false
	for _, st := range p.Stages {
		key := strings.ToLower(st.Name)
		switch {
		case strings.TrimSpace(st.Name) == "":
			add("a stage has no name")
		case seen[key]:
			add("the stage %q is listed twice", st.Name)
		}
		seen[key] = true
		if len(st.Steps) == 0 && st.State != "" && !slices.Contains(stepStates, st.State) {
			add("stage %q has the state %q (use %s)", st.Name, st.State, strings.Join(stepStates, ", "))
		}
		names := map[string]bool{}
		for _, s := range st.Steps {
			if strings.TrimSpace(s.Step) == "" {
				add("a step of %q has no name", st.Name)
				continue
			}
			if names[strings.ToLower(s.Step)] {
				add("the step %q is listed twice in %q", s.Step, st.Name)
			}
			names[strings.ToLower(s.Step)] = true
			if !slices.Contains(stepStates, s.State) {
				add("the step %q has the state %q (use %s)", s.Step, s.State, strings.Join(stepStates, ", "))
			}
		}
		state, _, total := stageState(st, false)
		any = any || total > 0
		if state != "done" {
			allDone = false
		}
	}
	for _, w := range p.Waiting {
		if strings.TrimSpace(w.What) == "" {
			add("a waiting item has no `what`")
		}
		if w.Since != "" && !dayRe.MatchString(w.Since) {
			add("waiting since %q is not YYYY-MM-DD", w.Since)
		}
		if w.Blocks != "" && !seen[strings.ToLower(w.Blocks)] {
			add("waiting %q blocks the stage %q, which is not in the plan", w.What, w.Blocks)
		}
	}
	if any && allDone && (p.Status == "in-progress" || p.Status == "planning") {
		add("every step is done but the status is %s (set it to done)", p.Status)
	}
	return out
}

// ───────── writing ─────────

var planMu sync.Mutex // serialises read-modify-write of plan notes within this process

// plainText is what may be written without quotes inside {...}: it starts with a letter or digit and holds no colon, comma, bracket, hash or quote.
var plainText = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} _./+()&'’-]*$`)

// scalar writes a string as one line of YAML that is safe inside {...}: whitespace is collapsed, control characters are dropped, and anything that is not plain
// text (commas, brackets, colons, a leading dash ...) is double-quoted.
func scalar(s string) string {
	s = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0x7f {
			return -1
		}
		return r
	}, s)), " ")
	b, err := yaml.Marshal(s)
	out := strings.TrimRight(string(b), "\n")
	if err != nil || out != s || !plainText.MatchString(s) {
		q, _ := json.Marshal(s)
		return string(q)
	}
	return out
}

func stageLines(stages []Stage) []string {
	if len(stages) == 0 {
		return []string{"stages: []"}
	}
	out := []string{"stages:"}
	for _, st := range stages {
		if len(st.Steps) == 0 {
			state := st.State
			if state == "" {
				state = StepTodo
			}
			out = append(out, fmt.Sprintf("  - {name: %s, state: %s}", scalar(st.Name), scalar(state)))
			continue
		}
		out = append(out, "  - name: "+scalar(st.Name), "    steps:")
		for _, s := range st.Steps {
			state := s.State
			if state == "" {
				state = StepTodo
			}
			out = append(out, fmt.Sprintf("      - {step: %s, state: %s}", scalar(s.Step), scalar(state)))
		}
	}
	return out
}

func waitLines(ws []Wait) []string {
	if len(ws) == 0 {
		return []string{"waiting: []"}
	}
	out := []string{"waiting:"}
	for _, w := range ws {
		parts := []string{"what: " + scalar(w.What)}
		if w.Who != "" {
			parts = append(parts, "who: "+scalar(w.Who))
		}
		if w.Since != "" {
			since := scalar(w.Since)
			if dayRe.MatchString(w.Since) {
				since = w.Since // a date stays plain
			}
			parts = append(parts, "since: "+since)
		}
		if w.Blocks != "" {
			parts = append(parts, "blocks: "+scalar(w.Blocks))
		}
		out = append(out, "  - {"+strings.Join(parts, ", ")+"}")
	}
	return out
}

// topKeyExtent finds a top-level key in front matter lines: its first line and the line after its last (a key's block is the indented or "- " lines under it).
func topKeyExtent(fm []string, key string) (start, end int) {
	start = -1
	for i, l := range fm {
		if strings.HasPrefix(l, key+":") {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}
	end = start + 1
	for end < len(fm) {
		l := fm[end]
		if l == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t") || strings.HasPrefix(l, "- ") || l == "-" {
			end++
			continue
		}
		break
	}
	// blank lines at the end of a block belong to what follows
	for end > start+1 && strings.TrimSpace(fm[end-1]) == "" {
		end--
	}
	return start, end
}

// setTopKey replaces a top-level key's lines (or adds them at the end of the front matter). An empty lines removes the key.
func setTopKey(fm []string, key string, lines []string) []string {
	start, end := topKeyExtent(fm, key)
	if start < 0 {
		if len(lines) == 0 {
			return fm
		}
		return append(fm, lines...)
	}
	out := append([]string{}, fm[:start]...)
	out = append(out, lines...)
	return append(out, fm[end:]...)
}

// SavePlan writes the plan into the note: only status, now, next, stages, waiting and updated change; the rest of the file is kept as it is.
func SavePlan(path string, p *PlanDoc, today time.Time) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fm, rest, ok := splitFrontMatter(data)
	if !ok {
		return fmt.Errorf("%s has no front matter", path)
	}
	fm = setTopKey(fm, "status", []string{"status: " + scalar(orDefault(p.Status, "planning"))})
	for _, kv := range [][2]string{{"now", p.Now}, {"next", p.Next}} {
		var lines []string
		if strings.TrimSpace(kv[1]) != "" {
			lines = []string{kv[0] + ": " + scalar(kv[1])}
		}
		fm = setTopKey(fm, kv[0], lines)
	}
	fm = setTopKey(fm, "stages", stageLines(p.Stages))
	fm = setTopKey(fm, "waiting", waitLines(p.Waiting))
	fm = setTopKey(fm, "updated", []string{"updated: " + today.Format("2006-01-02")})
	var b bytes.Buffer
	b.WriteString("---\n")
	b.WriteString(strings.Join(fm, "\n"))
	b.WriteString("\n---\n")
	b.WriteString(rest)
	return writeAtomic(path, b.Bytes())
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".docent-plan-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// Edit reads the plan, lets fn change it, and writes it back. It is the only way plans are changed.
func Edit(path string, today time.Time, fn func(*PlanDoc) error) (*PlanDoc, error) {
	planMu.Lock()
	defer planMu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, _, err := ParsePlan(data)
	if err != nil {
		return nil, err
	}
	if err := fn(doc); err != nil {
		return nil, err
	}
	for _, s := range doc.Stages {
		for _, st := range s.Steps {
			if !slices.Contains(stepStates, st.State) {
				return nil, fmt.Errorf("the step %q has the state %q (use %s)", st.Step, st.State, strings.Join(stepStates, ", "))
			}
		}
	}
	syncStatus(doc)
	return doc, SavePlan(path, doc, today)
}

// syncStatus keeps the status in step with the work: the first started step moves a plan out of "planning", and finishing every step marks it done (undoing a step reopens it).
func syncStatus(p *PlanDoc) {
	if p.Status == "dropped" {
		return
	}
	started, all, total := false, true, 0
	for _, st := range p.Stages {
		state, done, n := stageState(st, false)
		total += n
		if state != "done" {
			all = false
		}
		if done > 0 || state == "active" || state == "blocked" {
			started = true
		}
	}
	switch {
	case total > 0 && all:
		p.Status = "done"
	case p.Status == "done" || (p.Status == "planning" || p.Status == "") && started:
		p.Status = "in-progress"
	case p.Status == "":
		p.Status = "planning"
	}
}

// ───────── operations ─────────

func fold(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// FindStage returns the index of a stage by name (exact, ignoring case; or a unique prefix).
func (p *PlanDoc) FindStage(ref string) (int, error) {
	ref = fold(ref)
	var hits []int
	for i, s := range p.Stages {
		if fold(s.Name) == ref {
			return i, nil
		}
		if strings.HasPrefix(fold(s.Name), ref) && ref != "" {
			hits = append(hits, i)
		}
	}
	switch len(hits) {
	case 0:
		return -1, fmt.Errorf("no stage %q (stages: %s)", ref, p.stageNames())
	case 1:
		return hits[0], nil
	}
	return -1, fmt.Errorf("%q matches several stages (%s)", ref, p.stageNames())
}

func (p *PlanDoc) stageNames() string {
	names := make([]string, len(p.Stages))
	for i, s := range p.Stages {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

type stepRef struct{ stage, step int } // step is -1 for a stage as a whole

// resolve finds what a reference points at: "Stage/Step", a step name, or a stage name.
func (p *PlanDoc) resolve(ref string) (stepRef, error) {
	// "Stage/Step": try every "/" as the split, so stage names that contain one (QA/UAT) still work
	for at := 0; at < len(ref); at++ {
		if ref[at] != '/' {
			continue
		}
		a, b := ref[:at], ref[at+1:]
		si := -1
		for i, st := range p.Stages {
			if fold(st.Name) == fold(a) {
				si = i
			}
		}
		if si < 0 {
			continue
		}
		if strings.TrimSpace(b) == "" {
			return stepRef{si, -1}, nil
		}
		for i, s := range p.Stages[si].Steps {
			if fold(s.Step) == fold(b) {
				return stepRef{si, i}, nil
			}
		}
		return stepRef{}, fmt.Errorf("no step %q in %q", b, p.Stages[si].Name)
	}
	if a, b, ok := strings.Cut(ref, "/"); ok { // not an exact stage name: allow a unique prefix of the stage
		si, err := p.FindStage(a)
		if err != nil {
			return stepRef{}, err
		}
		if strings.TrimSpace(b) == "" {
			return stepRef{si, -1}, nil
		}
		for i, s := range p.Stages[si].Steps {
			if fold(s.Step) == fold(b) {
				return stepRef{si, i}, nil
			}
		}
		return stepRef{}, fmt.Errorf("no step %q in %q", b, p.Stages[si].Name)
	}
	want := fold(ref)
	var exact, loose []stepRef
	for si, s := range p.Stages {
		if fold(s.Name) == want {
			exact = append(exact, stepRef{si, -1})
		}
		for i, st := range s.Steps {
			switch {
			case fold(st.Step) == want:
				exact = append(exact, stepRef{si, i})
			case strings.Contains(fold(st.Step), want):
				loose = append(loose, stepRef{si, i})
			}
		}
		if len(s.Steps) == 0 && fold(s.Name) != want && strings.Contains(fold(s.Name), want) {
			loose = append(loose, stepRef{si, -1})
		}
	}
	hits := exact
	if len(hits) == 0 {
		hits = loose
	}
	switch len(hits) {
	case 0:
		return stepRef{}, fmt.Errorf("nothing in the plan matches %q", ref)
	case 1:
		return hits[0], nil
	}
	names := make([]string, len(hits))
	for i, h := range hits {
		names[i] = p.describe(h)
	}
	return stepRef{}, fmt.Errorf("%q matches more than one: %s (use Stage/Step)", ref, strings.Join(names, "; "))
}

func (p *PlanDoc) describe(r stepRef) string {
	if r.step < 0 {
		return p.Stages[r.stage].Name
	}
	return p.Stages[r.stage].Name + "/" + p.Stages[r.stage].Steps[r.step].Step
}

// SetState marks a step (or every step of a stage) with a state and returns what it changed.
func (p *PlanDoc) SetState(ref, state string) (string, error) {
	if !slices.Contains(stepStates, state) {
		return "", fmt.Errorf("the state must be one of %s", strings.Join(stepStates, ", "))
	}
	r, err := p.resolve(ref)
	if err != nil {
		return "", err
	}
	st := &p.Stages[r.stage]
	p.Now, p.Next = "", "" // the hand-written lines describe a moment: once a step changes they would be out of date, so the board works them out from the steps again
	switch {
	case r.step >= 0:
		st.Steps[r.step].State = state
	case len(st.Steps) == 0:
		st.State = state
	default:
		for i := range st.Steps {
			if st.Steps[i].State != StepSkipped || state == StepSkipped {
				st.Steps[i].State = state
			}
		}
	}
	return p.describe(r), nil
}

// AddStage adds a stage at the end, or after another one.
func (p *PlanDoc) AddStage(name, after string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("give the stage a name")
	}
	if i, err := p.FindStage(name); err == nil && fold(p.Stages[i].Name) == fold(name) {
		return fmt.Errorf("the stage %q already exists", p.Stages[i].Name)
	}
	at := len(p.Stages)
	if after != "" {
		i, err := p.FindStage(after)
		if err != nil {
			return err
		}
		at = i + 1
	}
	p.Stages = slices.Insert(p.Stages, at, Stage{Name: name, State: StepTodo})
	return nil
}

// AddSteps adds steps to a stage (after a step when after is set, else at the end).
func (p *PlanDoc) AddSteps(stage string, names []string, after string) error {
	si, err := p.FindStage(stage)
	if err != nil {
		return err
	}
	st := &p.Stages[si]
	if len(st.Steps) == 0 && st.State != "" && st.State != StepTodo {
		return fmt.Errorf("the stage %q is marked %s as a whole; steps cannot be added to it", st.Name, st.State)
	}
	at := len(st.Steps)
	if after != "" {
		found := false
		for i, s := range st.Steps {
			if fold(s.Step) == fold(after) {
				at, found = i+1, true
			}
		}
		if !found {
			return fmt.Errorf("no step %q in %q", after, st.Name)
		}
	}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		for _, s := range st.Steps {
			if fold(s.Step) == fold(n) {
				return fmt.Errorf("the step %q already exists in %q", n, st.Name)
			}
		}
		st.Steps = slices.Insert(st.Steps, at, Step{Step: n, State: StepTodo})
		at++
	}
	st.State = ""
	return nil
}

// Remove deletes a step, or a whole stage when ref names a stage.
func (p *PlanDoc) Remove(ref string) (string, error) {
	r, err := p.resolve(ref)
	if err != nil {
		return "", err
	}
	what := p.describe(r)
	if r.step < 0 {
		p.Stages = slices.Delete(p.Stages, r.stage, r.stage+1)
	} else {
		st := &p.Stages[r.stage]
		st.Steps = slices.Delete(st.Steps, r.step, r.step+1)
	}
	return what, nil
}

// AddWait records something to wait for.
func (p *PlanDoc) AddWait(w Wait, today time.Time) error {
	w.What = strings.TrimSpace(w.What)
	if w.What == "" {
		return fmt.Errorf("say what you are waiting for")
	}
	if w.Since == "" {
		w.Since = today.Format("2006-01-02")
	}
	if w.Blocks != "" {
		i, err := p.FindStage(w.Blocks)
		if err != nil {
			return err
		}
		w.Blocks = p.Stages[i].Name
	}
	p.Waiting = append(p.Waiting, w)
	return nil
}

// RemoveWait drops an item by its number (1-based) or by text it contains. It returns the item.
func (p *PlanDoc) RemoveWait(ref string) (Wait, error) {
	idx := -1
	var n int
	if _, err := fmt.Sscanf(ref, "%d", &n); err == nil && fmt.Sprint(n) == strings.TrimSpace(ref) {
		if n < 1 || n > len(p.Waiting) {
			return Wait{}, fmt.Errorf("there is no waiting item %d (there are %d)", n, len(p.Waiting))
		}
		idx = n - 1
	} else {
		var hits []int
		for i, w := range p.Waiting {
			if strings.Contains(fold(w.What), fold(ref)) {
				hits = append(hits, i)
			}
		}
		switch len(hits) {
		case 0:
			return Wait{}, fmt.Errorf("nothing waiting matches %q", ref)
		case 1:
			idx = hits[0]
		default:
			return Wait{}, fmt.Errorf("%q matches %d waiting items; use its number", ref, len(hits))
		}
	}
	w := p.Waiting[idx]
	p.Waiting = slices.Delete(p.Waiting, idx, idx+1)
	return w, nil
}

// SetStatus sets the plan status by hand (for example to drop a plan).
func (p *PlanDoc) SetStatus(status string) error {
	if status == "done" {
		return fmt.Errorf("done is set automatically when every step is done (use `skip` for steps you drop)")
	}
	if !slices.Contains(planStatuses, status) {
		return fmt.Errorf("the status must be one of %s", strings.Join(planStatuses, ", "))
	}
	p.Status = status
	return nil
}

// ───────── checks and notices ─────────

func planProblems(s Settings, rel string) []string {
	data, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	doc, _, err := ParsePlan(data)
	if err != nil {
		return []string{err.Error()}
	}
	return PlanProblems(doc)
}

// A plan that nobody has touched for this many days while work is open, and a waiting item this old, are worth a nudge.
const (
	planStaleDays = 5
	waitStaleDays = 5
)

func planNotices(s Settings, rel string, pg *library.Page) []library.Notice {
	data, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	doc, updated, err := ParsePlan(data)
	if err != nil {
		return nil
	}
	v := doc.View(updated, time.Now())
	if v.Status != "in-progress" {
		return nil
	}
	var items []string
	for _, w := range v.Waiting {
		if w.Days >= waitStaleDays {
			who := ""
			if w.Who != "" {
				who = " (" + w.Who + ")"
			}
			items = append(items, fmt.Sprintf("%s%s: waiting for %d days", w.What, who, w.Days))
		}
	}
	var out []library.Notice
	if v.Days >= planStaleDays {
		out = append(out, library.Notice{Kind: "warning", Title: "This plan has not been updated for " + fmt.Sprint(v.Days) + " days", Text: "Check that the stages below still match where the work is."})
	}
	if len(items) > 0 {
		out = append(out, library.Notice{Kind: "warning", Title: "Waiting for a while", Items: items})
	}
	return out
}

// ───────── finding plans, and the text board ─────────

// Plans lists every plan note of the library (a note in a folder named plan, at any depth).
func (s Settings) Plans() []string {
	var out []string
	_ = filepath.WalkDir(s.Root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") && filepath.Base(filepath.Dir(p)) == "plan" {
			out = append(out, p)
		}
		return nil
	})
	slices.Sort(out)
	return out
}

// FindPlan resolves a plan note from a path, a path under docs/, a note ID (DOC-0012) or a fragment of the file name.
func (s Settings) FindPlan(ref string) (string, error) {
	for _, cand := range []string{ref, filepath.Join(s.Root, ref)} {
		if info, err := os.Stat(cand); err == nil && !info.IsDir() {
			return filepath.Abs(cand)
		}
	}
	all := s.Plans()
	var matches []string
	if want, ok := NormalizeID(ref, s.Prefix); ok {
		for _, p := range all {
			if data, err := os.ReadFile(p); err == nil {
				if fm, _, ok := splitFrontMatter(data); ok && slices.ContainsFunc(fm, func(l string) bool { return strings.TrimSpace(l) == "id: "+want }) {
					matches = append(matches, p)
				}
			}
		}
	}
	if len(matches) == 0 {
		for _, p := range all {
			if strings.Contains(filepath.Base(p), ref) {
				matches = append(matches, p)
			}
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("no plan note matches %q (see `docent plan list`)", ref)
	}
	rel := make([]string, len(matches))
	for i, m := range matches {
		rel[i], _ = filepath.Rel(s.Root, m)
	}
	return "", fmt.Errorf("several plan notes match %q:\n  %s", ref, strings.Join(rel, "\n  "))
}

// Text draws the board for a terminal: the stages on one line, then what is happening now, what is next and what is waited for.
func (v *PlanView) Text(title string) string {
	var b strings.Builder
	head := title
	if head == "" {
		head = "Plan"
	}
	fmt.Fprintf(&b, "%s  [%s, %d of %d steps, %d%%]\n", head, v.Status, v.Done, v.Total, v.Percent)
	parts := make([]string, len(v.Stages))
	for i, st := range v.Stages {
		mark := ""
		switch {
		case st.State == "done":
			mark = "✔ "
		case st.State == "blocked":
			mark = "⏸ "
		case st.Current:
			mark = "▶ "
		}
		count := ""
		if st.State != "done" && st.Total > 1 {
			count = fmt.Sprintf(" %d/%d", st.Done, st.Total)
		}
		parts[i] = mark + st.Name + count
	}
	b.WriteString(strings.Join(parts, " ── ") + "\n")
	for _, st := range v.Stages {
		if !st.Current || len(st.Steps) == 0 {
			continue
		}
		for _, s := range st.Steps {
			sym := map[string]string{StepDone: "[x]", StepNow: "[>]", StepBlocked: "[!]", StepSkipped: "[-]"}[s.State]
			if sym == "" {
				sym = "[ ]"
			}
			fmt.Fprintf(&b, "  %s %s\n", sym, s.Step)
		}
	}
	if v.Now != "" {
		fmt.Fprintf(&b, "NOW   %s\n", v.Now)
	}
	if v.Next != "" {
		fmt.Fprintf(&b, "NEXT  %s\n", v.Next)
	}
	for i, w := range v.Waiting {
		line := fmt.Sprintf("WAIT  %d. %s", i+1, w.What)
		var meta []string
		if w.Who != "" {
			meta = append(meta, w.Who)
		}
		if w.Days >= 0 {
			meta = append(meta, fmt.Sprintf("%dd", w.Days))
		}
		if w.Blocks != "" {
			meta = append(meta, "holds "+w.Blocks)
		}
		if len(meta) > 0 {
			line += " (" + strings.Join(meta, ", ") + ")"
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// PlanHeader returns the ID and title of a plan note.
func PlanHeader(data []byte) (id, title string) {
	fm, _, ok := splitFrontMatter(data)
	if !ok {
		return "", ""
	}
	var h struct {
		ID    string `yaml:"id"`
		Title string `yaml:"title"`
	}
	_ = yaml.Unmarshal([]byte(strings.Join(fm, "\n")), &h)
	return h.ID, h.Title
}
