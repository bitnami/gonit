package monitor

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	u "net/url"
	"time"
)

// Client allows connection to an existing monitor via a UNIX socket
// and use it through the same API as when directly using the monitor
// The main difference is that service management call don't block
type Client struct {
	Socket string
	httpc  *http.Client
	Error  error
}

// NewClient returns a new monitor Client instance using the provided socket
// to connect to a previously running daemon
func NewClient(socket string) interface {
	ChecksManager
} {
	c := &Client{Socket: socket}
	c.httpc = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Dial: func(_ string, _ string) (net.Conn, error) {
				return net.Dial("unix", socket)
			},
		},
	}
	return c
}

// StatusText returns a string containing a long description of all checks
// and Monitor attributes
func (c *Client) StatusText(args ...string) string {
	url := "http://localhost/status"
	if len(args) > 0 {
		url = fmt.Sprintf("%s/%s", url, u.PathEscape(args[0]))
	}
	r, err := c.httpc.Get(url)
	if err != nil {
		c.Error = fmt.Errorf("Error getting status %s", err.Error())
		return ""
	}
	msg, err := c.readResponse(r)
	if err != nil {
		c.Error = err
		return ""
	}
	return msg
}

// SummaryText returns a string containing a short status summary for every
// check registered
func (c *Client) SummaryText(args ...string) string {
	url := "http://localhost/summary"
	if len(args) > 0 {
		url = fmt.Sprintf("%s/%s", url, u.PathEscape(args[0]))
	}
	r, err := c.httpc.Get(url)
	if err != nil {
		c.Error = fmt.Errorf("Error getting summary %s", err.Error())
		return ""
	}
	msg, err := c.readResponse(r)
	if err != nil {
		c.Error = err
		return ""
	}
	return msg
}

func (c *Client) checkOperation(op string, args ...string) error {
	var url, id string
	if len(args) == 0 {
		id = ""
		url = fmt.Sprintf("http://localhost/%s_all", op)
	} else {
		id = args[0]
		url = fmt.Sprintf("http://localhost/%s/%s", op, id)
	}
	r, err := c.httpc.Post(url, "", nil)
	if err != nil {
		c.Error = fmt.Errorf("Error executing %s %s: %s", op, id, err.Error())
		return c.Error
	}

	_, err = c.readResponse(r)
	if err != nil {
		c.Error = err
		return err
	}
	return nil
}

// Monitor looks for the Check with the provide id and set its
// monitored status to true
func (c *Client) Monitor(id string) error {
	return c.checkOperation("monitor", id)
}

// Unmonitor looks for the Check with the provide id and set its
// monitored status to false
func (c *Client) Unmonitor(id string) error {
	return c.checkOperation("unmonitor", id)
}

// Start allows starting a process check by ID
func (c *Client) Start(id string) error {
	return c.checkOperation("start", id)
}

// Stop allows stopping a process check by ID
func (c *Client) Stop(id string) error {
	return c.checkOperation("stop", id)
}

// Restart allows restarting a process check by ID
func (c *Client) Restart(id string) error {
	return c.checkOperation("restart", id)
}

// MonitorAll set all checks monitored status to true
func (c *Client) MonitorAll() (errors []error) {
	if err := c.checkOperation("monitor"); err != nil {
		errors = []error{err}
	}
	return errors
}

// UnmonitorAll set all checks monitored status to false
func (c *Client) UnmonitorAll() (errors []error) {
	if err := c.checkOperation("unmonitor"); err != nil {
		errors = []error{err}
	}
	return errors
}

// StartAll allows starting all process checks
func (c *Client) StartAll() (errors []error) {
	if err := c.checkOperation("start"); err != nil {
		errors = []error{err}
	}
	return errors
}

// StopAll allows stopping all process checks
func (c *Client) StopAll() (errors []error) {
	if err := c.checkOperation("stop"); err != nil {
		errors = []error{err}
	}
	return errors
}

// RestartAll allows restarting all process checks
func (c *Client) RestartAll() (errors []error) {
	if err := c.checkOperation("restart"); err != nil {
		errors = []error{err}
	}
	return errors
}

func (c *Client) readResponse(resp *http.Response) (msg string, err error) {
	defer resp.Body.Close()
	body := resp.Body
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Got invalid response from server")
	}
	cmdResp := cmdResponse{}
	if err := json.NewDecoder(body).Decode(&cmdResp); err != nil {
		return "", err
	} else if !cmdResp.Success {
		return "", fmt.Errorf("%s", cmdResp.Msg)
	} else {
		return cmdResp.Msg, nil
	}
}
