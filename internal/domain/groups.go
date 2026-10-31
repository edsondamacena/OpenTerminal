package domain

import (
	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/validate"
)

func findGroup(c *model.Content, id string) *model.Group {
	for i := range c.Groups {
		if c.Groups[i].ID == id {
			return &c.Groups[i]
		}
	}
	return nil
}

func sameParent(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// nextSortOrder: posição no fim dos irmãos.
func nextSortOrder(c *model.Content, parentID *string) int {
	max := -1
	for _, g := range c.Groups {
		if sameParent(g.ParentID, parentID) && g.SortOrder > max {
			max = g.SortOrder
		}
	}
	return max + 1
}

// isSelfOrDescendant: candidate é ancestorID ou está abaixo dele.
func isSelfOrDescendant(c *model.Content, ancestorID string, candidate *string) bool {
	seen := map[string]bool{}
	for cur := candidate; cur != nil && !seen[*cur]; {
		if *cur == ancestorID {
			return true
		}
		seen[*cur] = true
		g := findGroup(c, *cur)
		if g == nil {
			return false
		}
		cur = g.ParentID
	}
	return false
}

func ListGroups(c *model.Content) []model.Group {
	return append([]model.Group{}, c.Groups...)
}

func CreateGroup(c *model.Content, name string, parentID *string) (model.Group, error) {
	clean, err := validate.Text(name, 1, 100)
	if err != nil {
		return model.Group{}, err
	}
	if parentID != nil && findGroup(c, *parentID) == nil {
		return model.Group{}, invalid()
	}
	g := model.Group{ID: NewID(), Name: clean, ParentID: parentID, SortOrder: nextSortOrder(c, parentID)}
	c.Groups = append(c.Groups, g)
	return g, nil
}

func RenameGroup(c *model.Content, id, name string) (model.Group, error) {
	clean, err := validate.Text(name, 1, 100)
	if err != nil {
		return model.Group{}, err
	}
	g := findGroup(c, id)
	if g == nil {
		return model.Group{}, notFound()
	}
	g.Name = clean
	return *g, nil
}

func MoveGroup(c *model.Content, id string, parentID *string) error {
	g := findGroup(c, id)
	if g == nil {
		return notFound()
	}
	if isSelfOrDescendant(c, id, parentID) {
		return apperr.New(apperr.GroupCycle)
	}
	if parentID != nil && findGroup(c, *parentID) == nil {
		return invalid()
	}
	if sameParent(g.ParentID, parentID) {
		return nil
	}
	g.SortOrder = nextSortOrder(c, parentID)
	g.ParentID = parentID
	return nil
}

// RemoveGroup: hosts e subgrupos diretos sobem para a raiz; não há exclusão em cascata.
func RemoveGroup(c *model.Content, id string) error {
	if findGroup(c, id) == nil {
		return notFound()
	}
	order := nextSortOrder(c, nil)
	for i := range c.Groups {
		if c.Groups[i].ParentID != nil && *c.Groups[i].ParentID == id {
			c.Groups[i].ParentID = nil
			c.Groups[i].SortOrder = order
			order++
		}
	}
	for i := range c.Hosts {
		if c.Hosts[i].GroupID != nil && *c.Hosts[i].GroupID == id {
			c.Hosts[i].GroupID = nil
		}
	}
	groups := c.Groups[:0]
	for _, g := range c.Groups {
		if g.ID != id {
			groups = append(groups, g)
		}
	}
	c.Groups = groups
	return nil
}
