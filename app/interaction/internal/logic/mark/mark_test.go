package mark

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
)

// fakeMarkRepo is an in-memory stand-in for one mark table, including its
// unique key, so the duplicate rule is tested without MySQL.
type fakeMarkRepo struct {
	rows   map[uint64]*service.Mark
	nextID uint64
}

func newFakeMarkRepo() *fakeMarkRepo {
	return &fakeMarkRepo{rows: make(map[uint64]*service.Mark)}
}

func (f *fakeMarkRepo) Create(_ context.Context, m *service.Mark) (uint64, error) {
	for _, row := range f.rows {
		if row.UserID == m.UserID && row.ObjectType == m.ObjectType && row.ObjectID == m.ObjectID {
			return 0, service.ErrAlreadyExists
		}
	}
	f.nextID++
	stored := *m
	stored.ID = f.nextID
	f.rows[f.nextID] = &stored
	return f.nextID, nil
}

func (f *fakeMarkRepo) FindByID(_ context.Context, id uint64) (*service.Mark, error) {
	row, ok := f.rows[id]
	if !ok {
		return nil, nil
	}
	copied := *row
	return &copied, nil
}

func (f *fakeMarkRepo) matching(userID uint64, objectType uint32) []*service.Mark {
	matched := make([]*service.Mark, 0)
	for _, row := range f.rows {
		if row.UserID == userID && row.ObjectType == objectType {
			matched = append(matched, row)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID > matched[j].ID })
	return matched
}

func (f *fakeMarkRepo) ListByUser(_ context.Context, userID uint64, objectType uint32, page, size int) ([]*service.Mark, error) {
	matched := f.matching(userID, objectType)
	start := (page - 1) * size
	if start >= len(matched) {
		return []*service.Mark{}, nil
	}
	return matched[start:min(start+size, len(matched))], nil
}

func (f *fakeMarkRepo) CountByUser(_ context.Context, userID uint64, objectType uint32) (int, error) {
	return len(f.matching(userID, objectType)), nil
}

func (f *fakeMarkRepo) Delete(_ context.Context, userID, id uint64) error {
	if row, ok := f.rows[id]; ok && row.UserID == userID {
		delete(f.rows, id)
	}
	return nil
}

const (
	alice uint64 = 1
	bob   uint64 = 2
)

func TestCreateRejectsDuplicatesAndBadInput(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeMarkRepo())

	if _, err := svc.Create(ctx, alice, service.ObjectTypeGoods, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, alice, service.ObjectTypeGoods, 10); !errors.Is(err, service.ErrAlreadyExists) {
		t.Fatalf("duplicate: err = %v, want ErrAlreadyExists", err)
	}
	// The same object for another user, or another type for the same user, is
	// a different mark.
	if _, err := svc.Create(ctx, bob, service.ObjectTypeGoods, 10); err != nil {
		t.Fatalf("other user: %v", err)
	}
	if _, err := svc.Create(ctx, alice, service.ObjectTypeArticle, 10); err != nil {
		t.Fatalf("other type: %v", err)
	}
	for _, bad := range []struct {
		objectType uint32
		objectID   uint64
	}{{0, 10}, {3, 10}, {service.ObjectTypeGoods, 0}} {
		if _, err := svc.Create(ctx, alice, bad.objectType, bad.objectID); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("Create(%d, %d): err = %v, want ErrInvalidArgument", bad.objectType, bad.objectID, err)
		}
	}
}

func TestListOnlyShowsTheCallersOwnMarksOfOneType(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeMarkRepo())
	for _, objectID := range []uint64{1, 2, 3} {
		if _, err := svc.Create(ctx, alice, service.ObjectTypeGoods, objectID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Create(ctx, alice, service.ObjectTypeArticle, 9); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, bob, service.ObjectTypeGoods, 1); err != nil {
		t.Fatal(err)
	}

	items, total, err := svc.List(ctx, alice, service.ObjectTypeGoods, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(items) != 2 {
		t.Fatalf("total=%d len=%d, want 3 and 2", total, len(items))
	}
	if items[0].ObjectID != 3 || items[1].ObjectID != 2 {
		t.Fatalf("not newest first: %d, %d", items[0].ObjectID, items[1].ObjectID)
	}
	for _, item := range items {
		if item.UserID != alice || item.ObjectType != service.ObjectTypeGoods {
			t.Fatalf("leaked row %+v", item)
		}
	}
	if _, _, err := svc.List(ctx, alice, 0, 1, 10); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("type 0: err = %v, want ErrInvalidArgument", err)
	}
}

func TestDeleteIsScopedToTheOwner(t *testing.T) {
	ctx := context.Background()
	repo := newFakeMarkRepo()
	svc := New(repo)
	id, err := svc.Create(ctx, alice, service.ObjectTypeGoods, 10)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete(ctx, bob, id); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("someone else's mark: err = %v, want ErrNotFound", err)
	}
	if _, ok := repo.rows[id]; !ok {
		t.Fatal("another user deleted alice's mark")
	}
	if err := svc.Delete(ctx, alice, 999); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("missing mark: err = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, alice, id); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.rows[id]; ok {
		t.Fatal("mark still present after delete")
	}
	// Once deleted, the same object can be marked again.
	if _, err := svc.Create(ctx, alice, service.ObjectTypeGoods, 10); err != nil {
		t.Fatalf("re-create after delete: %v", err)
	}
}
