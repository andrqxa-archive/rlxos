package dl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Open — the main entry point for loading a shared object
// ──────────────────────────────────────────────────────────────────────────────

// Open loads a shared object and its dependencies, applies relocations,
// and runs constructors.  The returned Handle provides Sym/Close.
//
// Flags control binding and scope behavior:
//   - RTLD_NOW: resolve all symbols immediately (default if RTLD_LAZY not set)
//   - RTLD_LAZY: defer PLT binding (currently resolved eagerly for correctness)
//   - RTLD_GLOBAL: add symbols to the global scope
//   - RTLD_LOCAL: keep symbols in handle-local scope (default)
//   - RTLD_DEEPBIND: prefer object's own symbols
//   - RTLD_NOLOAD: return handle only if already loaded
//   - RTLD_NODELETE: prevent unloading
func (ld *Loader) Open(name string, flags Flags) (*Handle, error) {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	eager := flags&RTLD_NOW != 0 || flags&RTLD_LAZY == 0

	// Resolve the path.
	path, err := ld.findLibrary(name, nil)
	if err != nil {
		return nil, err
	}

	// RTLD_NOLOAD: return existing handle or error.
	if flags&RTLD_NOLOAD != 0 {
		obj, ok := ld.objects[path]
		if !ok {
			return nil, &LoadError{Path: name, Err: fmt.Errorf("not loaded (RTLD_NOLOAD)")}
		}
		obj.refcount++
		if flags&RTLD_GLOBAL != 0 && !obj.global {
			obj.global = true
			ld.global = append(ld.global, obj)
		}
		h := &Handle{
			loader: ld,
			obj:    obj,
			local:  ld.buildLocalScope(obj),
		}
		return h, nil
	}

	// Load the object and all its dependencies (breadth-first).
	loadOrder, err := ld.loadTree(path, flags)
	if err != nil {
		return nil, err
	}

	// Build the local scope for this handle (the primary object + all
	// transitive dependencies in load/BFS order).
	localScope := make([]*Object, len(loadOrder))
	copy(localScope, loadOrder)

	// Apply relocations in dependency order (dependencies first).
	for _, obj := range loadOrder {
		if err := ld.relocateObject(obj, localScope, eager); err != nil {
			return nil, err
		}
	}

	// Run constructors in dependency order (dependencies first).
	for _, obj := range loadOrder {
		obj.runInitFunctions()
	}

	primary := loadOrder[0]
	h := &Handle{
		loader: ld,
		obj:    primary,
		local:  localScope,
	}

	ld.logf("opened %s -> %s (flags=0x%x, %d objects)", name, path, flags, len(loadOrder))
	return h, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// loadTree — recursive dependency loading
// ──────────────────────────────────────────────────────────────────────────────

// loadTree loads the ELF at path and all its DT_NEEDED dependencies
// recursively (BFS).  Returns the load order (primary first, then deps).
// Already-loaded objects get refcount incremented but are not re-mapped.
func (ld *Loader) loadTree(path string, flags Flags) ([]*Object, error) {
	var order []*Object
	visited := make(map[string]bool)

	queue := []loadReq{{path: path, flags: flags, requester: nil}}

	for len(queue) > 0 {
		req := queue[0]
		queue = queue[1:]

		resolvedPath := req.path
		if !filepath.IsAbs(resolvedPath) {
			var err error
			resolvedPath, err = ld.findLibrary(req.path, req.requester)
			if err != nil {
				return nil, err
			}
		}

		if visited[resolvedPath] {
			continue
		}
		visited[resolvedPath] = true

		obj, err := ld.loadSingle(resolvedPath, req.flags)
		if err != nil {
			return nil, err
		}
		order = append(order, obj)

		// Enqueue DT_NEEDED dependencies.
		for _, needed := range obj.Needed {
			if !visited[needed] {
				depPath, err := ld.findLibrary(needed, obj)
				if err != nil {
					return nil, err
				}
				if !visited[depPath] {
					queue = append(queue, loadReq{path: depPath, flags: flags, requester: obj})
				}
			}
		}
	}

	// Reverse so dependencies come first (topological order for init).
	// Actually, BFS gives us primary-first.  For init, we want deps first.
	// Reverse the order: last loaded (deepest dep) initialized first.
	reversed := make([]*Object, len(order))
	for i, o := range order {
		reversed[len(order)-1-i] = o
	}
	// But the Handle.local scope should have primary first for symbol lookup.
	// We return primary-first order; the caller reverses for init.

	// Actually let's return load order (primary first) and let the caller
	// handle init order.  For init, iterate in reverse.

	// Re-reverse to get proper dependency-first init order:
	// loadOrder[0] = primary (loaded first, init'd last)
	// For relocation and init, we process deps first.
	// Let's return the order where deps come before the objects that need them.

	// BFS order is: [primary, dep1, dep2, dep1_dep, ...].  For init we need
	// leaves first.  Simply reverse the BFS order for init/reloc.
	initOrder := make([]*Object, len(order))
	for i := range order {
		initOrder[i] = order[len(order)-1-i]
	}

	// Return initOrder (deps first, primary last).  The Handle keeps this
	// order for scoped lookup and the primary pointer for Sym.
	return order, nil
}

type loadReq struct {
	path      string
	flags     Flags
	requester *Object // the object that issued DT_NEEDED
}

// loadSingle loads a single ELF object (no dependency resolution).
// If already loaded, increments refcount and returns the existing object.
func (ld *Loader) loadSingle(path string, flags Flags) (*Object, error) {
	// Check cache.
	if obj, ok := ld.objects[path]; ok {
		obj.refcount++
		if flags&RTLD_GLOBAL != 0 && !obj.global {
			obj.global = true
			ld.global = append(ld.global, obj)
		}
		if flags&RTLD_NODELETE != 0 {
			obj.nodelete = true
		}
		return obj, nil
	}

	ld.logf("loading %s", path)

	f, err := os.Open(path)
	if err != nil {
		return nil, &LoadError{Path: path, Err: err}
	}
	defer f.Close()

	// Read and validate ELF header.
	hdr, err := readELFHeader(f, path)
	if err != nil {
		return nil, err
	}

	// Read program headers.
	phdrs, err := readPhdrs(f, hdr, path)
	if err != nil {
		return nil, err
	}

	// Map PT_LOAD segments.
	obj, err := mapObject(f, phdrs, path)
	if err != nil {
		return nil, err
	}

	obj.loader = ld
	obj.refcount = 1

	// Apply flags.
	if flags&RTLD_DEEPBIND != 0 {
		obj.deepbind = true
	}
	if flags&RTLD_NODELETE != 0 {
		obj.nodelete = true
	}

	// Find and parse PT_DYNAMIC.
	var dynPhdr *elf64Phdr
	for i := range phdrs {
		if phdrs[i].Type == ptDYNAMIC {
			dynPhdr = &phdrs[i]
			break
		}
	}
	if dynPhdr == nil {
		obj.unmapObject()
		return nil, &LoadError{Path: path, Err: fmt.Errorf("no PT_DYNAMIC segment")}
	}

	entries, info, err := parseDynamic(obj.Base, *dynPhdr)
	if err != nil {
		obj.unmapObject()
		return nil, &LoadError{Path: path, Err: err}
	}
	obj.dynEntries = entries
	obj.dyn = *info

	// Rebase dynamic addresses to absolute.
	obj.rebaseDynamic()

	// Assign TLS module.
	ld.assignTLS(obj)

	// Register in cache.
	ld.objects[path] = obj

	// Also register by soname for dedup.
	if obj.Soname != "" && obj.Soname != path {
		if _, exists := ld.objects[obj.Soname]; !exists {
			ld.objects[obj.Soname] = obj
		}
	}

	// Add to global scope if requested.
	if flags&RTLD_GLOBAL != 0 {
		obj.global = true
		ld.global = append(ld.global, obj)
	}

	ld.logf("loaded %s at base 0x%x (soname=%q, %d needed)",
		path, obj.Base, obj.Soname, len(obj.Needed))

	return obj, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Library search
// ──────────────────────────────────────────────────────────────────────────────

// findLibrary resolves a library name (soname or path) to an absolute path.
// Search order:
//  1. If name contains '/', treat as direct path.
//  2. DT_RUNPATH of the requester (if present), with $ORIGIN expansion.
//  3. LD_LIBRARY_PATH (if enabled).
//  4. DT_RPATH of the requester (only if RUNPATH absent; legacy).
//  5. Default paths under root.
func (ld *Loader) findLibrary(name string, requester *Object) (string, error) {
	// Already resolved path?
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		// Try under root.
		rooted := filepath.Join(ld.root, name)
		if _, err := os.Stat(rooted); err == nil {
			return rooted, nil
		}
		return "", &LoadError{Path: name, Err: ErrNotFound}
	}

	// Direct path (contains slash).
	if strings.Contains(name, "/") {
		if _, err := os.Stat(name); err == nil {
			abs, _ := filepath.Abs(name)
			return abs, nil
		}
		return "", &LoadError{Path: name, Err: ErrNotFound}
	}

	// Already loaded by soname?
	if obj, ok := ld.objects[name]; ok {
		return obj.Path, nil
	}

	var searchDirs []string

	// RUNPATH from requester.
	if requester != nil && requester.Runpath != "" {
		for _, dir := range splitPath(requester.Runpath) {
			dir = expandOrigin(dir, requester.Path)
			searchDirs = append(searchDirs, ld.rootJoin(dir))
		}
	}

	// LD_LIBRARY_PATH.
	if ld.useLDPath {
		if ldPath := os.Getenv("LD_LIBRARY_PATH"); ldPath != "" {
			for _, dir := range splitPath(ldPath) {
				searchDirs = append(searchDirs, ld.rootJoin(dir))
			}
		}
	}

	// RPATH from requester (only if RUNPATH absent).
	if requester != nil && requester.Runpath == "" && requester.Rpath != "" {
		for _, dir := range splitPath(requester.Rpath) {
			dir = expandOrigin(dir, requester.Path)
			searchDirs = append(searchDirs, ld.rootJoin(dir))
		}
	}

	// Extra search paths.
	for _, dir := range ld.extraPaths {
		searchDirs = append(searchDirs, ld.rootJoin(dir))
	}

	// Default arch-specific dirs.
	for _, dir := range archDefaultLibDirs() {
		searchDirs = append(searchDirs, ld.rootJoin(dir))
	}

	// Search.
	for _, dir := range searchDirs {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			ld.logf("found %s -> %s", name, candidate)
			return candidate, nil
		}
	}

	return "", &LoadError{Path: name, Err: fmt.Errorf("%w: searched %d directories", ErrNotFound, len(searchDirs))}
}

// rootJoin joins the loader root with a path.  If the path is already
// absolute, it becomes root+path; otherwise root+"/"+path.
func (ld *Loader) rootJoin(path string) string {
	if ld.root == "/" || ld.root == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return filepath.Join(ld.root, path)
	}
	return filepath.Join(ld.root, path)
}

// splitPath splits a colon-separated path list.
func splitPath(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ":")
}

// expandOrigin replaces $ORIGIN (and ${ORIGIN}) in a search path with the
// directory containing the requesting object.
func expandOrigin(dir string, objPath string) string {
	if !strings.Contains(dir, "$ORIGIN") && !strings.Contains(dir, "${ORIGIN}") {
		return dir
	}
	origin := filepath.Dir(objPath)
	dir = strings.ReplaceAll(dir, "${ORIGIN}", origin)
	dir = strings.ReplaceAll(dir, "$ORIGIN", origin)
	return dir
}

// buildLocalScope constructs the local scope for an already-loaded object
// by walking its DT_NEEDED chain.
func (ld *Loader) buildLocalScope(obj *Object) []*Object {
	var scope []*Object
	visited := make(map[string]bool)

	var walk func(o *Object)
	walk = func(o *Object) {
		if visited[o.Path] {
			return
		}
		visited[o.Path] = true
		scope = append(scope, o)
		for _, needed := range o.Needed {
			if dep, ok := ld.objects[needed]; ok {
				walk(dep)
			}
		}
	}
	walk(obj)
	return scope
}

// ──────────────────────────────────────────────────────────────────────────────
// Close
// ──────────────────────────────────────────────────────────────────────────────

// closeHandle decrements refcounts and unloads objects that reach zero.
func (ld *Loader) closeHandle(h *Handle) error {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	// Decrement refcounts in reverse load order (dependents before deps).
	var toUnload []*Object
	for i := len(h.local) - 1; i >= 0; i-- {
		obj := h.local[i]
		obj.refcount--
		if obj.refcount <= 0 && !obj.nodelete {
			toUnload = append(toUnload, obj)
		}
	}

	// Run destructors in reverse dependency order and unmap.
	for _, obj := range toUnload {
		ld.logf("unloading %s", obj.Path)
		obj.runFiniFunctions()

		// Remove from global scope.
		if obj.global {
			for i, g := range ld.global {
				if g == obj {
					ld.global = append(ld.global[:i], ld.global[i+1:]...)
					break
				}
			}
		}

		// Remove from cache.
		delete(ld.objects, obj.Path)
		if obj.Soname != "" {
			if cached, ok := ld.objects[obj.Soname]; ok && cached == obj {
				delete(ld.objects, obj.Soname)
			}
		}

		// Unmap.
		obj.unmapObject()
	}

	return nil
}
