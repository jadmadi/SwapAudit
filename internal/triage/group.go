package triage

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

var (
	trailingDigits = regexp.MustCompile(`[0-9]{4,}$`)
	trailingHash   = regexp.MustCompile(`[-_][0-9a-fA-F]{8,}$`)
	trailingPID    = regexp.MustCompile(`[-.][0-9]{2,}$`)
)

// Group is a collapsed set of items that share a normalized name.
type Group struct {
	Key            string
	Category       model.Category
	CacheKind      model.CacheKind
	Count          int
	TotalBytes     int64
	MinIdle        time.Duration
	MaxIdle        time.Duration
	Representative model.SwapItem
	Members        []model.SwapItem
	Cleanable      bool
}

// NormalizeGroupKey strips trailing instance numbers, hashes, and PIDs.
func NormalizeGroupKey(name string) string {
	key := name
	stripped := false
	if loc := trailingHash.FindStringIndex(key); loc != nil && loc[1] == len(key) && loc[0] > 0 {
		key = key[:loc[0]]
		stripped = true
	} else if loc := trailingDigits.FindStringIndex(key); loc != nil && loc[1] == len(key) && loc[0] > 0 {
		key = key[:loc[0]]
		stripped = true
	} else if loc := trailingPID.FindStringIndex(key); loc != nil && loc[1] == len(key) && loc[0] > 0 {
		key = key[:loc[0]]
		stripped = true
	}
	key = strings.TrimRight(key, "-_.")
	if key == "" || !stripped {
		return name
	}
	return key + "*"
}

// GroupItems collapses repeated names. When enabled is false each item is
// returned as its own single-member group.
func GroupItems(items []model.SwapItem, enabled bool) []Group {
	if !enabled {
		groups := make([]Group, 0, len(items))
		for _, it := range items {
			groups = append(groups, groupOf(it))
		}
		sortGroups(groups)
		return groups
	}

	index := make(map[string]*Group)
	var order []string
	for _, it := range items {
		key := NormalizeGroupKey(it.Name)
		internal := string(it.Category) + "|" + string(it.CacheKind) + "|" + key
		g, ok := index[internal]
		if !ok {
			g = &Group{
				Key:       key,
				Category:  it.Category,
				CacheKind: it.CacheKind,
				Cleanable: true,
				MinIdle:   it.LastActive,
				MaxIdle:   it.LastActive,
			}
			index[internal] = g
			order = append(order, internal)
		}
		g.Count++
		g.TotalBytes += model.ItemSize(it)
		g.Members = append(g.Members, it)
		if !it.Cleanable {
			g.Cleanable = false
		}
		if it.LastActiveKnown {
			if g.MinIdle <= 0 || it.LastActive < g.MinIdle {
				g.MinIdle = it.LastActive
			}
			if it.LastActive > g.MaxIdle {
				g.MaxIdle = it.LastActive
			}
		}
		if model.ItemSize(it) > model.ItemSize(g.Representative) {
			g.Representative = it
		}
	}
	groups := make([]Group, 0, len(order))
	for _, internal := range order {
		g := index[internal]
		if g.Representative.Name == "" && len(g.Members) > 0 {
			g.Representative = g.Members[0]
		}
		groups = append(groups, *g)
	}
	sortGroups(groups)
	return groups
}

// ExpandGroup returns every member of a group.
func ExpandGroup(g Group) []model.SwapItem {
	if len(g.Members) > 0 {
		return append([]model.SwapItem(nil), g.Members...)
	}
	return []model.SwapItem{g.Representative}
}

// ExpandMatching expands groups whose key matches the requested prefix.
func ExpandMatching(groups []Group, key string) []Group {
	if key == "" {
		return groups
	}
	var out []Group
	for _, g := range groups {
		if g.Count > 1 && (g.Key == key || strings.HasPrefix(g.Key, key)) {
			for _, member := range ExpandGroup(g) {
				out = append(out, groupOf(member))
			}
			continue
		}
		out = append(out, g)
	}
	sortGroups(out)
	return out
}

func groupOf(it model.SwapItem) Group {
	return Group{
		Key:            it.Name,
		Category:       it.Category,
		CacheKind:      it.CacheKind,
		Count:          1,
		TotalBytes:     model.ItemSize(it),
		MinIdle:        it.LastActive,
		MaxIdle:        it.LastActive,
		Representative: it,
		Members:        []model.SwapItem{it},
		Cleanable:      it.Cleanable,
	}
}

func sortGroups(groups []Group) {
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].TotalBytes != groups[j].TotalBytes {
			return groups[i].TotalBytes > groups[j].TotalBytes
		}
		return groups[i].Key < groups[j].Key
	})
}
