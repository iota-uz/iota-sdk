package services

import (
	"context"
	"github.com/iota-agency/iota-sdk/elxolding/domain/entities/dashboard"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/domain/entities/position"
	"sync"
)

type DashboardService struct {
	positionRepo position.Repository
	productRepo  product.Repository
	orderRepo    order.Repository
}

func NewDashboardService(
	positionRepo position.Repository,
	productRepo product.Repository,
	orderRepo order.Repository,
) *DashboardService {
	return &DashboardService{
		positionRepo: positionRepo,
		productRepo:  productRepo,
		orderRepo:    orderRepo,
	}
}

func (s *DashboardService) GetStats(ctx context.Context) (*dashboard.Stats, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var positionsCount, productsCount, ordersCount int64
	var err error

	wg.Add(3)

	go func() {
		defer wg.Done()
		count, e := s.positionRepo.Count(ctx)
		mu.Lock()
		defer mu.Unlock()
		if e != nil {
			err = e
			return
		}
		positionsCount = count
	}()

	go func() {
		defer wg.Done()
		count, e := s.productRepo.Count(ctx)
		mu.Lock()
		defer mu.Unlock()
		if e != nil {
			err = e
			return
		}
		productsCount = count
	}()

	go func() {
		defer wg.Done()
		count, e := s.orderRepo.Count(ctx)
		mu.Lock()
		defer mu.Unlock()
		if e != nil {
			err = e
			return
		}
		ordersCount = count
	}()

	wg.Wait()

	if err != nil {
		return nil, err
	}

	var depth float64
	if positionsCount > 0 {
		depth = float64(productsCount) / float64(positionsCount)
	} else {
		depth = 0
	}

	return &dashboard.Stats{
		PositionsCount: positionsCount,
		ProductsCount:  productsCount,
		Depth:          depth,
		OrdersCount:    ordersCount,
	}, nil
}
