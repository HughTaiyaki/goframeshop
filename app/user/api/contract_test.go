package api_test

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"
	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestGeneratedRPCMethods(t *testing.T) {
	tests := []struct {
		name    string
		service protoreflect.ServiceDescriptor
		methods []string
	}{
		{
			name:    "user",
			service: userv1.File_user_v1_user_proto.Services().ByName("UserService"),
			methods: []string{"RegisterUser", "Login", "GetProfile", "ChangePassword", "UpdateProfile"},
		},
		{
			name:    "consignee",
			service: consigneev1.File_consignee_v1_consignee_proto.Services().ByName("ConsigneeService"),
			methods: []string{"ListConsignees", "CreateConsignee", "UpdateConsignee", "DeleteConsignee", "GetConsignee"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.service == nil {
				t.Fatal("generated service descriptor missing")
			}
			if got := tt.service.Methods().Len(); got != len(tt.methods) {
				t.Fatalf("method count = %d, want %d", got, len(tt.methods))
			}
			for _, name := range tt.methods {
				if tt.service.Methods().ByName(protoreflect.Name(name)) == nil {
					t.Errorf("generated method %q missing", name)
				}
			}
		})
	}
}

func TestPublicResponsesExcludePersistenceSecrets(t *testing.T) {
	for _, message := range []protoreflect.MessageDescriptor{
		userv1.File_user_v1_user_proto.Messages().ByName("UserProfile"),
		userv1.File_user_v1_user_proto.Messages().ByName("UpdateProfileRequest"),
		consigneev1.File_consignee_v1_consignee_proto.Messages().ByName("Consignee"),
	} {
		if message == nil {
			t.Fatal("public response message missing")
		}
		for _, forbidden := range []protoreflect.Name{
			"password", "password_hash", "password_salt", "secret_answer_hash", "deleted_at", "user_id",
		} {
			if message.Name() == "UserProfile" && forbidden == "user_id" {
				continue
			}
			if message.Fields().ByName(forbidden) != nil {
				t.Errorf("%s unexpectedly exposes %s", message.Name(), forbidden)
			}
		}
	}
}

func TestGeneratedValidationTags(t *testing.T) {
	tests := []struct {
		name string
		req  any
	}{
		{"missing username", &userv1.RegisterRequest{Password: "long-enough"}},
		{"zero page", &consigneev1.ListConsigneesRequest{Page: 0, Size: 10}},
		{"oversized page", &consigneev1.ListConsigneesRequest{Page: 1, Size: 101}},
		{"missing address id", &consigneev1.DeleteConsigneeRequest{}},
		{"unknown sex", &userv1.UpdateProfileRequest{Sex: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := g.Validator().Data(tt.req).Run(context.Background()); err == nil {
				t.Fatal("expected generated validation tag to reject request")
			}
		})
	}
}
