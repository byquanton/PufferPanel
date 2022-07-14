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

package panelmiddleware

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
)

func HasOAuth2Token(c *gin.Context) {
	failure := true
	defer func() {
		if err := recover(); err != nil {
			logging.Error.Printf("Error handling auth check: %s\n%s", err, debug.Stack())
			failure = true
		}
		if failure && !c.IsAborted() {
			c.AbortWithStatus(500)
		}
	}()

	if c.Request.Method == http.MethodOptions {
		failure = false
		c.Next()
		return
	}

	authHeader := c.Request.Header.Get("Authorization")
	authHeader = strings.TrimSpace(authHeader)

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		c.Header(WWWAuthenticateHeader, WWWAuthenticateHeaderContents)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if parts[0] != "Bearer" || parts[1] == "" {
		c.Header(WWWAuthenticateHeader, WWWAuthenticateHeaderContents)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token, err := services.ParseToken(parts[1])

	if err != nil || !token.Valid {
		c.Header(WWWAuthenticateHeader, WWWAuthenticateHeaderContents)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.Set("token", token)
	failure = false
	c.Next()
}

func HasPermission(requiredScope pufferpanel.Scope, requireServer bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := GetDatabase(c)

		if db == nil {
			NeedsDatabase(c)
			db = GetDatabase(c)
			if db == nil {
				response.HandleError(c, pufferpanel.ErrDatabaseNotAvailable, http.StatusInternalServerError)
				return
			}
		}

		token, ok := c.Get("token")
		if !ok {
			response.HandleError(c, errors.New("token invalid"), http.StatusInternalServerError)
			return
		}
		jwtToken, ok := token.(*pufferpanel.Token)
		if !ok {
			response.HandleError(c, errors.New("token invalid"), http.StatusInternalServerError)
			return
		}

		ti := jwtToken.Claims

		ss := &services.Server{DB: db}
		us := &services.User{DB: db}

		var serverId string

		var server *models.Server
		var err error

		i := c.Param("serverId")
		if requireServer {
			server, err = ss.Get(i)
			if err != nil {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		if requireServer && (server == nil || server.Identifier == "") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		} else if requireServer {
			serverId = server.Identifier
		}

		userId, err := strconv.ParseUint(ti.Subject, 10, 64)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		user, err := us.GetById(uint(userId))
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		allowed := false

		//if this is an audience of oauth2, we can use token directly
		//we only use one audience, so we will only pull that one
		if len(ti.Audience) != 1 {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		audience := ti.Audience[0]

		if audience == "oauth2" {
			if requiredScope != pufferpanel.ScopeNone {
				scopes := ti.PanelClaims.Scopes[serverId]
				if scopes != nil && pufferpanel.ContainsScope(scopes, requiredScope) {
					allowed = true
				} else {
					//if there isn't a defined rule, is this user an admin?
					scopes := ti.PanelClaims.Scopes[""]
					if scopes != nil && pufferpanel.ContainsScope(scopes, pufferpanel.ScopeServersAdmin) {
						allowed = true
					}
				}
			} else {
				allowed = true
			}
		} else if audience == "session" {
			//otherwise, we have to look at what the user has since session based
			ps := &services.Permission{DB: db}
			var perms *models.Permissions
			if serverId == "" {
				perms, err = ps.GetForUserAndServer(user.ID, nil)
			} else {
				perms, err = ps.GetForUserAndServer(user.ID, &serverId)
			}

			if response.HandleError(c, err, http.StatusInternalServerError) {
				return
			}

			if requiredScope != pufferpanel.ScopeNone {
				if pufferpanel.ContainsScope(perms.ToScopes(), requiredScope) {
					allowed = true
				} else {
					perms, err = ps.GetForUserAndServer(user.ID, nil)
					if response.HandleError(c, err, http.StatusInternalServerError) {
						return
					}
					if pufferpanel.ContainsScope(perms.ToScopes(), pufferpanel.ScopeServersAdmin) {
						allowed = true
					}
				}
			} else {
				allowed = true
			}
		} else {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		if !allowed {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Set("server", server)
		c.Set("user", user)
		c.Next()
	}
}
