// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mvs implements Minimal Version Selection.
// See https://research.swtch.com/vgo-mvs.
package mvs

import (
	"fmt"
	"sort"
	"sync"

	"cmd/go/internal/module"
	"cmd/go/internal/par"
)

// A Reqs is the requirement graph on which Minimal Version Selection (MVS) operates.
//
// The version strings are opaque except for the special versions "" and "none"
// (see the documentation for module.Version). In particular, MVS does not
// assume that the version strings are semantic versions; instead, the Max method
// gives access to the comparison operation.
//
// It must be safe to call methods on a Reqs from multiple goroutines simultaneously.
// Because a Reqs may read the underlying graph from the network on demand,
// the MVS algorithms parallelize the traversal to overlap network delays.
type Reqs interface {
	// Required returns the module versions explicitly required by m itself.
	// The caller must not modify the returned list.
	Required(m module.Version) ([]module.Version, error)

	// Max returns the maximum of v1 and v2 (it returns either v1 or v2).
	// For all versions v, Max(v, "none") must be v.
	// TODO(rsc,bcmills): For all versions v, Max(v, "") must be "" ? Maybe.
	//
	// Note that v1 < v2 can be written Max(v1, v2) != v1
	// and similarly v1 <= v2 can be written Max(v1, v2) == v2.
	Max(v1, v2 string) string

	// Latest returns the latest known version of the module at path
	// (the one to use during UpgradeAll).
	//
	// Latest never returns version "none": if no module exists at the given path,
	// it returns a non-nil error instead.
	//
	// TODO(bcmills): If path is the current module, must Latest return version
	// "", or the most recent prior version?
	Latest(path string) (module.Version, error)

	// Previous returns the version of m.Path immediately prior to m.Version,
	// or "none" if no such version is known.
	Previous(m module.Version) (module.Version, error)
}

type MissingModuleError struct {
	Module module.Version
}

func (e *MissingModuleError) Error() string {
	return fmt.Sprintf("missing module: %v", e.Module)
}

// BuildList returns the build list for the target module.
func BuildList(target module.Version, reqs Reqs) ([]module.Version, error) {
	return buildList(target, reqs, nil, nil)
}

func buildList(target module.Version, reqs Reqs, uses map[module.Version][]module.Version, vers map[string]string) ([]module.Version, error) {
	// Explore work graph in parallel in case reqs.Required
	// does high-latency network operations.
	var work par.Work
	work.Add(target)
	var (
		mu  sync.Mutex
		min = map[string]string{target.Path: target.Version}
	)
	work.Do(10, func(item interface{}) {
		m := item.(module.Version)
		required, _ := reqs.Required(m)

		for _, r := range required {
			work.Add(r)
		}

		mu.Lock()
		defer mu.Unlock()
		for _, r := range required {
			if uses != nil {
				uses[r] = append(uses[r], m)
			}
			if v, ok := min[r.Path]; !ok {
				min[r.Path] = r.Version
			} else if max := reqs.Max(v, r.Version); max != v {
				min[r.Path] = max
			}
		}
	})

	if min[target.Path] != target.Version {
		panic("unbuildable") // TODO
	}

	if vers == nil {
		vers = make(map[string]string)
	}
	list := []module.Version{target}
	for i := 0; i < len(list); i++ {
		m := list[i]
		required, err := reqs.Required(m)
		if err != nil {
			// TODO: Check error is decent.
			return nil, err
		}
		for _, r := range required {
			v := min[r.Path]
			if reqs.Max(v, r.Version) != v {
				panic("mistake") // TODO
			}
			if _, ok := vers[r.Path]; !ok {
				vers[r.Path] = v
				list = append(list, module.Version{Path: r.Path, Version: v})
			}
		}
	}
	tail := list[1:]
	sort.Slice(tail, func(i, j int) bool {
		return tail[i].Path < tail[j].Path
	})
	return list, nil
}

// Req returns the minimal requirement list for the target module
// that results in the given build list.
func Req(target module.Version, list []module.Version, reqs Reqs) ([]module.Version, error) {
	// Note: Not running in parallel because we assume
	// that list came from a previous operation that paged
	// in all the requirements, so there's no I/O to overlap now.

	// Compute postorder, cache requirements.
	var postorder []module.Version
	reqCache := map[module.Version][]module.Version{}
	reqCache[target] = nil
	var walk func(module.Version) error
	walk = func(m module.Version) error {
		_, ok := reqCache[m]
		if ok {
			return nil
		}
		required, err := reqs.Required(m)
		if err != nil {
			return err
		}
		reqCache[m] = required
		for _, m1 := range required {
			if err := walk(m1); err != nil {
				return err
			}
		}
		postorder = append(postorder, m)
		return nil
	}
	for _, m := range list {
		if err := walk(m); err != nil {
			return nil, err
		}
	}

	// Walk modules in reverse post-order, only adding those not implied already.
	have := map[string]string{}
	walk = func(m module.Version) error {
		if v, ok := have[m.Path]; ok && reqs.Max(m.Version, v) == v {
			return nil
		}
		have[m.Path] = m.Version
		for _, m1 := range reqCache[m] {
			walk(m1)
		}
		return nil
	}
	max := map[string]string{}
	for _, m := range list {
		if max[m.Path] == "" {
			max[m.Path] = m.Version
		} else {
			max[m.Path] = reqs.Max(m.Version, max[m.Path])
		}
	}
	var min []module.Version
	for i := len(postorder) - 1; i >= 0; i-- {
		m := postorder[i]
		if max[m.Path] != m.Version {
			// Older version.
			continue
		}
		if have[m.Path] != m.Version {
			min = append(min, m)
			walk(m)
		}
	}
	sort.Slice(min, func(i, j int) bool {
		return min[i].Path < min[j].Path
	})
	return min, nil
}

// UpgradeAll returns a build list for the target module
// in which every module is upgraded to its latest version.
func UpgradeAll(target module.Version, reqs Reqs) ([]module.Version, error) {
	// Explore work graph in parallel, like in buildList,
	// but here the work item is only a path, not a path+version pair,
	// because we always take the latest of any path.
	var work par.Work
	work.Add(target.Path)
	var (
		mu   sync.Mutex
		list []module.Version
		min  = map[string]string{target.Path: ""}
	)
	work.Do(10, func(item interface{}) {
		path := item.(string)
		m := module.Version{Path: path}
		if path != target.Path {
			latest, err := reqs.Latest(path)
			if err != nil {
				panic(err) // TODO
			}
			m.Version = latest.Version
		}

		required, err := reqs.Required(m)
		if err != nil {
			panic("TODO")
		}

		mu.Lock()
		// Important: must append to list before calling work.Add (below).
		// We expect the first work item (target) to be first in list.
		list = append(list, m)
		for _, r := range required {
			if v, ok := min[r.Path]; !ok {
				min[r.Path] = r.Version
			} else {
				min[r.Path] = reqs.Max(v, r.Version)
			}
		}
		mu.Unlock()

		for _, r := range required {
			work.Add(r.Path)
		}
	})

	for _, m := range list {
		if reqs.Max(m.Version, min[m.Path]) != m.Version {
			panic("mistake") // TODO
		}
	}

	tail := list[1:]
	sort.Slice(tail, func(i, j int) bool {
		return tail[i].Path < tail[j].Path
	})
	return list, nil
}

// Upgrade returns a build list for the target module
// in which the given additional modules are upgraded.
func Upgrade(target module.Version, reqs Reqs, upgrade ...module.Version) ([]module.Version, error) {
	list, err := reqs.Required(target)
	if err != nil {
		panic(err) // TODO
	}
	// TODO: Maybe if an error is given,
	// rerun with BuildList(upgrade[0], reqs) etc
	// to find which ones are the buggy ones.
	list = append([]module.Version(nil), list...)
	list = append(list, upgrade...)
	return BuildList(target, &override{target, list, reqs})
}

// Downgrade returns a build list for the target module
// in which the given additional modules are downgraded.
func Downgrade(target module.Version, reqs Reqs, downgrade ...module.Version) ([]module.Version, error) {
	list, err := reqs.Required(target)
	if err != nil {
		panic(err) // TODO
	}
	max := make(map[string]string)
	for _, r := range list {
		max[r.Path] = r.Version
	}
	for _, d := range downgrade {
		if v, ok := max[d.Path]; !ok || reqs.Max(v, d.Version) != d.Version {
			max[d.Path] = d.Version
		}
	}

	var (
		added    = make(map[module.Version]bool)
		rdeps    = make(map[module.Version][]module.Version)
		excluded = make(map[module.Version]bool)
	)
	var exclude func(module.Version)
	exclude = func(m module.Version) {
		if excluded[m] {
			return
		}
		excluded[m] = true
		for _, p := range rdeps[m] {
			exclude(p)
		}
	}
	var add func(module.Version)
	add = func(m module.Version) {
		if added[m] {
			return
		}
		added[m] = true
		if v, ok := max[m.Path]; ok && reqs.Max(m.Version, v) != v {
			exclude(m)
			return
		}
		list, err := reqs.Required(m)
		if err != nil {
			panic(err) // TODO
		}
		for _, r := range list {
			add(r)
			if excluded[r] {
				exclude(m)
				return
			}
			rdeps[r] = append(rdeps[r], m)
		}
	}

	var out []module.Version
	out = append(out, target)
List:
	for _, r := range list {
		add(r)
		for excluded[r] {
			p, err := reqs.Previous(r)
			if err != nil {
				return nil, err // TODO
			}
			// If the target version is a pseudo-version, it may not be
			// included when iterating over prior versions using reqs.Previous.
			// Insert it into the right place in the iteration.
			// If v is excluded, p should be returned again by reqs.Previous on the next iteration.
			if v := max[r.Path]; reqs.Max(v, r.Version) != v && reqs.Max(p.Version, v) != p.Version {
				p.Version = v
			}
			if p.Version == "none" {
				continue List
			}
			add(p)
			r = p
		}
		out = append(out, r)
	}

	return out, nil
}

type override struct {
	target module.Version
	list   []module.Version
	Reqs
}

func (r *override) Required(m module.Version) ([]module.Version, error) {
	if m == r.target {
		return r.list, nil
	}
	return r.Reqs.Required(m)
}
