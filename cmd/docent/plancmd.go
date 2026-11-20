package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/migara0/docent/internal/devnotes"
)

// ───────── docent plan ─────────

func newPlan() *cobra.Command {
	var libFlag string
	root := &cobra.Command{
		Use:   "plan",
		Short: "Keep the status board of a plan note: stages, steps, what is happening now, what is waited for",
		Long: "Works on notes of type `plan` (docent new <service> plan \"Title\"). The board lives in the note's front matter and Docent draws it at the top of the note and on the home page.\n" +
			"<note> is a note ID (DOC-0012), a path, a path under docs/, or a unique fragment of the file name.\n" +
			"A step is addressed as `Stage/Step` or by its name when that is unique; a stage name alone means every step of it.\n" +
			"Only the status, now, next, stages, waiting and updated keys of the front matter are written; the rest of the note is left as it is.",
	}
	root.PersistentFlags().StringVar(&libFlag, "library", "", "the developer-notes library (default: the one you are inside, or the only one)")

	open := func() (devnotes.Settings, error) {
		store, _, err := openStore()
		if err != nil {
			return devnotes.Settings{}, err
		}
		lib, err := devLibrary(store, libFlag)
		if err != nil {
			return devnotes.Settings{}, err
		}
		return devnotes.Resolve(lib), nil
	}
	// change runs one edit on a plan and prints the board afterwards.
	change := func(note string, fn func(p *devnotes.PlanDoc) (string, error)) error {
		s, err := open()
		if err != nil {
			return err
		}
		path, err := s.FindPlan(note)
		if err != nil {
			return err
		}
		var msg string
		before := ""
		doc, err := devnotes.Edit(path, time.Now(), func(p *devnotes.PlanDoc) error {
			before = p.Status
			var e error
			msg, e = fn(p)
			return e
		})
		if err != nil {
			return err
		}
		data, _ := os.ReadFile(path)
		_, updated, _ := devnotes.ParsePlan(data)
		_, title := devnotes.PlanHeader(data)
		if msg != "" {
			fmt.Println(msg)
		}
		if before != doc.Status {
			fmt.Printf("status: %s → %s\n", orNone(before), doc.Status)
		}
		fmt.Print(doc.View(updated, time.Now()).Text(title))
		return nil
	}

	for _, st := range []struct{ name, state, verb string }{
		{"done", devnotes.StepDone, "Mark a step (or a whole stage) done"},
		{"start", devnotes.StepNow, "Mark a step as the one being worked on now"},
		{"block", devnotes.StepBlocked, "Mark a step as blocked"},
		{"skip", devnotes.StepSkipped, "Leave a step out (it no longer counts)"},
		{"reset", devnotes.StepTodo, "Put a step back to to-do"},
	} {
		state := st.state
		root.AddCommand(&cobra.Command{
			Use:   st.name + " <note> <step>...",
			Short: st.verb,
			Args:  cobra.MinimumNArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return change(args[0], func(p *devnotes.PlanDoc) (string, error) {
					var done []string
					for _, ref := range args[1:] {
						what, err := p.SetState(ref, state)
						if err != nil {
							return "", err
						}
						done = append(done, what)
					}
					return fmt.Sprintf("%s: %s", state, strings.Join(done, ", ")), nil
				})
			},
		})
	}

	var after string
	addStage := &cobra.Command{
		Use:   "add-stage <note> <name>",
		Short: "Add a main stage (at the end, or after another stage)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return change(args[0], func(p *devnotes.PlanDoc) (string, error) {
				return "added stage " + args[1], p.AddStage(args[1], after)
			})
		},
	}
	addStage.Flags().StringVar(&after, "after", "", "put it after this stage")
	var afterStep string
	addStep := &cobra.Command{
		Use:   "add-step <note> <stage> <step>...",
		Short: "Add steps to a stage (at the end, or after another step)",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return change(args[0], func(p *devnotes.PlanDoc) (string, error) {
				return fmt.Sprintf("added %s to %s", strings.Join(args[2:], ", "), args[1]), p.AddSteps(args[1], args[2:], afterStep)
			})
		},
	}
	addStep.Flags().StringVar(&afterStep, "after", "", "put them after this step")
	remove := &cobra.Command{
		Use:   "rm <note> <step-or-stage>",
		Short: "Remove a step, or a whole stage",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return change(args[0], func(p *devnotes.PlanDoc) (string, error) {
				what, err := p.Remove(args[1])
				return "removed " + what, err
			})
		},
	}

	var who, blocks string
	wait := &cobra.Command{
		Use:   "wait <note> <what>",
		Short: "Record something other people have to do first",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return change(args[0], func(p *devnotes.PlanDoc) (string, error) {
				return "waiting: " + args[1], p.AddWait(devnotes.Wait{What: args[1], Who: who, Blocks: blocks}, time.Now())
			})
		},
	}
	wait.Flags().StringVar(&who, "who", "", "who has to do it")
	wait.Flags().StringVar(&blocks, "blocks", "", "the stage it holds up (shown as blocked)")
	unwait := &cobra.Command{
		Use:   "unwait <note> <number-or-text>",
		Short: "Remove a waiting item once it is done",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return change(args[0], func(p *devnotes.PlanDoc) (string, error) {
				w, err := p.RemoveWait(args[1])
				return "no longer waiting: " + w.What, err
			})
		},
	}
	text := func(key string) *cobra.Command {
		return &cobra.Command{
			Use:   key + " <note> [text]",
			Short: fmt.Sprintf("Set the %q line of the board (no text clears it, and the board works it out from the steps)", key),
			Args:  cobra.RangeArgs(1, 2),
			RunE: func(cmd *cobra.Command, args []string) error {
				val := ""
				if len(args) == 2 {
					val = args[1]
				}
				return change(args[0], func(p *devnotes.PlanDoc) (string, error) {
					if key == "now" {
						p.Now = val
					} else {
						p.Next = val
					}
					return "", nil
				})
			},
		}
	}
	status := &cobra.Command{
		Use:   "status <note> <planning|in-progress|dropped>",
		Short: "Set the plan's status by hand (done is set automatically when every step is done)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return change(args[0], func(p *devnotes.PlanDoc) (string, error) { return "", p.SetStatus(args[1]) })
		},
	}

	var asJSON bool
	show := &cobra.Command{
		Use:   "show <note>",
		Short: "Print the board of a plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := open()
			if err != nil {
				return err
			}
			path, err := s.FindPlan(args[0])
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			doc, updated, err := devnotes.ParsePlan(data)
			if err != nil {
				return err
			}
			v := doc.View(updated, time.Now())
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(v)
			}
			_, title := devnotes.PlanHeader(data)
			fmt.Print(v.Text(title))
			for _, p := range devnotes.PlanProblems(doc) {
				fmt.Fprintln(os.Stderr, "warning:", p)
			}
			return nil
		},
	}
	show.Flags().BoolVar(&asJSON, "json", false, "print the board as JSON")

	var all bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List the plans that are not finished, with progress, what is happening now and what is waited for",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := open()
			if err != nil {
				return err
			}
			shown := 0
			for _, p := range s.Plans() {
				data, err := os.ReadFile(p)
				if err != nil {
					continue
				}
				doc, updated, err := devnotes.ParsePlan(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", relTo(s.Root, p), err)
					continue
				}
				v := doc.View(updated, time.Now())
				if !all && (v.Status == "done" || v.Status == "dropped") {
					continue
				}
				id, title := devnotes.PlanHeader(data)
				line := fmt.Sprintf("%-9s %-12s %3d%%  %s", id, v.Status, v.Percent, title)
				if v.Now != "" {
					line += "  — " + v.Now
				}
				if n := len(v.Waiting); n > 0 {
					line += fmt.Sprintf("  (%d waiting)", n)
				}
				fmt.Println(line)
				fmt.Printf("          %s\n", relTo(s.Root, p))
				shown++
			}
			if shown == 0 {
				fmt.Println("no plans in progress")
			}
			return nil
		},
	}
	list.Flags().BoolVar(&all, "all", false, "include finished and dropped plans")

	root.AddCommand(addStage, addStep, remove, wait, unwait, text("now"), text("next"), status, show, list)
	return root
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
