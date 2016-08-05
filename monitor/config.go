package monitor

import "time"

// Config reprosents the basic configuration settings supported by the monitor
type Config struct {
	ControlFile     string
	Verbose         bool
	ShouldDaemonize bool
	CheckInterval   time.Duration
	PidFile         string
	SocketFile      string
	StateFile       string
	LogFile         string
}

type configWalker interface {
	AddCheck(interface {
		Checkable
	}) error
	SetNamespacedConfig(namespace string, attrs map[string]string)
	SetAttribute(key, value string)
}

type configLoader struct {
	app    *Monitor
	Logger Logger
}

func (cl *configLoader) SetAttribute(key, value string) {
	cl.Logger.Debugf("Ignoring attempt to set %s = %s\n", key, value)
}
func (cl *configLoader) AddCheck(c interface {
	Checkable
}) error {
	return cl.app.AddCheck(c)
}

func (cl *configLoader) SetNamespacedConfig(namespace string, attrs map[string]string) {

	if namespace == "httpd" {
		for key, value := range attrs {
			switch key {
			case "unixsocket":
				cl.app.SocketFile = value
			default:
				cl.Logger.Debugf("Ignoring %s attribute %s", namespace, key)
			}
		}
	} else {
		cl.Logger.Debugf("Namespace %s not supported", namespace)
	}
}
