package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	examples "github.com/go-git/go-git/v5/_examples"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
	"github.com/jessevdk/go-flags"
)

// Options represents command line options
type Options struct {
	Depth uint `short:"d" long:"depth" description:"The amount of commits traced back" default:"1000"`
}

// Fix represents a bugfix commit
type Fix struct {
	Message string
	Time    time.Time
	Changes object.Changes
}

// Spot stores a score showing how buggy a file is
type Spot struct {
	File  string
	Score float64
}

// SpotList is a slice of Spot
type SpotList []Spot

// these are interface for sort
func (s SpotList) Len() int           { return len(s) }
func (s SpotList) Less(i, j int) bool { return s[i].Score < s[j].Score }
func (s SpotList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func getChangeName(change *object.Change) string {
	var empty = object.ChangeEntry{}
	if change.From != empty {
		return change.From.Name
	}

	return change.To.Name
}

func main() {
	var opts Options

	args, err := flags.Parse(&opts)
	examples.CheckIfError(err)

	directory := "."
	if len(args) >= 1 {
		directory = args[0]
	}

	repo, err := git.PlainOpen(directory)
	examples.CheckIfError(err)

	ref, err := repo.Head()
	examples.CheckIfError(err)

	currentTime := time.Now()
	since := currentTime.AddDate(0, -6, 0)

	cIter, err := repo.Log(&git.LogOptions{
		From:  ref.Hash(),
		Since: &since,
		Until: &currentTime,
		Order: git.LogOrderCommitterTime,
	})
	examples.CheckIfError(err)
	defer cIter.Close()

	var fixes = make([]Fix, 0)
	var prevCommit *object.Commit
	var prevTree *object.Tree

	for {
		commit, err := cIter.Next()
		if commit == nil {
			fmt.Printf("\n")
			break
		}
		examples.CheckIfError(err)

		currentTree, err := commit.Tree()
		examples.CheckIfError(err)

		if prevCommit == nil {
			prevCommit = commit
			prevTree = currentTree
		}

		changes, err := currentTree.Diff(prevTree)
		examples.CheckIfError(err)

		msg := strings.Split(commit.Message, "\n")[0]

		fix := Fix{
			Message: msg,
			Time:    commit.Committer.When,
			Changes: changes,
		}

		fixes = append(fixes, fix)
		fmt.Printf(".")
	}

	oldestFixTime := fixes[len(fixes)-1].Time

	fmt.Printf("currentTime     : %s\n", currentTime.UTC())
	fmt.Printf("oldestCommitTime: %s\n", oldestFixTime.UTC())

	hotspots := make(map[string]float64)

	for _, fix := range fixes {
		t := 1 - (currentTime.Sub(fix.Time).Seconds() / currentTime.Sub(oldestFixTime).Seconds())

		for _, change := range fix.Changes {
			// Ignore deleted files
			action, err := change.Action()
			examples.CheckIfError(err)
			if action == merkletrie.Delete {
				continue
			}

			// Get filename
			name := getChangeName(change)

			accumulation := 0.0
			if val, ok := hotspots[name]; ok {
				accumulation = val
			}
			hotspots[name] = accumulation + (1 / (1 + math.Exp((-12*t)+12)))
		}
	}

	var spots = make(SpotList, len(hotspots))
	i := 0
	for file, score := range hotspots {
		spots[i] = Spot{
			File:  file,
			Score: score,
		}
		i++
	}
	sort.Sort(sort.Reverse(spots))

	for i, spot := range spots {
		fmt.Printf("Score: %6f File: %s\n", spot.Score, spot.File)
		if i > 100 {
			break
		}
	}

	fmt.Printf("hotspot count: %d", len(spots))
}
