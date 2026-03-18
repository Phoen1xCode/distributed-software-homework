package product

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

const (
	cacheKeyPrefix = "product:detail:"
	cacheTTLBase   = 5 * time.Minute
	cacheTTLJitter = 60 // seconds
	cacheNullTTL   = 60 * time.Second
	cacheNullValue = ""
)

var _ ProductServicer = (*CachedProductService)(nil)

type CachedProductService struct {
	inner ProductServicer
	rdb   *redis.Client
	group singleflight.Group
}

func NewCachedService(inner ProductServicer, rdb *redis.Client) *CachedProductService {
	return &CachedProductService{
		inner: inner,
		rdb:   rdb,
	}
}

func cacheKey(id uint) string {
	return fmt.Sprintf("%s%d", cacheKeyPrefix, id)
}

func cacheTTL() time.Duration {
	return cacheTTLBase + time.Duration(rand.Intn(cacheTTLJitter))*time.Second
}

// GetProductByID implements Cache-Aside with penetration, breakdown, and avalanche protection.
func (s *CachedProductService) GetProductByID(id uint) (*Product, error) {
	ctx := context.Background()
	key := cacheKey(id)

	// 1. Check Redis (graceful degradation: Redis error = cache miss)
	val, err := s.rdb.Get(ctx, key).Result()
	if err == nil {
		// Cache hit
		if val == cacheNullValue {
			// Penetration protection: cached empty marker
			return nil, ErrProductNotFound
		}
		var p Product
		if err := json.Unmarshal([]byte(val), &p); err == nil {
			return &p, nil
		}
		// Unmarshal failed — treat as cache miss, fall through
	} else if err != redis.Nil {
		// Redis error (not a miss) — log and fall through to DB
		log.Printf("[WARN] Redis GET %s failed: %v", key, err)
	}

	// 2. Cache miss — use singleflight to prevent breakdown
	result, err, _ := s.group.Do(key, func() (interface{}, error) {
		p, err := s.inner.GetProductByID(id)
		if err != nil {
			if err == ErrProductNotFound {
				// Penetration protection: cache empty marker with short TTL
				if setErr := s.rdb.Set(ctx, key, cacheNullValue, cacheNullTTL).Err(); setErr != nil {
					log.Printf("[WARN] Redis SET null %s failed: %v", key, setErr)
				}
			}
			return nil, err
		}

		// Backfill cache with randomized TTL (avalanche protection)
		data, _ := json.Marshal(p)
		if setErr := s.rdb.Set(ctx, key, data, cacheTTL()).Err(); setErr != nil {
			log.Printf("[WARN] Redis SET %s failed: %v", key, setErr)
		}

		return p, nil
	})
	if err != nil {
		return nil, err
	}

	return result.(*Product), nil
}

// Update delegates to inner service, then invalidates cache.
func (s *CachedProductService) Update(id uint, req *UpdateProductRequest) (*Product, error) {
	p, err := s.inner.Update(id, req)
	if err != nil {
		return nil, err
	}

	// Invalidate cache (graceful: ignore Redis errors)
	if delErr := s.rdb.Del(context.Background(), cacheKey(id)).Err(); delErr != nil {
		log.Printf("[WARN] Redis DEL %s failed: %v", cacheKey(id), delErr)
	}

	return p, nil
}

// Delete delegates to inner service, then invalidates cache.
func (s *CachedProductService) Delete(id uint) error {
	if err := s.inner.Delete(id); err != nil {
		return err
	}

	if delErr := s.rdb.Del(context.Background(), cacheKey(id)).Err(); delErr != nil {
		log.Printf("[WARN] Redis DEL %s failed: %v", cacheKey(id), delErr)
	}

	return nil
}

// --- Pass-through methods (no cache logic) ---

func (s *CachedProductService) CreateProduct(req *CreateProductRequest) (*Product, error) {
	return s.inner.CreateProduct(req)
}

func (s *CachedProductService) GetProductByName(name string) (*Product, error) {
	return s.inner.GetProductByName(name)
}

func (s *CachedProductService) ListProducts(page, pageSize int) ([]Product, int64, error) {
	return s.inner.ListProducts(page, pageSize)
}

func (s *CachedProductService) GetPrice(id uint) (float64, error) {
	return s.inner.GetPrice(id)
}
