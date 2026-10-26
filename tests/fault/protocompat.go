package fault

// T10 (design §10): a client built against an older .proto must keep
// working with a newer server and the other way round. On the wire a field
// is only its number and type, so the rules are: an existing field keeps
// its number, type and name; a removed field's number is reserved and
// never reused; new fields only add numbers. The current descriptors are
// compared with a committed snapshot of every API message.

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	_ "github.com/hughtaiyaki/goframeshop/app/admin/api/admin/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/banner/api/position/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/banner/api/rotation/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/file/api/file/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/interaction/api/collection/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/interaction/api/comment/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/interaction/api/praise/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/order/api/payment/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/paysim/api/provider/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/search/api/search/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"
	_ "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
)

// ProtoSnapshot maps "package.Message" to its fields ("number" -> "name
// kind") and reserved numbers ("number" -> "reserved").
type ProtoSnapshot map[string]map[string]string

// apiPackages are the proto packages of this repository's services (the
// package lines of app/*/manifest/protobuf, without pbentity). Messages of
// dependencies (etcd, gRPC) are not this repository's contract.
var apiPackages = map[string]bool{
	"admin.v1": true, "cart.v1": true, "category.v1": true, "collection.v1": true, "comment.v1": true,
	"consignee.v1": true, "file.v1": true, "goods.v1": true, "inventory.v1": true, "order.v1": true,
	"payment.v1": true, "position.v1": true, "praise.v1": true, "provider.v1": true, "refund.v1": true,
	"rotation.v1": true, "search.v1": true, "user.v1": true,
}

// CurrentProtos describes every API message the services expose.
func CurrentProtos() ProtoSnapshot {
	out := ProtoSnapshot{}
	protoregistry.GlobalFiles.RangeFiles(func(f protoreflect.FileDescriptor) bool {
		if apiPackages[string(f.Package())] {
			addMessages(out, f.Messages())
		}
		return true
	})
	return out
}

func addMessages(out ProtoSnapshot, msgs protoreflect.MessageDescriptors) {
	for i := 0; i < msgs.Len(); i++ {
		m := msgs.Get(i)
		fields := map[string]string{}
		for j := 0; j < m.Fields().Len(); j++ {
			f := m.Fields().Get(j)
			kind := f.Kind().String()
			switch {
			case f.Message() != nil:
				kind = string(f.Message().FullName())
			case f.Enum() != nil:
				kind = string(f.Enum().FullName())
			}
			if f.IsList() {
				kind = "repeated " + kind
			}
			fields[fmt.Sprint(f.Number())] = string(f.Name()) + " " + kind
		}
		ranges := m.ReservedRanges()
		for j := 0; j < ranges.Len(); j++ {
			r := ranges.Get(j)
			for n := r[0]; n < r[1]; n++ {
				fields[fmt.Sprint(n)] = "reserved"
			}
		}
		out[string(m.FullName())] = fields
		addMessages(out, m.Messages())
	}
}

// CompareProtos returns every way current breaks a client built against
// old. Added messages and fields are fine.
func CompareProtos(old, current ProtoSnapshot) []string {
	var bad []string
	names := make([]string, 0, len(old))
	for name := range old {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		now, ok := current[name]
		if !ok {
			bad = append(bad, fmt.Sprintf("%s was removed", name))
			continue
		}
		for number, was := range old[name] {
			is, ok := now[number]
			switch {
			case was == "reserved" && ok && is != "reserved":
				bad = append(bad, fmt.Sprintf("%s: reserved field %s reused as %q", name, number, is))
			case was == "reserved":
			case !ok:
				bad = append(bad, fmt.Sprintf("%s: field %s (%s) removed without reserving its number", name, number, was))
			case is == "reserved":
				// removed and reserved: old clients still decode
			case is != was:
				bad = append(bad, fmt.Sprintf("%s: field %s changed from %q to %q", name, number, was, is))
			}
		}
	}
	return bad
}
