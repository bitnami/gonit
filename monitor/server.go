package monitor

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
)

type monitorServer struct {
	http.Server
	SocketFile string
	Port       int
	monitor    *Monitor
	logger     Logger
	listener   *net.Listener
}

func (ms *monitorServer) ConnectionString() string {
	cs := ""
	if ms.Port != 0 {
		cs = fmt.Sprintf("http://localhost:%d", ms.Port)
	} else if ms.SocketFile != "" {
		cs = fmt.Sprintf("unix://%s", ms.SocketFile)
	}
	return cs
}

func (ms *monitorServer) Start() error {
	if err := syscall.Unlink(ms.SocketFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	mask := syscall.Umask(0777)
	defer syscall.Umask(mask)
	ln, err := net.Listen("unix", ms.SocketFile)
	// we found issues with other files accessing the umask before it gets restored
	// we explicitly call it here while preserving the defer in case of panics
	syscall.Umask(mask)
	if err != nil {
		return err
	}
	if err := os.Chmod(ms.SocketFile, 0660); err != nil {
		ln.Close()
		return err
	}
	ms.listener = &ln
	go ms.Serve(ln)
	return nil
}

func (ms *monitorServer) Stop() error {
	if ms.listener == nil {
		return fmt.Errorf("Refused to close a nil listener")
	}
	return (*ms.listener).Close()
}

type cmdResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

func (ms *monitorServer) formatResponse(fn func() (bool, string)) string {
	success, msg := fn()
	cmdResponse := cmdResponse{Success: success, Msg: msg}
	res, _ := json.MarshalIndent(cmdResponse, "", "  ")
	return string(res)
}

func (ms *monitorServer) defineServiceCmdRoutes(router *httprouter.Router, id string, cb func(string) error, shouldSkip func(interface {
	Checkable
}) bool) {

	router.POST(fmt.Sprintf("/%s/:service", id), func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		serviceName := ps.ByName("service")
		ms.logger.Debugf("[CLIENT_REQUEST] Requested execution of \"%s %s\"", id, serviceName)

		fmt.Fprintf(w, ms.formatResponse(func() (bool, string) {
			err := cb(serviceName)
			if err != nil {
				return false, err.Error()
			}
			return true, ""
		}))
	})

	router.POST(fmt.Sprintf("/%s_all", id), func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ms.logger.Debugf("[CLIENT_REQUEST] Requested execution of \"%s all\"", id)
		fmt.Fprintf(w, ms.formatResponse(func() (bool, string) {
			errMsgs := []string{}
			for _, c := range ms.monitor.checks {
				if shouldSkip(c) {
					continue
				}
				err := cb(c.GetID())
				if err != nil {
					errMsgs = append(errMsgs, err.Error())
				}
			}
			if len(errMsgs) > 0 {
				return false, strings.Join(errMsgs, "\n")
			}
			return true, ""
		}))
	})
}

func createServer(monitor *Monitor) *monitorServer {
	s := &monitorServer{
		SocketFile: monitor.SocketFile,
		logger:     monitor.logger,
		monitor:    monitor,
		Server: http.Server{
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}

	router := httprouter.New()

	for cmd, cb := range map[string]func(interface {
		CheckableProcess
	}) error{
		"start":   startProcess,
		"stop":    stopProcess,
		"restart": restartProcess,
	} {
		func(cb func(interface {
			CheckableProcess
		}) error) {
			s.defineServiceCmdRoutes(router, cmd, func(id string) error {
				c, err := monitor.findProcessCheck(id)
				if err != nil {
					return err
				}
				uid := c.GetUniqueID()
				timeout := c.GetTimeout() + (5 * time.Second)
				blocked := doOnce(uid, func() { cb(c) }, timeout, Opts{Logger: s.logger})
				if blocked {
					return fmt.Errorf("[%s] Other action already in progress -- please try again later", id)
				}
				return nil
			}, func(e interface {
				Checkable
			}) bool {
				_, ok := e.(CheckableProcess)
				return !ok
			})
		}(cb)
	}

	// monitor and unmonitor are synchronous
	for cmd, cb := range map[string]func(interface {
		Checkable
	}) error{
		"monitor":   monitor.monitorCheck,
		"unmonitor": monitor.unmonitorCheck,
	} {
		func(cb func(interface {
			Checkable
		}) error) {
			s.defineServiceCmdRoutes(router, cmd, func(id string) error {
				c := monitor.FindCheck(id)
				if c == nil {
					return fmt.Errorf("Cannot find check with id %s", id)
				}
				return cb(c)
			}, func(e interface {
				Checkable
			}) bool {
				_, ok := e.(Checkable)
				return !ok
			})
		}(cb)
	}

	for id, fn := range map[string]func() string{
		"status":  monitor.StatusText,
		"summary": monitor.SummaryText,
	} {
		func(cmd string, cb func() string) {
			router.GET(fmt.Sprintf("/%s", cmd), func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
				s.logger.Debugf("[CLIENT_REQUEST] Requested execution of \"%s\"", cmd)
				fmt.Fprintf(w, s.formatResponse(func() (bool, string) {
					return true, cb()
				}))
			})
		}(id, fn)
	}
	s.Handler = router
	return s
}
