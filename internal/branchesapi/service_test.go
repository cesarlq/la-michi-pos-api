package branchesapi

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

type fakeQuerier struct {
	branches []db.Branch
}

func (f *fakeQuerier) ListBranches(_ context.Context) ([]db.Branch, error) {
	out := make([]db.Branch, 0)
	for _, b := range f.branches {
		if b.Active {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeQuerier) ListAllBranches(_ context.Context) ([]db.Branch, error) {
	return f.branches, nil
}

func (f *fakeQuerier) GetBranch(_ context.Context, id string) (db.Branch, error) {
	for _, b := range f.branches {
		if b.ID == id {
			return b, nil
		}
	}
	return db.Branch{}, pgx.ErrNoRows
}

func (f *fakeQuerier) CreateBranch(_ context.Context, arg db.CreateBranchParams) (db.Branch, error) {
	b := db.Branch{ID: "new", Name: arg.Name, Address: arg.Address, Phone: arg.Phone, Active: true}
	f.branches = append(f.branches, b)
	return b, nil
}

func (f *fakeQuerier) UpdateBranch(_ context.Context, arg db.UpdateBranchParams) (db.Branch, error) {
	for i, b := range f.branches {
		if b.ID == arg.ID {
			if arg.Name != nil {
				f.branches[i].Name = *arg.Name
			}
			if arg.Address != nil {
				f.branches[i].Address = arg.Address
			}
			return f.branches[i], nil
		}
	}
	return db.Branch{}, pgx.ErrNoRows
}

func (f *fakeQuerier) SetBranchActive(_ context.Context, arg db.SetBranchActiveParams) (db.Branch, error) {
	for i, b := range f.branches {
		if b.ID == arg.ID {
			f.branches[i].Active = arg.Active
			return f.branches[i], nil
		}
	}
	return db.Branch{}, pgx.ErrNoRows
}

func seed() *fakeQuerier {
	return &fakeQuerier{branches: []db.Branch{
		{ID: "b1", Name: "Centro", Active: true},
		{ID: "b2", Name: "Norte", Active: false},
	}}
}

func TestList_OnlyActive(t *testing.T) {
	svc := NewService(seed())
	got, err := svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, quería 1 activa", len(got))
	}
}

func TestList_All(t *testing.T) {
	svc := NewService(seed())
	got, err := svc.List(context.Background(), true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, quería 2 (todas)", len(got))
	}
}

func TestCreate_OK(t *testing.T) {
	svc := NewService(seed())
	b, err := svc.Create(context.Background(), CreateInput{Name: "  Sur  "})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if b.Name != "Sur" {
		t.Errorf("Name = %q, quería 'Sur' (trim)", b.Name)
	}
}

func TestCreate_EmptyName(t *testing.T) {
	svc := NewService(seed())
	_, err := svc.Create(context.Background(), CreateInput{Name: "   "})
	if !errors.Is(err, ErrEmptyName) {
		t.Fatalf("err = %v, quería ErrEmptyName", err)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	svc := NewService(seed())
	name := "X"
	_, err := svc.Update(context.Background(), "nope", UpdateInput{Name: &name})
	if !errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("err = %v, quería ErrBranchNotFound", err)
	}
}

func TestSetActive_Reactivate(t *testing.T) {
	svc := NewService(seed())
	b, err := svc.SetActive(context.Background(), "b2", true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !b.Active {
		t.Error("la sucursal debería quedar activa")
	}
}
