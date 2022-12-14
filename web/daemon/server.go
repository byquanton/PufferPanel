/*
 Copyright 2022 (c) PufferPanel

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

package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/itsjamie/gin-cors"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/programs"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/satori/go.uuid"
	"github.com/spf13/cast"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func RegisterServerRoutes(e *gin.RouterGroup) {
	l := e.Group("/server")
	{
		l.PUT("/:serverId", middleware.RequiresPermission(pufferpanel.ScopeServersCreate, false), CreateServer)
		l.DELETE("/:serverId", middleware.RequiresPermission(pufferpanel.ScopeServersDelete, true), DeleteServer)
		l.GET("/:serverId", middleware.RequiresPermission(pufferpanel.ScopeServersEditAdmin, true), GetServerAdmin)
		l.POST("/:serverId", middleware.RequiresPermission(pufferpanel.ScopeServersEditAdmin, true), EditServerAdmin)
		l.OPTIONS("/:serverId", response.CreateOptions("PUT", "DELETE", "GET"))

		l.GET("/:serverId/data", middleware.RequiresPermission(pufferpanel.ScopeServersEdit, true), GetServerData)
		l.POST("/:serverId/data", middleware.RequiresPermission(pufferpanel.ScopeServersEdit, true), EditServerData)
		l.OPTIONS("/:serverId/data", response.CreateOptions("GET", "POST"))

		l.GET("/:serverId/tasks", middleware.RequiresPermission(pufferpanel.ScopeServersEdit, true), GetServerTasks)
		l.POST("/:serverId/tasks", middleware.RequiresPermission(pufferpanel.ScopeServersEdit, true), CreateServerTask)
		l.PUT("/:serverId/tasks/:taskId", middleware.RequiresPermission(pufferpanel.ScopeServersEdit, true), EditServerTask)
		l.DELETE("/:serverId/tasks/:taskId", middleware.RequiresPermission(pufferpanel.ScopeServersEdit, true), DeleteServerTask)
		l.OPTIONS("/:serverId/tasks", response.CreateOptions("GET", "POST", "PUT", "DELETE"))

		//l.POST("/:serverId/tasks/:taskId/run", middleware.OAuth2Handler(pufferpanel.ScopeServersEdit, true), RunServerTask)
		l.OPTIONS("/:serverId/tasks/:taskId/run", response.CreateOptions("POST"))

		l.POST("/:serverId/reload", middleware.RequiresPermission(pufferpanel.ScopeServersEditAdmin, true), ReloadServer)
		l.OPTIONS("/:serverId/reload", response.CreateOptions("POST"))

		l.POST("/:serverId/start", middleware.RequiresPermission(pufferpanel.ScopeServersStart, true), StartServer)
		l.OPTIONS("/:serverId/start", response.CreateOptions("POST"))

		l.POST("/:serverId/stop", middleware.RequiresPermission(pufferpanel.ScopeServersStop, true), StopServer)
		l.OPTIONS("/:serverId/stop", response.CreateOptions("POST"))

		l.POST("/:serverId/kill", middleware.RequiresPermission(pufferpanel.ScopeServersStop, true), KillServer)
		l.OPTIONS("/:serverId/kill", response.CreateOptions("POST"))

		l.POST("/:serverId/install", middleware.RequiresPermission(pufferpanel.ScopeServersInstall, true), InstallServer)
		l.OPTIONS("/:serverId/install", response.CreateOptions("POST"))

		l.GET("/:serverId/file/*filename", middleware.RequiresPermission(pufferpanel.ScopeServersFilesGet, true), GetFile)
		l.PUT("/:serverId/file/*filename", middleware.RequiresPermission(pufferpanel.ScopeServersFilesPut, true), PutFile)
		l.DELETE("/:serverId/file/*filename", middleware.RequiresPermission(pufferpanel.ScopeServersFilesPut, true), DeleteFile)
		l.POST("/:serverId/file/*filename", middleware.RequiresPermission(pufferpanel.ScopeServersFilesPut, true), response.NotImplemented)
		l.OPTIONS("/:serverId/file/*filename", response.CreateOptions("GET", "PUT", "DELETE", "POST"))

		l.GET("/:serverId/console", middleware.RequiresPermission(pufferpanel.ScopeServersConsole, true), GetLogs)
		l.POST("/:serverId/console", middleware.RequiresPermission(pufferpanel.ScopeServersConsoleSend, true), PostConsole)
		l.OPTIONS("/:serverId/console", response.CreateOptions("GET", "POST"))

		l.GET("/:serverId/stats", middleware.RequiresPermission(pufferpanel.ScopeServersStat, true), GetStats)
		l.OPTIONS("/:serverId/stats", response.CreateOptions("GET"))

		l.GET("/:serverId/status", middleware.RequiresPermission(pufferpanel.ScopeServersView, true), GetStatus)
		l.OPTIONS("/:serverId/status", response.CreateOptions("GET"))

		l.POST("/:serverId/archive/*filename", middleware.RequiresPermission(pufferpanel.ScopeServersFilesPut, true), Archive)
		l.GET("/:serverId/extract/*filename", middleware.RequiresPermission(pufferpanel.ScopeServersFilesPut, true), Extract)

	}

	p := e.Group("/socket")
	{
		p.GET("/:serverId", middleware.RequiresPermission(pufferpanel.ScopeServersConsole, true), cors.Middleware(cors.Config{
			Origins:     "*",
			Credentials: true,
		}), OpenSocket)
		p.Handle("CONNECT", "/:serverId", middleware.RequiresPermission(pufferpanel.ScopeServersConsole, true), func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Credentials", "false")
		})
		p.OPTIONS("/:serverId", response.CreateOptions("GET"))
	}

	l.POST("", middleware.RequiresPermission(pufferpanel.ScopeServersCreate, false), CreateServer)
	l.OPTIONS("", response.CreateOptions("POST"))
}

func StartServer(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	_, wait := c.GetQuery("wait")

	if wait {
		err := server.Start()
		if response.HandleError(c, err, http.StatusInternalServerError) {
		} else {
			c.Status(http.StatusNoContent)
		}
	} else {
		go func() {
			err := server.Start()
			if err != nil {
				logging.Error.Printf("Error starting server %s: %s", server.Id(), err)
			}
		}()
		c.Status(http.StatusAccepted)
	}
}

func StopServer(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	_, wait := c.GetQuery("wait")

	err := server.Stop()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if wait {
		err = server.GetEnvironment().WaitForMainProcess()
		if response.HandleError(c, err, http.StatusInternalServerError) {
		} else {
			c.Status(http.StatusNoContent)
		}
	} else {
		c.Status(204)
	}
}

func KillServer(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	err := server.Kill()
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func CreateServer(c *gin.Context) {
	serverId := c.Param("serverId")
	if serverId == "" {
		id := uuid.NewV4()
		serverId = id.String()
	}
	prg, _ := programs.Get(serverId)

	if prg != nil {
		response.HandleError(c, pufferpanel.ErrServerAlreadyExists, http.StatusConflict)
		return
	}

	prg = programs.CreateProgram()
	err := json.NewDecoder(c.Request.Body).Decode(prg)
	if err != nil {
		logging.Error.Printf("Error decoding JSON body: %s", err)
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	prg.Identifier = serverId

	err = prg.Requirements.Test(prg.Server)
	if err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	if err := programs.Create(prg); err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		_ = programs.Delete(prg.Id())
		return
	}

	if err := prg.Scheduler.LoadMap(prg.Tasks); err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		_ = programs.Delete(prg.Id())
		return
	}

	if err := prg.Scheduler.Start(); err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		_ = programs.Delete(prg.Id())
		return
	}

	c.JSON(200, &pufferpanel.ServerIdResponse{Id: serverId})
}

func DeleteServer(c *gin.Context) {
	item, _ := c.Get("server")
	prg := item.(*programs.Program)
	err := programs.Delete(prg.Id())
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func InstallServer(c *gin.Context) {
	item, _ := c.Get("server")
	prg := item.(*programs.Program)

	_, wait := c.GetQuery("wait")

	if wait {
		err := prg.Install()
		if response.HandleError(c, err, http.StatusInternalServerError) {
		} else {
			c.Status(http.StatusNoContent)
		}
	} else {
		go func(p *programs.Program) {
			_ = p.Install()
		}(prg)

		c.Status(http.StatusAccepted)
	}
}

func EditServerData(c *gin.Context) {
	item, _ := c.Get("server")
	prg := item.(*programs.Program)

	data := &pufferpanel.ServerData{}
	err := json.NewDecoder(c.Request.Body).Decode(&data)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = prg.EditData(data.Variables, isAdmin(c))
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func CreateServerTask(c *gin.Context) {
	item, _ := c.Get("server")
	prg := item.(*programs.Program)

	var task pufferpanel.Task
	err := c.ShouldBindJSON(&task)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	err = prg.Scheduler.Add(task)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func EditServerTask(c *gin.Context) {
	item, _ := c.Get("server")
	prg := item.(*programs.Program)

	var task pufferpanel.Task
	err := c.ShouldBindJSON(&task)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	err = prg.Scheduler.Remove(task.Name)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	err = prg.Scheduler.Add(task)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func DeleteServerTask(c *gin.Context) {
	item, _ := c.Get("server")
	prg := item.(*programs.Program)

	taskName := c.Param("taskName")

	err := prg.Scheduler.Remove(taskName)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func ReloadServer(c *gin.Context) {
	item, _ := c.Get("server")
	prg := item.(*programs.Program)

	err := programs.Reload(prg.Id())
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func GetServerData(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	data := server.GetData()

	if !isAdmin(c) {
		var replacement = make(map[string]pufferpanel.Variable)
		for k, v := range data {
			if v.UserEditable {
				replacement[k] = v
			}
		}
		data = replacement
	}

	c.JSON(200, &pufferpanel.ServerData{Variables: data})
}

func GetServerTasks(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	c.JSON(200, &pufferpanel.ServerTasks{Tasks: server.Tasks})
}

func GetServerAdmin(c *gin.Context) {
	item, _ := c.MustGet("server").(*programs.Program)

	c.JSON(200, &pufferpanel.ServerDataAdmin{Server: &item.Server})
}

func EditServerAdmin(c *gin.Context) {
	item, _ := c.MustGet("server").(*programs.Program)
	server := &item.Server

	replacement := &pufferpanel.Server{}
	err := c.BindJSON(replacement)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	//backup, just in case we break
	backup := &pufferpanel.Server{}
	backup.CopyFrom(server)

	//copy from request
	server.CopyFrom(replacement)

	err = programs.Save(item.Id())
	if response.HandleError(c, err, http.StatusInternalServerError) {
		//REVERT!!!!!!!
		server.CopyFrom(backup)
		return
	}

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

func GetFile(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	targetPath := c.Param("filename")

	data, err := server.GetItem(targetPath)
	defer func() {
		if data != nil {
			pufferpanel.Close(data.Contents)
		}
	}()

	if err != nil {
		if os.IsNotExist(err) {
			c.AbortWithStatus(404)
		} else if err == pufferpanel.ErrIllegalFileAccess {
			response.HandleError(c, err, http.StatusBadRequest)
		} else {
			response.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	if data.FileList != nil {
		c.JSON(200, data.FileList)
	} else if data.Contents != nil {
		fileName := filepath.Base(data.Name)

		extraHeaders := map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, fileName),
		}

		//discard the built-in response, we cannot use this one at all
		c.DataFromReader(http.StatusOK, data.ContentLength, "application/octet-stream", data.Contents, extraHeaders)
	} else {
		//uhhhhhhhhhhhhh
		response.HandleError(c, errors.New("no file content or file list"), http.StatusInternalServerError)
	}
}

func PutFile(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	targetPath := c.Param("filename")

	if targetPath == "" {
		c.Status(404)
		return
	}

	var err error

	_, mkFolder := c.GetQuery("folder")
	if mkFolder {
		err = server.CreateFolder(targetPath)
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	var sourceFile io.ReadCloser

	v := c.Request.Header.Get("Content-Type")
	if t, _, _ := mime.ParseMediaType(v); t == "multipart/form-data" {
		sourceFile, _, err = c.Request.FormFile("file")
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	} else {
		sourceFile = c.Request.Body
	}

	file, err := server.OpenFile(targetPath)
	defer pufferpanel.Close(file)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		_, err = io.Copy(file, sourceFile)
		if response.HandleError(c, err, http.StatusInternalServerError) {
		} else {
			c.Status(http.StatusNoContent)
		}
	}
}

func DeleteFile(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	targetPath := c.Param("filename")

	err := server.DeleteItem(targetPath)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func PostConsole(c *gin.Context) {
	item, _ := c.Get("server")
	prg := item.(*programs.Program)

	d, _ := ioutil.ReadAll(c.Request.Body)
	cmd := string(d)
	err := prg.Execute(cmd)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func GetStats(c *gin.Context) {
	item, _ := c.Get("server")
	svr := item.(*programs.Program)

	results, err := svr.GetEnvironment().GetStats()
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.JSON(200, results)
	}
}

func GetLogs(c *gin.Context) {
	item, _ := c.Get("server")
	program := item.(*programs.Program)

	time := c.DefaultQuery("time", "0")

	castedTime, ok := cast.ToInt64E(time)
	if ok != nil {
		response.HandleError(c, pufferpanel.ErrInvalidUnixTime, http.StatusBadRequest)
		return
	}

	console, epoch := program.GetEnvironment().GetConsoleFrom(castedTime)
	msg := ""
	for _, k := range console {
		msg += k
	}

	c.JSON(200, &pufferpanel.ServerLogs{
		Epoch: epoch,
		Logs:  msg,
	})
}

func GetStatus(c *gin.Context) {
	item, _ := c.Get("server")
	program := item.(*programs.Program)

	running, err := program.IsRunning()

	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.JSON(200, &pufferpanel.ServerRunning{Running: running})
	}
}

func Archive(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)
	var files []string

	if err := c.BindJSON(&files); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if len(files) == 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	destination := c.Param("filename")

	err := server.ArchiveItems(files, destination)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func Extract(c *gin.Context) {
	item, _ := c.Get("server")
	server := item.(*programs.Program)

	targetPath := c.Param("filename")
	destination := c.Query("destination")

	err := server.Extract(targetPath, destination)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

func OpenSocket(c *gin.Context) {
	item, _ := c.Get("server")
	program := item.(*programs.Program)

	conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	internalMap, _ := c.Get("scopes")
	scopes := internalMap.([]pufferpanel.Scope)

	socket := pufferpanel.Create(conn)

	go listenOnSocket(socket, program, scopes)

	program.GetEnvironment().AddListener(socket)
}

func isAdmin(c *gin.Context) bool {
	o, _ := c.Get("scopes")
	if scopes, ok := o.([]pufferpanel.Scope); ok {
		for _, v := range scopes {
			if v == pufferpanel.ScopeServersAdmin {
				return true
			}
		}
	}
	return false
}
