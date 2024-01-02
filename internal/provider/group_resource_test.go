package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
)

func TestParseHttpResponse(t *testing.T) {
	t.Run("Basic Test", func(t *testing.T) {
		g := groupResourceModel{
			Id:   basetypes.NewInt64Value(1),
			Name: types.StringValue("group1"),
			URL:  types.StringValue("/api/v2/groups/24/"),
		}
		body := []byte(`{"name": "group1", "url": "/api/v2/groups/24/", "description": ""}`)
		err := g.ParseHttpResponse(body)
		assert.NoError(t, err)
	})
	t.Run("Test with variables", func(t *testing.T) {
		g := groupResourceModel{
			Id:   basetypes.NewInt64Value(1),
			Name: types.StringValue("group1"),
			URL:  types.StringValue("/api/v2/groups/24/"),
		}
		body := []byte(`{"name": "group1", "url": "/api/v2/groups/24/", "description": "", "variables": "{\"ansible_network_os\":\"ios\"}"}`)
		err := g.ParseHttpResponse(body)
		assert.NoError(t, err)
	})
}
