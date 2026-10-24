package plugin

// SurfaceHost describes an app-provided host for a surface.
type SurfaceHost struct {
	Mount string
	DOM   string // shadow (default), light, frame
}

// SurfaceHostsFor returns the app-provided host descriptors for a plugin's surfaces.
func SurfaceHostsFor(m *Manifest) []SurfaceHost {
	var out []SurfaceHost
	for _, p := range m.Surfaces() {
		dom := p.DOMKind()
		out = append(out, SurfaceHost{Mount: p.Mount, DOM: dom})
	}
	return out
}

// IsLightDOM reports whether a manifest declares dom.inline.
func IsLightDOM(m *Manifest) bool {
	return m.HasCapability("dom.inline")
}

// IsShadowHost reports default shadow.
func IsShadowHost(m *Manifest) bool { return !IsLightDOM(m) }

// RenderConfig returns the declarative config fields (core renders form).
func RenderConfig(m *Manifest) []ConfigField     { return m.Config }
func RenderConfigForm(m *Manifest) []ConfigField { return m.Config }
func ConfigForm(m *Manifest) []ConfigField       { return m.Config }

// ConfigSurface returns a plugin-provided config surface, if any, taking precedence.
func ConfigSurface(m *Manifest) *Provides {
	for i := range m.Surfaces() {
		s := m.Surfaces()[i]
		if s.ID == "config" || s.ID == "settings" || s.Mount == "dialog" {
			// heuristic: a surface named config/settings or a dialog surface
			cp := s
			return &cp
		}
	}
	return nil
}

// HasConfigSurface reports whether plugin provides its own config surface.
func HasConfigSurface(m *Manifest) bool { return ConfigSurface(m) != nil }

// ConfigTakesPrecedence is an alias helper.
func ConfigTakesPrecedence(m *Manifest) bool { return HasConfigSurface(m) }
