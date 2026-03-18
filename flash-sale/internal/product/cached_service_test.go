package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// mockProductService is a test double that implements ProductServicer.
type mockProductService struct {
	getByIDFunc  func(id uint) (*Product, error)
	getByIDCount atomic.Int32
	createFunc   func(req *CreateProductRequest) (*Product, error)
	getByNameFunc func(name string) (*Product, error)
	listFunc     func(page, pageSize int) ([]Product, int64, error)
	updateFunc   func(id uint, req *UpdateProductRequest) (*Product, error)
	deleteFunc   func(id uint) error
	getPriceFunc func(id uint) (float64, error)
}

func (m *mockProductService) GetProductByID(id uint) (*Product, error) {
	m.getByIDCount.Add(1)
	if m.getByIDFunc != nil {
		return m.getByIDFunc(id)
	}
	return nil, ErrProductNotFound
}

func (m *mockProductService) CreateProduct(req *CreateProductRequest) (*Product, error) {
	if m.createFunc != nil {
		return m.createFunc(req)
	}
	return nil, nil
}

func (m *mockProductService) GetProductByName(name string) (*Product, error) {
	if m.getByNameFunc != nil {
		return m.getByNameFunc(name)
	}
	return nil, ErrProductNotFound
}

func (m *mockProductService) ListProducts(page, pageSize int) ([]Product, int64, error) {
	if m.listFunc != nil {
		return m.listFunc(page, pageSize)
	}
	return nil, 0, nil
}

func (m *mockProductService) Update(id uint, req *UpdateProductRequest) (*Product, error) {
	if m.updateFunc != nil {
		return m.updateFunc(id, req)
	}
	return nil, nil
}

func (m *mockProductService) Delete(id uint) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(id)
	}
	return nil
}

func (m *mockProductService) GetPrice(id uint) (float64, error) {
	if m.getPriceFunc != nil {
		return m.getPriceFunc(id)
	}
	return 0, nil
}

func setupTest(t *testing.T) (*miniredis.Miniredis, *redis.Client, *mockProductService) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mock := &mockProductService{}
	return mr, rc, mock
}

var sampleProduct = &Product{
	ID:       1,
	Name:     "Flash Sale Phone",
	Price:    999.99,
	Category: "electronics",
	Status:   1,
}

func TestCacheHit(t *testing.T) {
	mr, rc, mock := setupTest(t)
	_ = mr
	svc := NewCachedService(mock, rc)

	// Pre-populate Redis with cached product
	data, _ := json.Marshal(sampleProduct)
	rc.Set(context.Background(), "product:detail:1", data, 5*time.Minute)

	got, err := svc.GetProductByID(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != sampleProduct.ID || got.Name != sampleProduct.Name {
		t.Fatalf("got %+v, want %+v", got, sampleProduct)
	}
	if mock.getByIDCount.Load() != 0 {
		t.Fatalf("inner service should NOT be called on cache hit, called %d times", mock.getByIDCount.Load())
	}
}

func TestCacheMiss_ThenFill(t *testing.T) {
	_, rc, mock := setupTest(t)
	mock.getByIDFunc = func(id uint) (*Product, error) {
		return sampleProduct, nil
	}
	svc := NewCachedService(mock, rc)

	got, err := svc.GetProductByID(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != sampleProduct.ID {
		t.Fatalf("got %+v, want %+v", got, sampleProduct)
	}
	if mock.getByIDCount.Load() != 1 {
		t.Fatalf("inner service should be called once, called %d times", mock.getByIDCount.Load())
	}

	// Verify data was written to Redis
	val, err := rc.Get(context.Background(), "product:detail:1").Result()
	if err != nil {
		t.Fatalf("expected cache to be filled, got error: %v", err)
	}
	var cached Product
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		t.Fatalf("failed to unmarshal cached value: %v", err)
	}
	if cached.ID != sampleProduct.ID {
		t.Fatalf("cached product ID = %d, want %d", cached.ID, sampleProduct.ID)
	}
}

func TestCachePenetration(t *testing.T) {
	_, rc, mock := setupTest(t)
	mock.getByIDFunc = func(id uint) (*Product, error) {
		return nil, ErrProductNotFound
	}
	svc := NewCachedService(mock, rc)

	// First call: should query inner service, cache empty marker
	_, err := svc.GetProductByID(999)
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got: %v", err)
	}
	if mock.getByIDCount.Load() != 1 {
		t.Fatalf("first call should hit inner service, called %d times", mock.getByIDCount.Load())
	}

	// Verify empty marker exists in Redis
	val, err := rc.Get(context.Background(), "product:detail:999").Result()
	if err != nil {
		t.Fatalf("expected empty marker in cache, got error: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty string marker, got: %q", val)
	}

	// Second call: should NOT query inner service (penetration protection)
	_, err = svc.GetProductByID(999)
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got: %v", err)
	}
	if mock.getByIDCount.Load() != 1 {
		t.Fatalf("second call should NOT hit inner service, called %d times", mock.getByIDCount.Load())
	}
}

func TestCacheBreakdown(t *testing.T) {
	_, rc, mock := setupTest(t)
	mock.getByIDFunc = func(id uint) (*Product, error) {
		time.Sleep(50 * time.Millisecond) // simulate DB latency
		return sampleProduct, nil
	}
	svc := NewCachedService(mock, rc)

	// Launch 10 concurrent requests for the same product
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.GetProductByID(1)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d got error: %v", i, err)
		}
	}

	// singleflight should ensure inner service called exactly once
	count := mock.getByIDCount.Load()
	if count != 1 {
		t.Fatalf("expected inner service called 1 time (singleflight), got %d", count)
	}
}

func TestCacheAvalanche(t *testing.T) {
	mr, rc, mock := setupTest(t)
	mock.getByIDFunc = func(id uint) (*Product, error) {
		return &Product{ID: id, Name: "Product", Price: 10.0, Status: 1}, nil
	}
	svc := NewCachedService(mock, rc)

	// Cache multiple products
	for i := uint(1); i <= 20; i++ {
		_, _ = svc.GetProductByID(i)
	}

	// Collect TTLs from miniredis
	ttls := make(map[time.Duration]bool)
	for i := uint(1); i <= 20; i++ {
		key := "product:detail:" + fmt.Sprintf("%d", i)
		ttl := mr.TTL(key)
		ttls[ttl] = true
	}

	// With random offset, not all TTLs should be identical
	if len(ttls) < 2 {
		t.Fatalf("expected varied TTLs (avalanche protection), but all %d keys have the same TTL", 20)
	}
}

func TestCacheInvalidation_Update(t *testing.T) {
	_, rc, mock := setupTest(t)
	svc := NewCachedService(mock, rc)

	// Pre-populate cache
	data, _ := json.Marshal(sampleProduct)
	rc.Set(context.Background(), "product:detail:1", data, 5*time.Minute)

	// Update product — mock returns updated product
	status := int16(0)
	mock.updateFunc = func(id uint, req *UpdateProductRequest) (*Product, error) {
		p := *sampleProduct
		p.Status = *req.Status
		return &p, nil
	}
	_, err := svc.Update(1, &UpdateProductRequest{Status: &status})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cache should be deleted
	exists, _ := rc.Exists(context.Background(), "product:detail:1").Result()
	if exists != 0 {
		t.Fatalf("cache should be invalidated after update")
	}
}

func TestCacheInvalidation_Delete(t *testing.T) {
	_, rc, mock := setupTest(t)
	svc := NewCachedService(mock, rc)

	// Pre-populate cache
	data, _ := json.Marshal(sampleProduct)
	rc.Set(context.Background(), "product:detail:1", data, 5*time.Minute)

	mock.deleteFunc = func(id uint) error { return nil }
	err := svc.Delete(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exists, _ := rc.Exists(context.Background(), "product:detail:1").Result()
	if exists != 0 {
		t.Fatalf("cache should be invalidated after delete")
	}
}

func TestRedisDown_FallsBackToDB(t *testing.T) {
	mr, _, mock := setupTest(t)
	// Create client pointing to miniredis, then shut it down
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mr.Close()

	mock.getByIDFunc = func(id uint) (*Product, error) {
		return sampleProduct, nil
	}
	svc := NewCachedService(mock, rc)

	got, err := svc.GetProductByID(1)
	if err != nil {
		t.Fatalf("should fall back to DB when Redis is down, got error: %v", err)
	}
	if got.ID != sampleProduct.ID {
		t.Fatalf("got %+v, want %+v", got, sampleProduct)
	}
	if mock.getByIDCount.Load() != 1 {
		t.Fatalf("should call inner service when Redis is down, called %d times", mock.getByIDCount.Load())
	}
}
