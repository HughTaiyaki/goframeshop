package consignee

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/user/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
)

// fakeConsigneeRepo is an in-memory stand-in for MySQL. Keeping the repository
// behind an interface is what makes these tests possible without Docker.
type fakeConsigneeRepo struct {
	records map[uint64]*entity.ConsigneeInfo
	nextID  uint64
}

func newFakeConsigneeRepo() *fakeConsigneeRepo {
	return &fakeConsigneeRepo{records: make(map[uint64]*entity.ConsigneeInfo)}
}

func (f *fakeConsigneeRepo) Create(_ context.Context, c *entity.ConsigneeInfo) (uint64, error) {
	f.nextID++
	stored := *c
	stored.Id = f.nextID
	f.records[f.nextID] = &stored
	return f.nextID, nil
}

func (f *fakeConsigneeRepo) FindByID(_ context.Context, id uint64) (*entity.ConsigneeInfo, error) {
	record, ok := f.records[id]
	if !ok {
		return nil, nil
	}
	copied := *record
	return &copied, nil
}

func (f *fakeConsigneeRepo) ListByUser(_ context.Context, userID uint64, page, size int) ([]*entity.ConsigneeInfo, error) {
	matched := make([]*entity.ConsigneeInfo, 0)
	for _, record := range f.records {
		if record.UserId == userID && record.DeletedAt == nil {
			matched = append(matched, record)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].Id > matched[j].Id })
	start := (page - 1) * size
	if start >= len(matched) {
		return []*entity.ConsigneeInfo{}, nil
	}
	end := start + size
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], nil
}

func (f *fakeConsigneeRepo) CountByUser(_ context.Context, userID uint64) (int, error) {
	total := 0
	for _, record := range f.records {
		if record.UserId == userID && record.DeletedAt == nil {
			total++
		}
	}
	return total, nil
}

func (f *fakeConsigneeRepo) Update(_ context.Context, c *entity.ConsigneeInfo) error {
	record, ok := f.records[c.Id]
	if !ok {
		return nil
	}
	record.IsDefault = c.IsDefault
	record.Name = c.Name
	record.Phone = c.Phone
	record.Province = c.Province
	record.City = c.City
	record.Town = c.Town
	record.Street = c.Street
	record.Detail = c.Detail
	return nil
}

func (f *fakeConsigneeRepo) UpdateAsDefault(_ context.Context, c *entity.ConsigneeInfo) error {
	for _, record := range f.records {
		if record.UserId == c.UserId {
			record.IsDefault = 0
		}
	}
	return f.Update(context.Background(), c)
}

func (f *fakeConsigneeRepo) SoftDelete(_ context.Context, userID, id uint64) error {
	record, ok := f.records[id]
	if !ok || record.UserId != userID {
		return nil
	}
	record.DeletedAt = gtime.Now()
	return nil
}

func addressInput(name string) *service.ConsigneeInput {
	return &service.ConsigneeInput{
		Name:     name,
		Phone:    "13800000000",
		Province: "广东省",
		City:     "东莞市",
		Town:     "松山湖",
		Street:   "大学路 1 号",
		Detail:   "宿舍 A 栋 101",
	}
}

func TestListReturnsOnlyOwnLiveAddresses(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeConsigneeRepo())

	if _, err := svc.Create(ctx, 1, addressInput("张三")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, 1, addressInput("李四")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, 2, addressInput("王五")); err != nil {
		t.Fatal(err)
	}

	items, total, err := svc.List(ctx, 1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	// Newest first: without a fixed order, page 1 and page 2 could overlap.
	if items[0].ID < items[1].ID {
		t.Fatalf("expected descending id order, got %d then %d", items[0].ID, items[1].ID)
	}
	for _, item := range items {
		if item.UserID != 1 {
			t.Fatalf("leaked another user's address: %d", item.UserID)
		}
	}
}

func TestListAfterDeleteExcludesDeleted(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeConsigneeRepo())
	id, err := svc.Create(ctx, 7, addressInput("张三"))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, 7, id); err != nil {
		t.Fatal(err)
	}
	items, total, err := svc.List(ctx, 7, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("after delete: total = %d, items = %d, want 0", total, len(items))
	}
}

func TestCrossUserAccessIsReportedAsNotFound(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeConsigneeRepo())
	id, err := svc.Create(ctx, 1, addressInput("张三"))
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Update(ctx, 2, id, addressInput("攻击者")); !errors.Is(err, service.ErrConsigneeNotFound) {
		t.Fatalf("update by another user: err = %v, want ErrConsigneeNotFound", err)
	}
	if err := svc.Delete(ctx, 2, id); !errors.Is(err, service.ErrConsigneeNotFound) {
		t.Fatalf("delete by another user: err = %v, want ErrConsigneeNotFound", err)
	}
}

func TestMissingAddressIsNotFound(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeConsigneeRepo())
	if err := svc.Delete(ctx, 1, 9999); !errors.Is(err, service.ErrConsigneeNotFound) {
		t.Fatalf("err = %v, want ErrConsigneeNotFound", err)
	}
}

func TestCreateRejectsIncompleteAddress(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeConsigneeRepo())
	incomplete := addressInput("")
	incomplete.Name = ""
	if _, err := svc.Create(ctx, 1, incomplete); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument", err)
	}
}

func TestSettingDefaultDemotesPreviousDefault(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeConsigneeRepo())

	first := addressInput("旧地址")
	first.IsDefault = true
	if _, err := svc.Create(ctx, 1, first); err != nil {
		t.Fatal(err)
	}

	second := addressInput("新地址")
	second.IsDefault = true
	if _, err := svc.Create(ctx, 1, second); err != nil {
		t.Fatal(err)
	}

	items, _, err := svc.List(ctx, 1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	defaults := 0
	for _, item := range items {
		if item.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("default address count = %d, want 1", defaults)
	}
}

func TestGetIsScopedToTheOwner(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeConsigneeRepo())
	id, err := svc.Create(ctx, 1, &service.ConsigneeInput{Name: "张三", Phone: "138", Detail: "A101"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, 1, id)
	if err != nil || got.Name != "张三" {
		t.Fatalf("owner: %+v %v", got, err)
	}
	if _, err := svc.Get(ctx, 2, id); !errors.Is(err, service.ErrConsigneeNotFound) {
		t.Fatalf("other user: err = %v", err)
	}
	if err := svc.Delete(ctx, 1, id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, 1, id); !errors.Is(err, service.ErrConsigneeNotFound) {
		t.Fatalf("deleted: err = %v", err)
	}
}
