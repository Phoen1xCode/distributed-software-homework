// Gateway is the HW6 API entrypoint. It discovers backend microservices via
// Nacos, proxies HTTP traffic to them, and applies sentinel-golang based flow
// control / circuit breaking / fallback (graceful degrade).
package main

import (
	"context"
	"flash-sale/pkg/config"
	"flash-sale/pkg/registry"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	cb "github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/gin-gonic/gin"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
)

// route is the resolved view of a config.RouteConfig.
type route struct {
	prefix      string
	serviceName string
	resource    string
}

// instancePool is the set of healthy upstream instances for one service.
// It is updated by the Nacos subscription callback and by a periodic poll as a
// safety net (in case the subscription drops a frame).
type instancePool struct {
	mu        sync.RWMutex
	instances []model.Instance
}

func (p *instancePool) set(insts []model.Instance) {
	p.mu.Lock()
	defer p.mu.Unlock()
	healthy := make([]model.Instance, 0, len(insts))
	for _, in := range insts {
		if in.Healthy && in.Enable {
			healthy = append(healthy, in)
		}
	}
	p.instances = healthy
}

func (p *instancePool) pick() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.instances) == 0 {
		return "", false
	}
	in := p.instances[rand.Intn(len(p.instances))]
	return fmt.Sprintf("http://%s:%d", in.Ip, in.Port), true
}

func main() {
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("gateway: load config: %v", err)
	}

	// 1. Sentinel: flow + circuit breaker rules ---------------------------------
	if err := sentinel.InitDefault(); err != nil {
		log.Fatalf("gateway: sentinel init: %v", err)
	}
	if err := loadSentinelRules(cfg); err != nil {
		log.Fatalf("gateway: sentinel rules: %v", err)
	}

	// 2. Nacos: register self + discover backends -------------------------------
	nacosClient, deregister := registry.BootstrapService(cfg)
	defer deregister()
	if nacosClient == nil {
		log.Println("gateway: nacos disabled, falling back to docker DNS routing")
	}

	pools := make(map[string]*instancePool)
	for _, r := range cfg.Gateway.Routes {
		pools[r.ServiceName] = &instancePool{}
	}

	if nacosClient != nil {
		for svc, pool := range pools {
			if err := primePool(nacosClient, svc, pool); err != nil {
				log.Printf("gateway: prime %s: %v", svc, err)
			}
			svc, pool := svc, pool
			if err := nacosClient.Subscribe(svc, func(insts []model.Instance, subErr error) {
				if subErr != nil {
					log.Printf("gateway: subscribe %s callback: %v", svc, subErr)
					return
				}
				pool.set(insts)
				log.Printf("gateway: %s instances updated -> %d healthy", svc, len(insts))
			}); err != nil {
				log.Printf("gateway: subscribe %s: %v", svc, err)
			}
		}
		// safety-net poller
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pollLoop(ctx, nacosClient, pools, refreshInterval(cfg))
	}

	// 3. HTTP router with longest-prefix routing --------------------------------
	routes := buildRoutes(cfg)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(accessLog())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "gateway"})
	})
	r.GET("/gateway/instances", func(c *gin.Context) {
		out := make(map[string][]string)
		for svc, p := range pools {
			p.mu.RLock()
			for _, in := range p.instances {
				out[svc] = append(out[svc], fmt.Sprintf("%s:%d", in.Ip, in.Port))
			}
			p.mu.RUnlock()
		}
		c.JSON(200, out)
	})

	r.NoRoute(func(c *gin.Context) {
		matched, ok := matchRoute(routes, c.Request.URL.Path)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "no route for " + c.Request.URL.Path})
			return
		}
		handleProxy(c, matched, pools[matched.serviceName])
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("gateway: listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("gateway: run: %v", err)
	}
}

// buildRoutes sorts routes by descending prefix length so seckill (longer)
// wins over orders/products (shorter) at match time.
func buildRoutes(cfg *config.Config) []route {
	out := make([]route, 0, len(cfg.Gateway.Routes))
	for _, r := range cfg.Gateway.Routes {
		res := r.Resource
		if res == "" {
			res = "route:" + r.ServiceName
		}
		out = append(out, route{prefix: r.PathPrefix, serviceName: r.ServiceName, resource: res})
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i].prefix) > len(out[j].prefix) })
	return out
}

func matchRoute(routes []route, path string) (route, bool) {
	for _, r := range routes {
		if strings.HasPrefix(path, r.prefix) {
			return r, true
		}
	}
	return route{}, false
}

// handleProxy: sentinel.Entry -> pick instance -> reverse proxy.
// Errors / blocks degrade to a friendly 503 JSON envelope.
func handleProxy(c *gin.Context, r route, pool *instancePool) {
	entry, blockErr := sentinel.Entry(r.resource, sentinel.WithTrafficType(base.Inbound))
	if blockErr != nil {
		// Either rate limited or circuit open -> friendly fallback.
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":     503,
			"message":  "service degraded, please retry later",
			"resource": r.resource,
			"reason":   blockErr.BlockType().String(),
		})
		return
	}
	defer entry.Exit()

	target, ok := pool.pick()
	if !ok {
		// No healthy instance -> count as error so the breaker can open.
		sentinel.TraceError(entry, fmt.Errorf("no healthy instance for %s", r.serviceName))
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "no upstream available"})
		return
	}

	u, err := url.Parse(target)
	if err != nil {
		sentinel.TraceError(entry, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, perr error) {
		sentinel.TraceError(entry, perr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":502,"message":"upstream error"}`))
	}
	// Capture HTTP 5xx as breaker errors so degrade kicks in for downstream
	// failures, not just transport-level errors.
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode >= 500 {
			sentinel.TraceError(entry, fmt.Errorf("upstream %d", resp.StatusCode))
		}
		return nil
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func primePool(client *registry.Client, svc string, pool *instancePool) error {
	insts, err := client.SelectInstances(svc)
	if err != nil {
		return err
	}
	pool.set(insts)
	log.Printf("gateway: %s primed with %d instance(s)", svc, len(insts))
	return nil
}

func pollLoop(ctx context.Context, client *registry.Client, pools map[string]*instancePool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for svc, pool := range pools {
				if insts, err := client.SelectInstances(svc); err == nil {
					pool.set(insts)
				}
			}
		}
	}
}

func refreshInterval(cfg *config.Config) time.Duration {
	if cfg.Gateway.RefreshSecMs <= 0 {
		return 5 * time.Second
	}
	return time.Duration(cfg.Gateway.RefreshSecMs) * time.Millisecond
}

func loadSentinelRules(cfg *config.Config) error {
	s := cfg.Gateway.Sentinel
	if s.SeckillQPS == 0 {
		s.SeckillQPS = 100
	}
	if s.DefaultQPS == 0 {
		s.DefaultQPS = 1000
	}
	if s.BreakerStatIntervalMs == 0 {
		s.BreakerStatIntervalMs = 10000
	}
	if s.BreakerRetryTimeoutMs == 0 {
		s.BreakerRetryTimeoutMs = 5000
	}
	if s.BreakerMinRequests == 0 {
		s.BreakerMinRequests = 10
	}
	if s.SeckillBreakerErrorRate == 0 {
		s.SeckillBreakerErrorRate = 0.5
	}

	flowRules := []*flow.Rule{
		{
			Resource:               "route:seckill",
			Threshold:              s.SeckillQPS,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			StatIntervalInMs:       1000,
		},
		{
			Resource:               "route:default",
			Threshold:              s.DefaultQPS,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			StatIntervalInMs:       1000,
		},
	}
	// Apply default flow rule to non-seckill routes too.
	for _, r := range cfg.Gateway.Routes {
		if r.Resource == "" || r.Resource == "route:seckill" {
			continue
		}
		flowRules = append(flowRules, &flow.Rule{
			Resource:               r.Resource,
			Threshold:              s.DefaultQPS,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			StatIntervalInMs:       1000,
		})
	}
	if _, err := flow.LoadRules(flowRules); err != nil {
		return fmt.Errorf("load flow rules: %w", err)
	}

	cbRules := []*cb.Rule{
		{
			Resource:         "route:seckill",
			Strategy:         cb.ErrorRatio,
			RetryTimeoutMs:   s.BreakerRetryTimeoutMs,
			MinRequestAmount: s.BreakerMinRequests,
			StatIntervalMs:   s.BreakerStatIntervalMs,
			Threshold:        s.SeckillBreakerErrorRate,
		},
	}
	if _, err := cb.LoadRules(cbRules); err != nil {
		return fmt.Errorf("load cb rules: %w", err)
	}
	return nil
}

func accessLog() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] [gateway] %s %s -> %d (%s)\n",
			p.TimeStamp.Format(time.RFC3339),
			p.Method, p.Path, p.StatusCode, p.Latency,
		)
	})
}
