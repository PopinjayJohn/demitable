// Command spikehost runs the plugin host spike.
//
// It is the smallest thing that can be true: a manifest, a capability model,
// a surface host, a bridge, two plugins and a shell to look at them in.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"

	pluginhost "github.com/PopinjayJohn/demitable/spikes/plugin-host"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/bridge"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/caps"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/manifest"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/plugins/export"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/plugins/recap"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/vault"
)

type server struct {
	mu sync.Mutex

	store *vault.Store
	bus   *bridge.Bus

	instances map[string]*bridge.Instance // by role key
	manifests map[string]*manifest.Manifest

	webFS   fs.FS
	assetFS map[string]fs.FS // by plugin id
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8777", "listen address")
	flag.Parse()

	s := &server{
		store:     vault.Demo(),
		bus:       bridge.NewBus(),
		instances: map[string]*bridge.Instance{},
		manifests: map[string]*manifest.Manifest{},
		webFS:     pluginhost.Web(),
		assetFS:   map[string]fs.FS{},
	}

	if err := s.loadPlugins(); err != nil {
		log.Fatalf("loading plugins: %v", err)
	}
	s.makeInstances()

	log.Printf("plugin host spike on http://%s  (try ?role=gm and ?role=account)", *addr)
	if err := http.ListenAndServe(*addr, s.routes()); err != nil {
		log.Fatal(err)
	}
}

func (s *server) loadPlugins() error {
	// Plugin A: a UI plugin whose manifest is a YAML file next to the code.
	recapManifest, err := recap.Manifest()
	if err != nil {
		return fmt.Errorf("recap: %w", err)
	}
	s.manifests[recapManifest.ID] = recapManifest
	s.assetFS[recapManifest.ID] = recap.UI()

	// Plugin B: a compiled-in plugin whose manifest is a Go literal.
	exportManifest := export.Manifest()
	if errs := manifest.Validate(exportManifest); len(errs) > 0 {
		return fmt.Errorf("export: invalid manifest: %w", manifest.Errors(errs))
	}
	s.manifests[exportManifest.ID] = exportManifest

	return nil
}

// makeInstances builds one instance per role. ADR-0019: role-scoped
// instances, never shared state.
func (s *server) makeInstances() {
	principals := map[string]vault.Principal{
		"gm":      vault.GM(),
		"account": vault.Account("p_aria", "stormdark"),
	}
	for key, p := range principals {
		reg := bridge.NewRegistry()
		inst := bridge.NewInstance(s.manifests["demitable.recap"], p, s.store, s.bus, reg, log.Default())
		s.instances[key] = inst

		// The export plugin is a server-side plugin. It registers its
		// command once, in the GM's instance only.
		if key == "gm" {
			exportSvc := &export.Service{Store: s.store}
			inst2 := inst
			if err := reg.Register(&bridge.Command{
				ID:          "export.campaign",
				Title:       "Export: campaign as HTML",
				Description: "Renders every note the calling role may see.",
				Source:      "server",
				Plugin:      "demitable.export",
				Run: func(args json.RawMessage) (any, error) {
					title := inst2.Config()["title"].(string)
					return map[string]any{
						"bytes": len(exportSvc.Export(inst2.Principal, title)),
						"title": title,
					}, nil
				},
			}); err != nil {
				log.Fatal(err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// routing
// ---------------------------------------------------------------------------

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/shell/", s.handleShell)
	mux.HandleFunc("/plugin-assets/", s.handlePluginAssets)
	mux.HandleFunc("/api/bootstrap", s.handleBootstrap)
	mux.HandleFunc("/api/ready", s.handleReady)
	mux.HandleFunc("/api/call", s.handleCall)
	mux.HandleFunc("/api/host", s.handleHost)
	mux.HandleFunc("/api/emit", s.handleEmit)
	mux.HandleFunc("/api/context", s.handleContext)
	mux.HandleFunc("/api/notes", s.handleNotes)
	mux.HandleFunc("/api/commands", s.handleCommands)
	mux.HandleFunc("/api/commands/invoke", s.handleInvoke)
	mux.HandleFunc("/api/touch", s.handleTouch)
	mux.HandleFunc("/api/events", s.handleEvents)
	return mux
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "gm"
	}
	http.SetCookie(w, &http.Cookie{Name: "spike-role", Value: role, Path: "/"})
	data, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("content-type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *server) handleShell(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/shell/")
	if name == "" {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(s.webFS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch path.Ext(name) {
	case ".js":
		w.Header().Set("content-type", "text/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("content-type", "text/css; charset=utf-8")
	}
	w.Write(data)
}

func (s *server) handlePluginAssets(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/plugin-assets/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	pluginID, name := parts[0], parts[1]
	fsys, ok := s.assetFS[pluginID]
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch path.Ext(name) {
	case ".js":
		w.Header().Set("content-type", "text/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("content-type", "text/css; charset=utf-8")
	}
	w.Write(data)
}

// ---------------------------------------------------------------------------
// the API
// ---------------------------------------------------------------------------

func (s *server) instance(r *http.Request) *bridge.Instance {
	c, err := r.Cookie("spike-role")
	role := "gm"
	if err == nil && c.Value != "" {
		role = c.Value
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[role]
	if !ok {
		inst = s.instances["gm"]
	}
	return inst
}

type surfaceJSON struct {
	ID       string `json:"id"`
	PluginID string `json:"pluginId"`
	DOM      string `json:"dom"`
	Title    string `json:"title"`
	Mount    string `json:"mount"`
	Entry    string `json:"entry"`
	Style    string `json:"style,omitempty"`
	Element  string `json:"element,omitempty"`
	When     string `json:"when,omitempty"`
	Order    int    `json:"order"`
}

func (s *server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	m := s.manifests[inst.PluginID]
	var surfaces []surfaceJSON
	for _, p := range m.Surfaces() {
		surfaces = append(surfaces, surfaceJSON{
			ID:       p.ID,
			PluginID: m.ID,
			DOM:      p.DOMKind(),
			Title:    p.Title,
			Mount:    p.Mount,
			Entry:    "/plugin-assets/" + m.ID + "/" + p.Entry,
			Style:    styleURL(m.ID, p),
			Element:  p.Element,
			When:     p.When,
			Order:    p.Order,
		})
	}
	sort.Slice(surfaces, func(i, j int) bool { return surfaces[i].Order < surfaces[j].Order })
	writeJSON(w, map[string]any{
		"pluginId":  m.ID,
		"role":      inst.Principal.Kind,
		"principal": inst.Principal,
		"surfaces":  surfaces,
	})
}

func styleURL(pluginID string, p manifest.Provides) string {
	if p.Style == "" {
		return ""
	}
	return "/plugin-assets/" + pluginID + "/" + p.Style
}

func (s *server) handleReady(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	writeJSON(w, inst.Ready())
}

func (s *server) handleCall(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	var call bridge.Call
	if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
		writeJSON(w, bridge.Result{OK: false, Error: &bridge.Envelope{
			Code: caps.CodeInvalid, Message: err.Error(),
		}})
		return
	}
	writeJSON(w, inst.Dispatch(call))
}

func (s *server) handleHost(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	var call bridge.Call
	if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
		writeJSON(w, bridge.Result{OK: false, Error: &bridge.Envelope{
			Code: caps.CodeInvalid, Message: err.Error(),
		}})
		return
	}
	writeJSON(w, inst.DispatchHost(call.Op, call.Args, call.ClientID))
}

func (s *server) handleEmit(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	var body struct {
		Name    string `json:"name"`
		Payload any    `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	inst.Emit(body.Name, body.Payload)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *server) handleContext(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	var body struct {
		NoteUID string `json:"noteUid"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	ctx := bridge.Context{}
	title := ""
	if body.NoteUID != "" {
		if n, err := s.store.Get(inst.Principal, body.NoteUID); err == nil {
			ctx = bridge.Context{
				NoteUID:  n.UID,
				NoteType: n.Type,
				Campaign: n.Campaign,
				Path:     n.Path,
			}
			title = n.Title
		}
	}
	inst.SetContext(ctx)
	writeJSON(w, map[string]any{"context": ctx, "title": title})
}

func (s *server) handleNotes(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	notes := s.store.List(inst.Principal, vault.Query{})
	writeJSON(w, map[string]any{"notes": notes})
}

func (s *server) handleCommands(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	writeJSON(w, map[string]any{"commands": inst.Commands.List()})
}

func (s *server) handleInvoke(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	var body struct {
		ID   string          `json:"id"`
		Args json.RawMessage `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	res, err := inst.Invoke(body.ID, body.Args)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "result": res})
}

func (s *server) handleTouch(w http.ResponseWriter, r *http.Request) {
	inst := s.instance(r)
	notes := s.store.List(inst.Principal, vault.Query{})
	if len(notes) > 0 {
		n := notes[0]
		n.Body = n.Body + " (touched)"
		s.store.Replace(n)
	}
	s.bus.Publish(bridge.Event{
		Channel: "core",
		Name:    "vault.changed",
		Payload: map[string]any{"uid": "n-session-1", "kind": "updated"},
	})
	writeJSON(w, map[string]any{"ok": true})
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flusher", 500)
		return
	}
	inst := s.instance(r)
	w.Header().Set("content-type", "text/event-stream")
	w.Header().Set("cache-control", "no-cache")
	ch, cancel := s.bus.Subscribe()
	defer cancel()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			// An event that implies a capability is only delivered to an
			// instance that holds it.
			if need, gated := caps.EventCapability[ev.Name]; gated && !inst.Grants.Has(need) {
				continue
			}
			raw, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", raw)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
