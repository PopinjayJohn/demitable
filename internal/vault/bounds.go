package vault

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// checkBounds enforces depth and key-count caps on a parsed yaml.Node.
func checkBounds(n *yaml.Node) error {
	if n == nil {
		return nil
	}
	return walkBounds(n, 0)
}

func walkBounds(n *yaml.Node, depth int) error {
	if depth > MaxFrontmatterDepth {
		return fmt.Errorf("frontmatter deeper than %d levels", MaxFrontmatterDepth)
	}
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			if err := walkBounds(c, depth); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		if len(n.Content)/2 > MaxFrontmatterKeys {
			return fmt.Errorf("frontmatter has more than %d keys", MaxFrontmatterKeys)
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if err := walkBounds(n.Content[i], depth+1); err != nil {
				return err
			}
			if err := walkBounds(n.Content[i+1], depth+1); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if err := walkBounds(c, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
