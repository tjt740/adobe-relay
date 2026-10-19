package admin

import (
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// List groups from all confirmed Okad imports, independent of the current page and filters.
func (h *AccountHandler) OkadGroups(c *gin.Context) {
	accounts, err := h.listAccountsFiltered(c.Request.Context(), service.PlatformAdobe, service.AccountTypeOAuth, "", "", 0, "", "id", "asc")
	if err != nil {
		response.Error(c, 503, "读取 Okad 分组失败")
		return
	}
	type option struct {
		Value string `json:"value"`
		Name  string `json:"name"`
	}
	groups := map[string]string{}
	for _, a := range accounts {
		key, name := service.OkadAccountGroup(&a)
		if key != "" && key != "ungrouped" {
			groups[key] = name
		}
	}
	items := make([]option, 0, len(groups))
	for key, name := range groups {
		items = append(items, option{key, name})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Value < items[j].Value
		}
		return items[i].Name < items[j].Name
	})
	response.Success(c, items)
}
