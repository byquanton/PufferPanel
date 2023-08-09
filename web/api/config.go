package api

import (
	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/config"
	"net/http"
	"os"
	"strings"
)

// @Summary Get config
// @Description Gets the editable config entries for the panel
// @Success 200 {object} pufferpanel.EditableConfig
// @Router /api/config [get]
// @Security OAuth2Application[none]
func panelConfig(c *gin.Context) {
	var themes []string
	files, err := os.ReadDir(config.WebRoot.Value() + "/theme")
	if err != nil {
		themes = append(themes, "PufferPanel")
	} else {
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".tar") {
				themes = append(themes, f.Name()[:len(f.Name())-4])
			}
		}
	}

	c.JSON(http.StatusOK, pufferpanel.EditableConfig{
		Themes: pufferpanel.ThemeConfig{
			Active:    config.DefaultTheme.Value(),
			Settings:  config.ThemeSettings.Value(),
			Available: themes,
		},
		Branding: pufferpanel.BrandingConfig{
			Name: config.CompanyName.Value(),
		},
		RegistrationEnabled: config.RegistrationEnabled.Value(),
	})
}
