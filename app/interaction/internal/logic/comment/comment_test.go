package comment

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
)

type fakeCommentRepo struct {
	rows   map[uint64]*entity.CommentInfo
	nextID uint64
}

func newFakeCommentRepo() *fakeCommentRepo {
	return &fakeCommentRepo{rows: make(map[uint64]*entity.CommentInfo)}
}

func (f *fakeCommentRepo) Create(_ context.Context, c *entity.CommentInfo) (uint64, error) {
	f.nextID++
	stored := *c
	stored.Id = f.nextID
	f.rows[f.nextID] = &stored
	return f.nextID, nil
}

func (f *fakeCommentRepo) FindByID(_ context.Context, id uint64) (*entity.CommentInfo, error) {
	row, ok := f.rows[id]
	if !ok {
		return nil, nil
	}
	copied := *row
	return &copied, nil
}

func (f *fakeCommentRepo) live(objectType uint32, objectID uint64) []*entity.CommentInfo {
	matched := make([]*entity.CommentInfo, 0)
	for _, row := range f.rows {
		if uint32(row.ObjectType) == objectType && row.ObjectId == objectID && row.DeletedAt == nil {
			matched = append(matched, row)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].Id > matched[j].Id })
	return matched
}

func (f *fakeCommentRepo) ListByObject(_ context.Context, objectType uint32, objectID uint64, page, size int) ([]*entity.CommentInfo, error) {
	matched := f.live(objectType, objectID)
	start := (page - 1) * size
	if start >= len(matched) {
		return []*entity.CommentInfo{}, nil
	}
	return matched[start:min(start+size, len(matched))], nil
}

func (f *fakeCommentRepo) CountByObject(_ context.Context, objectType uint32, objectID uint64) (int, error) {
	return len(f.live(objectType, objectID)), nil
}

func (f *fakeCommentRepo) SoftDelete(_ context.Context, userID, id uint64) error {
	if row, ok := f.rows[id]; ok && row.UserId == userID && row.DeletedAt == nil {
		row.DeletedAt = gtime.Now()
	}
	return nil
}

const (
	alice uint64 = 1
	bob   uint64 = 2
)

func goods(id uint64, parent uint64, content string) *service.CommentInput {
	return &service.CommentInput{ObjectType: service.ObjectTypeGoods, ObjectID: id, ParentID: parent, Content: content}
}

func TestCreateValidatesContent(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeCommentRepo())

	// 255 Chinese characters is 765 bytes but exactly fits VARCHAR(255).
	if _, err := svc.Create(ctx, alice, goods(1, 0, strings.Repeat("好", 255))); err != nil {
		t.Fatalf("255 runes: %v", err)
	}
	bad := map[string]*service.CommentInput{
		"empty":            goods(1, 0, ""),
		"whitespace only":  goods(1, 0, " \n\t "),
		"256 runes":        goods(1, 0, strings.Repeat("好", 256)),
		"unknown type":     {ObjectType: 3, ObjectID: 1, Content: "x"},
		"missing objectId": goods(0, 0, "x"),
	}
	for name, in := range bad {
		if _, err := svc.Create(ctx, alice, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
}

func TestCreateStoresTrimmedContentUnderTheCaller(t *testing.T) {
	ctx := context.Background()
	repo := newFakeCommentRepo()
	id, err := New(repo).Create(ctx, alice, goods(1, 0, "  great  "))
	if err != nil {
		t.Fatal(err)
	}
	if row := repo.rows[id]; row.UserId != alice || row.Content != "great" {
		t.Fatalf("stored %+v", row)
	}
}

func TestReplyMustTargetALiveCommentOnTheSameObject(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeCommentRepo())
	parent, err := svc.Create(ctx, alice, goods(1, 0, "top"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, bob, goods(1, parent, "reply")); err != nil {
		t.Fatalf("valid reply: %v", err)
	}

	deleted, err := svc.Create(ctx, alice, goods(1, 0, "to delete"))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, alice, deleted); err != nil {
		t.Fatal(err)
	}

	bad := map[string]*service.CommentInput{
		"missing parent":        goods(1, 999, "reply"),
		"parent on other goods": goods(2, parent, "reply"),
		"parent of other type":  {ObjectType: service.ObjectTypeArticle, ObjectID: 1, ParentID: parent, Content: "reply"},
		"deleted parent":        goods(1, deleted, "reply"),
	}
	for name, in := range bad {
		if _, err := svc.Create(ctx, bob, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
}

func TestOnlyTheAuthorCanDelete(t *testing.T) {
	ctx := context.Background()
	repo := newFakeCommentRepo()
	svc := New(repo)
	id, err := svc.Create(ctx, alice, goods(1, 0, "mine"))
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete(ctx, bob, id); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("other user: err = %v, want ErrNotFound", err)
	}
	if repo.rows[id].DeletedAt != nil {
		t.Fatal("another user deleted alice's comment")
	}
	if err := svc.Delete(ctx, alice, id); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, alice, id); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestListShowsLiveCommentsOfOneObjectNewestFirst(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeCommentRepo())
	first, _ := svc.Create(ctx, alice, goods(1, 0, "first"))
	second, _ := svc.Create(ctx, bob, goods(1, first, "second"))
	gone, _ := svc.Create(ctx, alice, goods(1, 0, "gone"))
	if _, err := svc.Create(ctx, alice, goods(2, 0, "other goods")); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, alice, gone); err != nil {
		t.Fatal(err)
	}

	items, total, err := svc.List(ctx, service.ObjectTypeGoods, 1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 || items[0].ID != second || items[1].ID != first {
		t.Fatalf("total=%d items=%+v", total, items)
	}
	if items[0].ParentID != first {
		t.Fatalf("reply lost its parent: %+v", items[0])
	}
}
