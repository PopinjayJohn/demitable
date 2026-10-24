package vault

// Visible applies the projection rules (seam; M5 replaces).
func Visible(p Principal, n Note) bool {
	if p.IsGM() {
		return true
	}
	if n.Visibility == "gm" {
		return false
	}
	if n.Campaign != "" && !contains(p.Campaigns, n.Campaign) {
		return false
	}
	switch n.Visibility {
	case "public":
		return true
	case "party":
		return n.Campaign != "" && contains(p.Campaigns, n.Campaign)
	case "owner", "private":
		return n.Owner != "" && n.Owner == p.ID
	case "":
		return false
	}
	return false
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
