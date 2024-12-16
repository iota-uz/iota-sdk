package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.57

import (
	"context"
	"fmt"

	model "github.com/iota-agency/iota-sdk/pkg/interfaces/graph/gqlmodels"
)

// CreatePayment is the resolver for the createPayment field.
func (r *mutationResolver) CreatePayment(ctx context.Context, input model.CreatePayment) (*model.Payment, error) {
	panic(fmt.Errorf("not implemented: CreatePayment - createPayment"))
}

// UpdatePayment is the resolver for the updatePayment field.
func (r *mutationResolver) UpdatePayment(ctx context.Context, id int64, input model.UpdatePayment) (*model.Payment, error) {
	panic(fmt.Errorf("not implemented: UpdatePayment - updatePayment"))
}

// DeletePayment is the resolver for the deletePayment field.
func (r *mutationResolver) DeletePayment(ctx context.Context, id int64) (bool, error) {
	panic(fmt.Errorf("not implemented: DeletePayment - deletePayment"))
}

// Payment is the resolver for the payment field.
func (r *queryResolver) Payment(ctx context.Context, id int64) (*model.Payment, error) {
	panic(fmt.Errorf("not implemented: Payment - payment"))
}

// Payments is the resolver for the payments field.
func (r *queryResolver) Payments(ctx context.Context, offset int, limit int, sortBy []string) (*model.PaginatedPayments, error) {
	panic(fmt.Errorf("not implemented: Payments - payments"))
}

// PaymentCreated is the resolver for the paymentCreated field.
func (r *subscriptionResolver) PaymentCreated(ctx context.Context) (<-chan *model.Payment, error) {
	panic(fmt.Errorf("not implemented: PaymentCreated - paymentCreated"))
}

// PaymentUpdated is the resolver for the paymentUpdated field.
func (r *subscriptionResolver) PaymentUpdated(ctx context.Context) (<-chan *model.Payment, error) {
	panic(fmt.Errorf("not implemented: PaymentUpdated - paymentUpdated"))
}

// PaymentDeleted is the resolver for the paymentDeleted field.
func (r *subscriptionResolver) PaymentDeleted(ctx context.Context) (<-chan int64, error) {
	panic(fmt.Errorf("not implemented: PaymentDeleted - paymentDeleted"))
}
