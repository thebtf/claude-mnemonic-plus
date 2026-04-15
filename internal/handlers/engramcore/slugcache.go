package engramcore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/thebtf/engram/internal/proxy"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// slugCache is the project-ID resolution cache. Keyed by
// muxcore.ProjectContext.ID so repeated requests for the same session skip
// the git call-out inside proxy.ResolveProjectSlug. Ported verbatim from
// engramHandler v4.2.0.
//
// Thread-safety: sync.Map — Resolve is called from muxcore dispatch
// goroutines and may race with OnProjectRemoved clearing entries.
type slugCache struct {
	entries sync.Map // ProjectContext.ID → resolvedSlug
}

// resolvedSlug is the cached value — project slug + whether logging has
// already announced this project. The announce flag prevents spamming the
// stderr log once per subsequent request on the same ID.
type resolvedSlug struct {
	id       string
	announce bool
}

// Resolve returns the engram project slug for the given session. On first
// lookup it calls proxy.ResolveProjectSlug and logs the result once. Later
// lookups for the same ProjectContext.ID return the cached value with no
// I/O.
//
// On error it falls back to the muxcore-provided ID (which is already
// git-hash-derived inside muxcore's session layer) so the daemon never
// fails to respond due to a git lookup hiccup.
func (c *slugCache) Resolve(p muxcore.ProjectContext) string {
	if cached, ok := c.entries.Load(p.ID); ok {
		return cached.(resolvedSlug).id
	}

	id, displayName, remote, err := proxy.ResolveProjectSlug(p.Cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[engram] warning: project identity failed for %s: %v\n", p.Cwd, err)
		id = p.ID
		displayName = filepath.Base(p.Cwd)
	}
	if remote != "" {
		fmt.Fprintf(os.Stderr, "[engram] project: %s (%s, remote: %s)\n", displayName, id, safeRemoteURL(remote))
	} else {
		fmt.Fprintf(os.Stderr, "[engram] project: %s (%s)\n", displayName, id)
	}

	c.entries.Store(p.ID, resolvedSlug{id: id, announce: true})
	return id
}

// Forget removes the cached entry for a project ID. Called from
// Module.OnProjectRemoved so a subsequent session on the same project
// does not reuse stale identity metadata.
func (c *slugCache) Forget(projectID string) {
	c.entries.Delete(projectID)
}

// ForceCacheEntry injects a synthetic entry into the slug cache keyed by
// projectID. The injected slug is used as the resolved project identity for
// subsequent ProxyTools / ProxyHandleTool calls, bypassing the git call-out
// inside proxy.ResolveProjectSlug.
//
// test helper — see contract_test.go
func (c *slugCache) ForceCacheEntry(projectID, slug string) {
	c.entries.Store(projectID, resolvedSlug{id: slug, announce: true})
}

// HasEntry reports whether the cache contains an entry for projectID. Used by
// unit tests to assert cache state after OnProjectRemoved.
//
// test helper — see module_test.go
func (c *slugCache) HasEntry(projectID string) bool {
	_, ok := c.entries.Load(projectID)
	return ok
}
