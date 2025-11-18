package proc

import (
	"container/list"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultMaxEntries = 4096

type Resolver struct {
	ttl        time.Duration
	maxEntries int

	mu    sync.Mutex
	cache map[uint32]*list.Element
	lru   *list.List
}

type entry struct {
	pid     uint32
	name    string
	expires time.Time
}

func NewResolver(ttl time.Duration) *Resolver {
	return &Resolver{
		ttl:        ttl,
		maxEntries: defaultMaxEntries,
		cache:      make(map[uint32]*list.Element),
		lru:        list.New(),
	}
}

func (r *Resolver) Lookup(pid uint32) string {
	if pid == 0 {
		return "swapper"
	}

	now := time.Now()

	r.mu.Lock()
	if elem, ok := r.cache[pid]; ok {
		ent := elem.Value.(*entry)
		if ent.expires.After(now) {
			r.lru.MoveToFront(elem)
			name := ent.name
			r.mu.Unlock()
			return name
		}
		r.removeElement(elem)
	}
	r.mu.Unlock()

	name := readComm(pid)

	r.mu.Lock()
	elem := r.lru.PushFront(&entry{
		pid:     pid,
		name:    name,
		expires: now.Add(r.ttl),
	})
	r.cache[pid] = elem
	r.evictLocked(now)
	r.mu.Unlock()

	return name
}

func (r *Resolver) removeElement(elem *list.Element) {
	if elem == nil {
		return
	}
	ent := elem.Value.(*entry)
	delete(r.cache, ent.pid)
	r.lru.Remove(elem)
}

func (r *Resolver) evictLocked(now time.Time) {
	for elem := r.lru.Back(); elem != nil; {
		ent := elem.Value.(*entry)
		if ent.expires.After(now) {
			break
		}
		prev := elem.Prev()
		r.removeElement(elem)
		elem = prev
	}

	for r.maxEntries > 0 && r.lru.Len() > r.maxEntries {
		if tail := r.lru.Back(); tail != nil {
			r.removeElement(tail)
		} else {
			break
		}
	}
}

func readComm(pid uint32) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}
