package domain

import (
	"strings"
	"testing"

	"openterminal/internal/model"
)

func groupByID(c *model.Content, id string) model.Group {
	for _, g := range c.Groups {
		if g.ID == id {
			return g
		}
	}
	return model.Group{}
}

func TestCreateGroupOrder(t *testing.T) {
	c := testContent()
	a, _ := CreateGroup(c, "  Produção  ", nil)
	b, _ := CreateGroup(c, "Clientes", nil)
	child, _ := CreateGroup(c, "ACME", &b.ID)
	if a.Name != "Produção" || a.ParentID != nil || a.SortOrder != 0 || b.SortOrder != 1 {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
	if *child.ParentID != b.ID || child.SortOrder != 0 {
		t.Fatalf("child=%+v", child)
	}
}

func TestGroupNames(t *testing.T) {
	_, err := CreateGroup(testContent(), "   ", nil)
	wantCode(t, err, "INVALID_INPUT")
	_, err = CreateGroup(testContent(), strings.Repeat("x", 101), nil)
	wantCode(t, err, "INVALID_INPUT")
	c := testContent()
	p, err := CreateGroup(c, strings.Repeat("x", 100), nil)
	mustOK(t, err)
	_, _ = CreateGroup(c, "Web", nil)
	w, err := CreateGroup(c, "Web", &p.ID)
	mustOK(t, err)
	if w.Name != "Web" {
		t.Fatal("nome igual em pai diferente")
	}
	_, err = CreateGroup(testContent(), "A", model.Ptr(missing))
	wantCode(t, err, "INVALID_INPUT")
}

func TestRenameGroup(t *testing.T) {
	c := testContent()
	g, _ := CreateGroup(c, "A", nil)
	r, err := RenameGroup(c, g.ID, " B ")
	mustOK(t, err)
	if r.Name != "B" {
		t.Fatal(r.Name)
	}
	_, err = RenameGroup(c, missing, "C")
	wantCode(t, err, "NOT_FOUND")
}

func TestMoveGroup(t *testing.T) {
	c := testContent()
	a, _ := CreateGroup(c, "A", nil)
	b, _ := CreateGroup(c, "B", nil)
	_, _ = CreateGroup(c, "B1", &b.ID)
	mustOK(t, MoveGroup(c, a.ID, &b.ID))
	if g := groupByID(c, a.ID); *g.ParentID != b.ID || g.SortOrder != 1 {
		t.Fatalf("%+v", g)
	}
	mustOK(t, MoveGroup(c, a.ID, nil))
	if groupByID(c, a.ID).ParentID != nil {
		t.Fatal("não foi para a raiz")
	}
}

func TestMoveGroupCycle(t *testing.T) {
	c := testContent()
	a, _ := CreateGroup(c, "A", nil)
	b, _ := CreateGroup(c, "B", &a.ID)
	d, _ := CreateGroup(c, "D", &b.ID)
	wantCode(t, MoveGroup(c, a.ID, &a.ID), "GROUP_CYCLE")
	wantCode(t, MoveGroup(c, a.ID, &d.ID), "GROUP_CYCLE")
	wantCode(t, MoveGroup(c, a.ID, model.Ptr(missing)), "INVALID_INPUT")
	if groupByID(c, a.ID).ParentID != nil {
		t.Fatal("mudou após erro")
	}
}

func TestRemoveGroupLiftsChildren(t *testing.T) {
	c := testContent()
	a, _ := CreateGroup(c, "A", nil)
	b, _ := CreateGroup(c, "B", &a.ID)
	deep, _ := CreateGroup(c, "C", &b.ID)
	in1 := baseInput()
	in1.GroupID = &a.ID
	h1, _ := CreateHost(c, in1, t0)
	in2 := baseInput()
	in2.GroupID = &b.ID
	h2, _ := CreateHost(c, in2, t0)
	mustOK(t, RemoveGroup(c, a.ID))
	if len(c.Groups) != 2 || c.Groups[0].ID != b.ID || c.Groups[1].ID != deep.ID {
		t.Fatalf("grupos: %+v", c.Groups)
	}
	if groupByID(c, b.ID).ParentID != nil || *groupByID(c, deep.ID).ParentID != b.ID {
		t.Fatal("hierarquia errada")
	}
	for _, h := range c.Hosts {
		if h.ID == h1.ID && h.GroupID != nil {
			t.Fatal("h1 deveria ir para a raiz")
		}
		if h.ID == h2.ID && *h.GroupID != b.ID {
			t.Fatal("h2 deveria ficar em B")
		}
	}
	wantCode(t, RemoveGroup(c, a.ID), "NOT_FOUND")
}
