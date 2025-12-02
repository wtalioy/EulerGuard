package rules

import (
	"slices"
	"sort"
	"strings"

	"eulerguard/pkg/types"
	"eulerguard/pkg/utils"
)

type fileEvent struct {
	filename       string
	pathVariants   []string
	pid            uint32
	cgroupID       uint64
	matchedByInode bool
}

func (e fileEvent) hasExactPath(target string) bool {
	if target == "" {
		return false
	}
	return slices.Contains(e.pathVariants, target)
}

type pathPrefixBucket struct {
	prefix string
	rules  []*types.Rule
}

type fileMatcher struct {
	inodeRules map[types.InodeKey][]*types.Rule
	pathRules  map[string][]*types.Rule
	prefixes   []pathPrefixBucket
}

func newFileMatcher(rules []types.Rule) *fileMatcher {
	matcher := &fileMatcher{
		inodeRules: make(map[types.InodeKey][]*types.Rule),
		pathRules:  make(map[string][]*types.Rule),
		prefixes:   make([]pathPrefixBucket, 0),
	}

	prefixIndex := make(map[string]int)

	for i := range rules {
		rule := &rules[i]

		if key, ok := rule.Match.InodeKey(); ok {
			matcher.inodeRules[key] = append(matcher.inodeRules[key], rule)
		}

		if keys := rule.Match.ExactPathKeys(); len(keys) > 0 {
			for _, key := range keys {
				if key == "" {
					continue
				}
				matcher.pathRules[key] = append(matcher.pathRules[key], rule)
			}
		}

		if prefixes := rule.Match.PrefixPathKeys(); len(prefixes) > 0 {
			for _, prefix := range prefixes {
				if prefix == "" {
					continue
				}
				if idx, ok := prefixIndex[prefix]; ok {
					matcher.prefixes[idx].rules = append(matcher.prefixes[idx].rules, rule)
				} else {
					prefixIndex[prefix] = len(matcher.prefixes)
					matcher.prefixes = append(matcher.prefixes, pathPrefixBucket{
						prefix: prefix,
						rules:  []*types.Rule{rule},
					})
				}
			}
		}
	}

	sort.SliceStable(matcher.prefixes, func(i, j int) bool {
		return len(matcher.prefixes[i].prefix) > len(matcher.prefixes[j].prefix)
	})

	return matcher
}

func (m *fileMatcher) Match(ino, dev uint64, filename string, pid uint32, cgroupID uint64) (matched bool, rule *types.Rule, allowed bool) {
	if m == nil {
		return false, nil, false
	}

	variants := utils.PathVariants(filename)
	if len(variants) == 0 && filename != "" {
		if normalized := utils.NormalizeFilename(filename); normalized != "" {
			variants = append(variants, normalized)
		}
	}

	event := fileEvent{
		filename:     filename,
		pathVariants: variants,
		pid:          pid,
		cgroupID:     cgroupID,
	}

	if rules := m.inodeRules[types.InodeKey{Ino: ino, Dev: dev}]; len(rules) > 0 {
		inodeEvent := event
		inodeEvent.matchedByInode = true
		if matched, rule, allowed := filterRulesByAction(rules, m.matchRule, inodeEvent); matched {
			return matched, rule, allowed
		}
	}

	for _, key := range event.pathVariants {
		if key == "" {
			continue
		}
		if rules := m.pathRules[key]; len(rules) > 0 {
			if matched, rule, allowed := filterRulesByAction(rules, m.matchRule, event); matched {
				return matched, rule, allowed
			}
		}
	}

	for _, bucket := range m.prefixes {
		for _, variant := range event.pathVariants {
			if variant == "" {
				continue
			}
			if strings.HasPrefix(variant, bucket.prefix) {
				if matched, rule, allowed := filterRulesByAction(bucket.rules, m.matchRule, event); matched {
					return matched, rule, allowed
				}
				break
			}
		}
	}

	return false, nil, false
}

func (m *fileMatcher) matchRule(rule *types.Rule, event fileEvent) bool {
	match := rule.Match
	if match.Filename == "" && len(match.PrefixPathKeys()) == 0 {
		return false
	}

	if len(match.ExactPathKeys()) > 0 && !event.matchedByInode {
		found := slices.ContainsFunc(match.ExactPathKeys(), event.hasExactPath)
		if !found {
			return false
		}
	}

	if len(match.PrefixPathKeys()) > 0 {
		found := false
		for _, prefix := range match.PrefixPathKeys() {
			if prefix == "" {
				continue
			}
			for _, variant := range event.pathVariants {
				if variant == "" {
					continue
				}
				if strings.HasPrefix(ensureTrailingSlash(variant), prefix) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return matchCgroupID(match.CgroupID, event.cgroupID) && matchPID(match.PID, event.pid)
}

func ensureTrailingSlash(path string) string {
	if path == "" || path == "/" || strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}
