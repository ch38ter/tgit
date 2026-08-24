package git

import (
	"strings"
)

// Ref is one branch or remote-tracking ref from `git for-each-ref`.
type Ref struct {
	Name string // short refname, e.g. "main" or "origin/main"
	Head bool   // true when HEAD points at this ref (%(HEAD) == "*")
}

// refsFormat is the NUL-separated field format:
// short refname \0 HEAD marker ("*" or " ").
const refsFormat = "--format=%(refname:short)%00%(HEAD)"

// ListRefs returns local branches (refs/heads) followed by remote-tracking
// refs (refs/remotes), alphabetical within each group — for-each-ref's
// refname sort order. Head marks the branch HEAD points at; remote-tracking
// refs never have it set.
//
// Silent-degradation contract: a repository without any refs (e.g. freshly
// initialized) yields an empty slice and nil error.
func ListRefs() ([]Ref, error) {
	out, err := RunGit("for-each-ref", refsFormat, "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}

	refs := []Ref{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\x00")
		if len(fields) < 2 {
			continue
		}
		refs = append(refs, Ref{Name: fields[0], Head: fields[1] == "*"})
	}
	return refs, nil
}
