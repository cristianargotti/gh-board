package domain

import "strings"

func (p *tidyPlanner) inherit(it Item) {
	if it.Issue.Parent == nil || len(p.in.Config.Tidy.InheritFromParent) == 0 {
		return
	}
	parent, ok := tidyParent(it.Issue.Parent, p.in.Items)
	if !ok || !IsEpic(p.caps, parent) {
		p.skip(it, "inherit_from_parent", "épico pai não encontrado no conjunto de itens")
		return
	}
	for _, role := range p.in.Config.Tidy.InheritFromParent {
		field := p.inheritField(role)
		if field == "" {
			p.skip(it, role, "capacidade não mapeada em board.yml")
			continue
		}
		if value := parent.Text(field); value != "" {
			p.projectStep(it, field, value, role, "Herdar valor do épico pai")
		} else {
			p.skip(it, role, "épico pai sem valor para herdar")
		}
	}
}

func (p *tidyPlanner) inheritField(role string) string {
	switch role {
	case "lane":
		if p.caps.Lane != nil {
			return p.caps.Lane.Field
		}
	case "epic":
		if p.caps.Epic != nil {
			return p.caps.Epic.Field
		}
	}
	return ""
}

func tidyParent(ref *ParentRef, items []Item) (Item, bool) {
	for _, it := range items {
		if tidyChildOf(ref, it) {
			return it, true
		}
	}
	return Item{}, false
}

func tidyChildOf(ref *ParentRef, parent Item) bool {
	if ref == nil {
		return false
	}
	if ref.NodeID != "" && parent.Issue.NodeID != "" {
		return ref.NodeID == parent.Issue.NodeID
	}
	return ref.Number > 0 && ref.Number == parent.Issue.Number &&
		strings.EqualFold(ref.Owner, parent.Issue.Owner) && strings.EqualFold(ref.Repo, parent.Issue.Repo)
}
