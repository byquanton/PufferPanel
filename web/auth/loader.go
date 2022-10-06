/*
 Copyright 2020 Padduck, LLC
  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at
  	http://www.apache.org/licenses/LICENSE-2.0
  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package auth

import (
	"github.com/gin-gonic/gin"
	cors "github.com/itsjamie/gin-cors"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/middleware/panelmiddleware"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.Use(func(c *gin.Context) {
		middleware.ResponseAndRecover(c)
	})
	rg.POST("login", panelmiddleware.NeedsDatabase, LoginPost)
	rg.POST("logout", panelmiddleware.NeedsDatabase, LogoutPost)
	rg.POST("otp", panelmiddleware.NeedsDatabase, OtpPost)
	rg.POST("register", panelmiddleware.NeedsDatabase, RegisterPost)
	rg.POST("reauth", panelmiddleware.AuthMiddleware, panelmiddleware.NeedsDatabase, Reauth)
	rg.Handle("GET", "/node/socket", middleware.RequiresPermission(pufferpanel.ScopeServersConsole, true), cors.Middleware(cors.Config{
		Origins:     "*",
		Credentials: true,
	}), registerNodeLogin)
}
