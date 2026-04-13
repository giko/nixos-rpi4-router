package collector

import (
	"container/list"
	"net/netip"
	"sync"
	"time"
)

type EnricherOpts struct {
	PerClientCap int
	GlobalCap    int           // optional; 0 = unlimited
	Grace        time.Duration // default 30 min
}

type enrichEntry struct {
	client    netip.Addr
	remote    netip.Addr
	domain    string
	expiresAt time.Time
	elem      *list.Element
}

type clientCache struct {
	byIP map[netip.Addr]*enrichEntry
	lru  *list.List // front = newest
}

type DomainEnricher struct {
	mu       sync.RWMutex
	opts     EnricherOpts
	clients  map[netip.Addr]*clientCache
	globalSz int
}

func NewDomainEnricher(opts EnricherOpts) *DomainEnricher {
	if opts.PerClientCap == 0 {
		opts.PerClientCap = 2000
	}
	if opts.Grace == 0 {
		opts.Grace = 30 * time.Minute
	}
	return &DomainEnricher{opts: opts, clients: make(map[netip.Addr]*clientCache)}
}

func (e *DomainEnricher) Record(client, remote netip.Addr, domain string, ttl time.Duration, now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	cc, ok := e.clients[client]
	if !ok {
		cc = &clientCache{byIP: make(map[netip.Addr]*enrichEntry), lru: list.New()}
		e.clients[client] = cc
	}
	if existing, found := cc.byIP[remote]; found {
		existing.domain = domain
		existing.expiresAt = now.Add(ttl).Add(e.opts.Grace)
		cc.lru.MoveToFront(existing.elem)
		return
	}
	entry := &enrichEntry{client: client, remote: remote, domain: domain,
		expiresAt: now.Add(ttl).Add(e.opts.Grace)}
	entry.elem = cc.lru.PushFront(entry)
	cc.byIP[remote] = entry
	e.globalSz++

	for len(cc.byIP) > e.opts.PerClientCap {
		back := cc.lru.Back()
		if back == nil {
			break
		}
		victim := back.Value.(*enrichEntry)
		cc.lru.Remove(back)
		delete(cc.byIP, victim.remote)
		e.globalSz--
	}
	if e.opts.GlobalCap > 0 {
		e.evictGlobal()
	}
}

func (e *DomainEnricher) evictGlobal() {
	for e.globalSz > e.opts.GlobalCap {
		for _, cc := range e.clients {
			back := cc.lru.Back()
			if back == nil {
				continue
			}
			victim := back.Value.(*enrichEntry)
			cc.lru.Remove(back)
			delete(cc.byIP, victim.remote)
			e.globalSz--
			if e.globalSz <= e.opts.GlobalCap {
				return
			}
		}
	}
}

func (e *DomainEnricher) Lookup(client, remote netip.Addr, now time.Time) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cc, ok := e.clients[client]
	if !ok {
		return "", false
	}
	entry, ok := cc.byIP[remote]
	if !ok {
		return "", false
	}
	if now.After(entry.expiresAt) {
		return "", false
	}
	return entry.domain, true
}

// Drop removes all per-client state (used when the lifecycle tracker
// tombstones or reaps a client).
func (e *DomainEnricher) Drop(client netip.Addr) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cc, ok := e.clients[client]
	if !ok {
		return
	}
	e.globalSz -= len(cc.byIP)
	delete(e.clients, client)
}
