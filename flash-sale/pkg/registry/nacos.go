// Package registry wraps the Nacos Go SDK so that microservices and the gateway
// can register, discover and watch dynamic configuration without dragging the
// SDK types into business code.
package registry

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

const (
	DefaultGroup = "DEFAULT_GROUP"
)

// Options collects everything needed to talk to Nacos.
type Options struct {
	Host        string
	Port        uint64
	Namespace   string
	Group       string
	ServiceName string
	InstanceIP  string
	InstancePort uint64
	DataID      string
}

// Client groups the naming and config clients together and remembers the
// instance metadata so Deregister can run on shutdown.
type Client struct {
	naming naming_client.INamingClient
	config config_client.IConfigClient
	opts   Options

	mu         sync.Mutex
	registered bool
}

// New constructs a Nacos client. Either naming or config can fail to construct
// (e.g. if Nacos is unreachable); callers can still use whatever succeeded.
func New(opts Options) (*Client, error) {
	if opts.Host == "" {
		return nil, errors.New("nacos: host is required")
	}
	if opts.Port == 0 {
		opts.Port = 8848
	}
	if opts.Group == "" {
		opts.Group = DefaultGroup
	}

	clientCfg := constant.ClientConfig{
		NamespaceId:         opts.Namespace,
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		LogLevel:            "info",
	}
	serverCfgs := []constant.ServerConfig{
		*constant.NewServerConfig(opts.Host, opts.Port, constant.WithContextPath("/nacos")),
	}

	naming, err := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &clientCfg,
		ServerConfigs: serverCfgs,
	})
	if err != nil {
		return nil, fmt.Errorf("nacos: new naming client: %w", err)
	}

	cfg, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &clientCfg,
		ServerConfigs: serverCfgs,
	})
	if err != nil {
		return nil, fmt.Errorf("nacos: new config client: %w", err)
	}

	return &Client{naming: naming, config: cfg, opts: opts}, nil
}

// Register puts the current process into Nacos under opts.ServiceName.
// If opts.InstanceIP is empty, the first non-loopback IPv4 is auto-detected so
// other containers on the same Docker network can reach the service.
func (c *Client) Register() error {
	if c.opts.ServiceName == "" || c.opts.InstancePort == 0 {
		return errors.New("nacos: service_name and instance_port are required to register")
	}
	ip := c.opts.InstanceIP
	if ip == "" {
		detected, err := detectLocalIP()
		if err != nil {
			return fmt.Errorf("nacos: detect local ip: %w", err)
		}
		ip = detected
	}
	ok, err := c.naming.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        c.opts.InstancePort,
		ServiceName: c.opts.ServiceName,
		GroupName:   c.opts.Group,
		Weight:      1,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
	})
	if err != nil {
		return fmt.Errorf("nacos: register: %w", err)
	}
	if !ok {
		return errors.New("nacos: register returned false")
	}
	c.mu.Lock()
	c.registered = true
	c.opts.InstanceIP = ip
	c.mu.Unlock()
	return nil
}

// Deregister removes the instance. Safe to call even if Register failed.
func (c *Client) Deregister() error {
	c.mu.Lock()
	if !c.registered {
		c.mu.Unlock()
		return nil
	}
	c.registered = false
	c.mu.Unlock()

	_, err := c.naming.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          c.opts.InstanceIP,
		Port:        c.opts.InstancePort,
		ServiceName: c.opts.ServiceName,
		GroupName:   c.opts.Group,
		Ephemeral:   true,
	})
	return err
}

// SelectInstances returns the currently healthy instances of serviceName.
func (c *Client) SelectInstances(serviceName string) ([]model.Instance, error) {
	return c.naming.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   c.opts.Group,
		HealthyOnly: true,
	})
}

// Subscribe wires a callback that fires whenever the instance list of
// serviceName changes. The callback receives the new instance list.
func (c *Client) Subscribe(serviceName string, cb func([]model.Instance, error)) error {
	return c.naming.Subscribe(&vo.SubscribeParam{
		ServiceName: serviceName,
		GroupName:   c.opts.Group,
		SubscribeCallback: func(services []model.Instance, err error) {
			cb(services, err)
		},
	})
}

// GetConfig fetches the current value of a config item.
func (c *Client) GetConfig(dataID string) (string, error) {
	return c.config.GetConfig(vo.ConfigParam{
		DataId: dataID,
		Group:  c.opts.Group,
	})
}

// PublishConfig writes a config item, creating it if missing.
func (c *Client) PublishConfig(dataID, content string) (bool, error) {
	return c.config.PublishConfig(vo.ConfigParam{
		DataId:  dataID,
		Group:   c.opts.Group,
		Content: content,
	})
}

// ListenConfig fires onChange whenever dataID is updated in Nacos.
func (c *Client) ListenConfig(dataID string, onChange func(content string)) error {
	return c.config.ListenConfig(vo.ConfigParam{
		DataId: dataID,
		Group:  c.opts.Group,
		OnChange: func(_, _, _, data string) {
			onChange(data)
		},
	})
}

func detectLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if v4 := ipNet.IP.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", errors.New("no non-loopback IPv4 found")
}
