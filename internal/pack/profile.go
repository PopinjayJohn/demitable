package pack

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// checkProfile enforces a strict YAML profile: no anchors/aliases, no merge
// keys, no duplicate keys, no implicit dates/booleans.
func checkProfile(root *yaml.Node) error {
	if root == nil {
		return fmt.Errorf("pack: empty document")
	}
	var walk func(n *yaml.Node, path string) error
	walk = func(n *yaml.Node, path string) error {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				if err := walk(c, path); err != nil {
					return err
				}
			}
		case yaml.AliasNode:
			return fmt.Errorf("pack: YAML anchors and aliases are not allowed (%s)", path)
		case yaml.MappingNode:
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				if k.Tag == "!!merge" {
					return fmt.Errorf("pack: YAML merge keys are not allowed (%s)", path)
				}
				if k.Kind != yaml.ScalarNode {
					return fmt.Errorf("pack: non-string key at %s", path)
				}
				if seen[k.Value] {
					return fmt.Errorf("pack: duplicate key %q at %s", k.Value, path)
				}
				seen[k.Value] = true
				if err := walk(v, path+"."+k.Value); err != nil {
					return err
				}
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				if err := walk(c, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		case yaml.ScalarNode:
			// Implicit dates are rejected: the writer quotes date-shaped scalars
			// (ADR-0049), so an unquoted date in a pack is a mistake, not a
			// value. YAML 1.1 boolean words (yes/no/on/off/y/n) arrive here
			// with tag !!str and are accepted, matching what the writer emits.
			if n.Tag == "!!timestamp" {
				return fmt.Errorf("pack: implicit dates are not allowed (%s = %q)", path, n.Value)
			}
			if n.Tag == "!!bool" && n.Value != "true" && n.Value != "false" {
				return fmt.Errorf("pack: %q is not a boolean; write true or false (%s)", n.Value, path)
			}
		}
		return nil
	}
	// unwrap document
	node := root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		node = root.Content[0]
	}
	if node == nil {
		return fmt.Errorf("pack: empty document")
	}
	return walk(root, "$")
}
