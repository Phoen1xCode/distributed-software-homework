package registry

import (
	"flash-sale/pkg/config"
	"log"
)

// BootstrapService is a convenience wrapper that the three microservices call
// in main() to register with Nacos. It is a no-op when Nacos is disabled.
// Returns the client (may be nil if disabled or unreachable) and a deregister
// function the caller should defer.
func BootstrapService(cfg *config.Config) (*Client, func()) {
	noop := func() {}
	if !cfg.Nacos.Enabled {
		return nil, noop
	}
	if cfg.Nacos.ServiceName == "" {
		log.Printf("[registry] nacos enabled but service_name is empty; skipping registration")
		return nil, noop
	}
	c, err := New(Options{
		Host:         cfg.Nacos.Host,
		Port:         cfg.Nacos.Port,
		Namespace:    cfg.Nacos.Namespace,
		Group:        cfg.Nacos.Group,
		ServiceName:  cfg.Nacos.ServiceName,
		InstanceIP:   cfg.Nacos.InstanceIP,
		InstancePort: uint64(cfg.Server.Port),
		DataID:       cfg.Nacos.DataID,
	})
	if err != nil {
		log.Printf("[registry] failed to construct nacos client: %v (continuing without registry)", err)
		return nil, noop
	}
	if err := c.Register(); err != nil {
		log.Printf("[registry] failed to register with nacos: %v (continuing)", err)
		return c, noop
	}
	log.Printf("[registry] registered %s on port %d with nacos@%s:%d", cfg.Nacos.ServiceName, cfg.Server.Port, cfg.Nacos.Host, cfg.Nacos.Port)
	return c, func() {
		if err := c.Deregister(); err != nil {
			log.Printf("[registry] deregister: %v", err)
		}
	}
}
