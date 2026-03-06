package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const majorVersion = "v2"

func main() {
	target := "HEAD"
	if len(os.Args) > 1 && os.Args[1] != "" {
		target = os.Args[1]
	}

	fullSHA := mustGit("rev-parse", target)

	exactTags := gitLines("tag", "--points-at", fullSHA, "--list", majorVersion+".*")
	if exactTag := highestTag(exactTags); exactTag != "" {
		fmt.Println(exactTag)
		return
	}

	baseVersion := highestTag(gitLines("tag", "--merged", fullSHA, "--list", majorVersion+".*"))

	commitTimeText := mustGit("show", "-s", "--format=%cI", fullSHA)
	commitTime, err := time.Parse(time.RFC3339, commitTimeText)
	if err != nil {
		fail("parse commit time %q: %v", commitTimeText, err)
	}

	shortSHA := mustGit("rev-parse", "--short=12", fullSHA)
	fmt.Println(module.PseudoVersion(majorVersion, baseVersion, commitTime.UTC(), shortSHA))
}

func gitLines(args ...string) []string {
	out, err := gitOutput(args...)
	if err != nil {
		return nil
	}

	return strings.Fields(out)
}

func mustGit(args ...string) string {
	out, err := gitOutput(args...)
	if err != nil {
		fail("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func highestTag(tags []string) string {
	highest := ""
	for _, tag := range tags {
		if !strings.HasPrefix(tag, majorVersion+".") {
			continue
		}
		if !semver.IsValid(tag) {
			continue
		}
		if semver.Compare(tag, highest) > 0 {
			highest = tag
		}
	}
	return highest
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
