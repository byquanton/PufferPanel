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

package main

import (
	"encoding/hex"
	"github.com/braintree/manners"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
	"github.com/pufferpanel/pufferpanel/v3/config"
	daemon "github.com/pufferpanel/pufferpanel/v3/daemon/entry"
	"github.com/pufferpanel/pufferpanel/v3/daemon/programs"
	"github.com/pufferpanel/pufferpanel/v3/database"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"github.com/pufferpanel/pufferpanel/v3/sftp"
	"github.com/pufferpanel/pufferpanel/v3/web"
	"github.com/spf13/cobra"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the panel",
	Run:   executeRun,
}

var webService *manners.GracefulServer

func executeRun(cmd *cobra.Command, args []string) {
	term := make(chan bool, 10)

	internalRun(term)
	//wait for the termination signal, so we can shut down
	<-term

	//shut down everything
	//all of these can be closed regardless of what type of install this is, as they all check if they are even being
	//used
	logging.Debug.Printf("stopping http server")
	if webService != nil {
		webService.Close()
	}

	logging.Debug.Printf("stopping sftp server")
	sftp.Stop()

	logging.Debug.Printf("stopping servers")
	programs.ShutdownService()
	for _, p := range programs.GetAll() {
		_ = p.Stop()
		p.RunningEnvironment.WaitForMainProcessFor(time.Minute) //wait 60 seconds
	}

	logging.Debug.Printf("stopping database connections")
	database.Close()
}

func internalRun(terminate chan bool) {
	logging.Initialize(true)
	signal.Ignore(syscall.SIGPIPE, syscall.SIGHUP)

	go func() {
		quit := make(chan os.Signal)
		// kill (no param) default send syscall.SIGTERM
		// kill -2 is syscall.SIGINT
		// kill -9 is syscall.SIGKILL but can"t be catch, so don't need add it
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logging.Info.Println("Shutting down...")
		terminate <- true
	}()

	if config.PanelEnabled.Value() {
		router := gin.New()
		router.Use(gin.Recovery())
		router.Use(gin.LoggerWithWriter(logging.Info.Writer()))
		gin.DefaultWriter = logging.Info.Writer()
		gin.DefaultErrorWriter = logging.Error.Writer()

		panel()

		if config.SessionKey.Value() == "" {
			k := securecookie.GenerateRandomKey(32)
			if err := config.SessionKey.Set(hex.EncodeToString(k), true); err != nil {
				logging.Error.Printf("error saving session key: %s", err.Error())
				terminate <- true
				return
			}
		}

		result, err := hex.DecodeString(config.SessionKey.Value())
		if err != nil {
			logging.Error.Printf("error decoding session key: %s", err.Error())
			terminate <- true
			return
		}
		sessionStore := cookie.NewStore(result)
		router.Use(sessions.Sessions("session", sessionStore))

		web.RegisterRoutes(router)

		go func() {
			l, err := net.Listen("tcp", config.WebHost.Value())
			if err != nil {
				logging.Error.Printf("error starting http server: %s", err.Error())
				terminate <- true
				return
			}

			logging.Info.Printf("Listening for HTTP requests on %s", l.Addr().String())
			webService = manners.NewWithServer(&http.Server{Handler: router})
			err = webService.Serve(l)
			if err != nil && err != http.ErrServerClosed {
				logging.Error.Printf("error listening for http requests: %s", err.Error())
				terminate <- true
			}
		}()
	}

	if config.DaemonEnabled.Value() {
		err := daemon.Start()
		if err != nil {
			logging.Error.Printf("error starting daemon server: %s", err.Error())
			terminate <- true
			return
		}
	}

	return
}

func panel() {
	services.LoadEmailService()

	//if we have the web, then let's use our sftp auth instead
	sftp.SetAuthorization(&services.DatabaseSFTPAuthorization{})

	_, err := database.GetConnection()
	if err != nil {
		logging.Error.Printf("Error connecting to database: %s", err.Error())
	}
}
