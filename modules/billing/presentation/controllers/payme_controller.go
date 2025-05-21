package controllers

import (
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	paymeapi "github.com/iota-uz/payme"
	paymeauth "github.com/iota-uz/payme/auth"
	"log"
	"math"
	"net/http"
	"time"
)

type PaymeController struct {
	app            application.Application
	billingService *services.BillingService
	payme          configuration.PaymeOptions
	basePath       string
}

func NewPaymeController(app application.Application, payme configuration.PaymeOptions, basePath string) application.Controller {
	return &PaymeController{
		app:            app,
		billingService: app.Service(services.BillingService{}).(*services.BillingService),
		payme:          payme,
		basePath:       basePath,
	}
}

func (c *PaymeController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.HandleFunc("", c.Handle)
}

func (c *PaymeController) Key() string {
	return c.basePath
}

func (c *PaymeController) Handle(w http.ResponseWriter, r *http.Request) {
	var (
		req             paymeapi.JSONRPCRequest
		successResponse *paymeapi.JSONRPCSuccessResponse
		errorResponse   *paymeapi.JSONRPCErrorResponse
	)

	if err := paymeauth.ValidateBasicAuth(r, c.payme.User, c.payme.SecretKey); err != nil {
		errorResponse = &paymeapi.JSONRPCErrorResponse{
			Error: paymeapi.InsufficientPrivilegesError(),
		}
	}
	if errorResponse == nil && r.Method != http.MethodPost {
		errorResponse = &paymeapi.JSONRPCErrorResponse{
			Error: paymeapi.MethodNotPOSTError(),
		}
	}

	if errorResponse == nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse = &paymeapi.JSONRPCErrorResponse{
				Error: paymeapi.JSONParseError(),
			}
		}
	}
	if errorResponse == nil {
		switch {
		case req.CancelTransactionRequestWrapper != nil:
			cancelReq := req.CancelTransactionRequestWrapper
			resp, errRPC := c.cancel(r.Context(), &cancelReq.Params)
			if errRPC != nil {
				errorResponse = &paymeapi.JSONRPCErrorResponse{
					Id:    cancelReq.Id,
					Error: *errRPC,
				}
			} else {
				successResponse = &paymeapi.JSONRPCSuccessResponse{
					Id: cancelReq.Id,
					Result: paymeapi.JSONRPCSuccessResponseResult{
						CancelTransactionResponse: resp,
					},
				}
			}
		case req.CheckPerformTransactionRequestWrapper != nil:
			checkPerformReq := req.CheckPerformTransactionRequestWrapper
			resp, errRPC := c.checkPerform(r.Context(), &checkPerformReq.Params)
			if errRPC != nil {
				errorResponse = &paymeapi.JSONRPCErrorResponse{
					Id:    checkPerformReq.Id,
					Error: *errRPC,
				}
			} else {
				successResponse = &paymeapi.JSONRPCSuccessResponse{
					Id: checkPerformReq.Id,
					Result: paymeapi.JSONRPCSuccessResponseResult{
						CheckPerformTransactionResponse: resp,
					},
				}
			}
		case req.CheckTransactionRequestWrapper != nil:
			checkReq := req.CheckTransactionRequestWrapper
			resp, errRPC := c.check(r.Context(), &checkReq.Params)
			if errRPC != nil {
				errorResponse = &paymeapi.JSONRPCErrorResponse{
					Id:    checkReq.Id,
					Error: *errRPC,
				}
			} else {
				successResponse = &paymeapi.JSONRPCSuccessResponse{
					Id: checkReq.Id,
					Result: paymeapi.JSONRPCSuccessResponseResult{
						CheckTransactionResponse: resp,
					},
				}
			}
		case req.CreateTransactionRequestWrapper != nil:
			createReq := req.CreateTransactionRequestWrapper
			resp, errRPC := c.create(r.Context(), &createReq.Params)
			if errRPC != nil {
				errorResponse = &paymeapi.JSONRPCErrorResponse{
					Id:    createReq.Id,
					Error: *errRPC,
				}
			} else {
				successResponse = &paymeapi.JSONRPCSuccessResponse{
					Id: createReq.Id,
					Result: paymeapi.JSONRPCSuccessResponseResult{
						CreateTransactionResponse: resp,
					},
				}
			}
		case req.PerformTransactionRequestWrapper != nil:
			performReq := req.PerformTransactionRequestWrapper
			resp, errRPC := c.perform(r.Context(), &performReq.Params)
			if errRPC != nil {
				errorResponse = &paymeapi.JSONRPCErrorResponse{
					Id:    performReq.Id,
					Error: *errRPC,
				}
			} else {
				successResponse = &paymeapi.JSONRPCSuccessResponse{
					Id: performReq.Id,
					Result: paymeapi.JSONRPCSuccessResponseResult{
						PerformTransactionResponse: resp,
					},
				}
			}
		case req.GetStatementRequestWrapper != nil:
			getStatementReq := req.GetStatementRequestWrapper
			resp, errRPC := c.getStatement(r.Context(), &getStatementReq.Params)
			if errRPC != nil {
				errorResponse = &paymeapi.JSONRPCErrorResponse{
					Id:    getStatementReq.Id,
					Error: *errRPC,
				}
			} else {
				successResponse = &paymeapi.JSONRPCSuccessResponse{
					Id: getStatementReq.Id,
					Result: paymeapi.JSONRPCSuccessResponseResult{
						GetStatementResponse: resp,
					},
				}
			}

		default:
			errorResponse = &paymeapi.JSONRPCErrorResponse{
				Error: paymeapi.MethodNotFoundInDataError(),
			}
		}
	}

	if errorResponse != nil {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
		return
	}

	if successResponse != nil {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(successResponse); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
		return
	}
}

func (c *PaymeController) check(ctx context.Context, r *paymeapi.CheckTransactionRequest) (*paymeapi.CheckTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
	entities, err := c.billingService.GetByDetailsFields(
		ctx,
		billing.Payme,
		[]billing.DetailsFieldFilter{
			{
				Path:     []string{"id"},
				Operator: billing.OpEqual,
				Value:    r.Id,
			},
		},
	)
	if err != nil || len(entities) != 1 {
		errRPC := paymeapi.CheckTransactionTransactionNotFoundError()
		return nil, &errRPC
	}
	entity := entities[0]

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	resp := paymeapi.CheckTransactionResponse{
		CreateTime:  paymeDetails.CreatedTime(),
		PerformTime: paymeDetails.PerformTime(),
		CancelTime:  paymeDetails.CancelTime(),
		Transaction: paymeDetails.Transaction(),
		State:       paymeDetails.State(),
		Reason:      paymeapi.NullableInt32{},
	}

	if paymeDetails.Reason() != 0 {
		reason := paymeDetails.Reason()
		resp.Reason.Set(&reason)
	}

	return &resp, nil
}

func (c *PaymeController) create(ctx context.Context, r *paymeapi.CreateTransactionRequest) (*paymeapi.CreateTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
	filters := make([]billing.DetailsFieldFilter, 0, len(r.Account))
	for k, v := range r.Account {
		filters = append(filters, billing.DetailsFieldFilter{
			Path:     []string{"account", k},
			Operator: billing.OpEqual,
			Value:    v,
		})
	}
	entities, err := c.billingService.GetByDetailsFields(
		ctx,
		billing.Payme,
		filters,
	)
	if err != nil || len(entities) != 1 {
		errRPC := paymeapi.InvalidAccountError()
		return nil, &errRPC
	}

	entity := entities[0]

	if math.Abs(entity.Amount().Quantity()-r.Amount) >= 1e-9 {
		errRPC := paymeapi.InvalidAmountError()
		return nil, &errRPC
	}

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	if paymeDetails.ID() != "" && paymeDetails.ID() != r.Id {
		errRPC := paymeapi.InvalidAccountError()
		return nil, &errRPC
	}

	paymeDetails = paymeDetails.
		SetID(r.Id).
		SetTime(r.Time).
		SetAccount(r.Account)

	if paymeDetails.CreatedTime() == 0 {
		paymeDetails = paymeDetails.SetCreatedTime(time.Now().UnixMilli())
	}

	entity = entity.
		SetDetails(paymeDetails)

	entity, err = c.billingService.Update(ctx, entity)
	if err != nil {
		log.Printf("Failed to update transaction: %v", err)
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	paymeDetails, ok = entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	resp := &paymeapi.CreateTransactionResponse{
		CreateTime:  paymeDetails.CreatedTime(),
		Transaction: paymeDetails.Transaction(),
		State:       paymeDetails.State(),
	}

	if len(paymeDetails.Receivers()) > 0 {
		receivers := make([]paymeapi.Receiver, 0, len(paymeDetails.Receivers()))
		for _, r := range paymeDetails.Receivers() {
			receivers = append(receivers, paymeapi.Receiver{
				Id:     r.ID(),
				Amount: r.Amount(),
			})
		}
		resp.Receivers = receivers
	}

	return resp, nil
}

func (c *PaymeController) cancel(ctx context.Context, r *paymeapi.CancelTransactionRequest) (*paymeapi.CancelTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
	entities, err := c.billingService.GetByDetailsFields(
		ctx,
		billing.Payme,
		[]billing.DetailsFieldFilter{
			{
				Path:     []string{"id"},
				Operator: billing.OpEqual,
				Value:    r.Id,
			},
		},
	)
	if err != nil || len(entities) != 1 {
		errRPC := paymeapi.CancelTransactionTransactionNotFoundError()
		return nil, &errRPC
	}

	entity := entities[0]

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	if paymeDetails.CancelTime() == 0 {
		paymeDetails = paymeDetails.SetCancelTime(time.Now().UnixMilli())
	}

	paymeDetails = paymeDetails.SetReason(r.Reason)

	switch paymeDetails.State() {
	case paymeapi.TransactionStateCreated:
		paymeDetails = paymeDetails.SetState(paymeapi.TransactionStateCancelledBeforeCompletion)
		entity = entity.SetStatus(billing.Canceled)
	case paymeapi.TransactionStateCompleted:
		paymeDetails = paymeDetails.SetState(paymeapi.TransactionStateCancelledAfterCompletion)
		entity = entity.SetStatus(billing.Refunded)
	}

	entity = entity.SetDetails(paymeDetails)

	entity, err = c.billingService.Update(ctx, entity)
	if err != nil {
		log.Printf("Failed to update transaction: %v", err)
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	paymeDetails, ok = entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	return &paymeapi.CancelTransactionResponse{
		Transaction: paymeDetails.Transaction(),
		CancelTime:  paymeDetails.CancelTime(),
		State:       paymeDetails.State(),
	}, nil
}

func (c *PaymeController) checkPerform(ctx context.Context, r *paymeapi.CheckPerformTransactionRequest) (*paymeapi.CheckPerformTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
	filters := make([]billing.DetailsFieldFilter, 0, len(r.Account))
	for k, v := range r.Account {
		filters = append(filters, billing.DetailsFieldFilter{
			Path:     []string{"account", k},
			Operator: billing.OpEqual,
			Value:    v,
		})
	}
	entities, err := c.billingService.GetByDetailsFields(
		ctx,
		billing.Payme,
		filters,
	)
	if err != nil || len(entities) != 1 {
		errRPC := paymeapi.CheckPerformTransactionInvalidAccountError()
		return nil, &errRPC
	}

	entity := entities[0]

	if math.Abs(entity.Amount().Quantity()-r.Amount) >= 1e-9 {
		errRPC := paymeapi.CheckPerformTransactionInvalidAmountError()
		return nil, &errRPC
	}

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	if paymeDetails.State() != paymeapi.TransactionStateCreated {
		errRPC := paymeapi.InvalidAccountError()
		return nil, &errRPC
	}

	if entity.Status() == billing.Created {
		entity = entity.SetStatus(billing.Pending)
	}

	entity = entity.
		SetDetails(
			paymeDetails.
				SetAccount(r.Account),
		)

	entity, err = c.billingService.Update(ctx, entity)
	if err != nil {
		log.Printf("Failed to update transaction: %v", err)
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	paymeDetails, ok = entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	return &paymeapi.CheckPerformTransactionResponse{
		Allow: true,
	}, nil
}

func (c *PaymeController) perform(ctx context.Context, r *paymeapi.PerformTransactionRequest) (*paymeapi.PerformTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
	entities, err := c.billingService.GetByDetailsFields(
		ctx,
		billing.Payme,
		[]billing.DetailsFieldFilter{
			{
				Path:     []string{"id"},
				Operator: billing.OpEqual,
				Value:    r.Id,
			},
		},
	)
	if err != nil || len(entities) != 1 {
		errRPC := paymeapi.PerformTransactionTransactionNotFoundError()
		return nil, &errRPC
	}

	entity := entities[0]

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	if paymeDetails.State() == paymeapi.TransactionStateCreated {
		paymeDetails = paymeDetails.
			SetState(paymeapi.TransactionStateCompleted).
			SetPerformTime(time.Now().UnixMilli())
	}

	entity = entity.
		SetStatus(billing.Completed).
		SetDetails(paymeDetails)

	entity, err = c.billingService.Update(ctx, entity)
	if err != nil {
		log.Printf("Failed to update transaction: %v", err)
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	paymeDetails, ok = entity.Details().(details.PaymeDetails)
	if !ok {
		log.Printf("Details is not of type PaymeDetails")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	return &paymeapi.PerformTransactionResponse{
		Transaction: paymeDetails.Transaction(),
		State:       paymeDetails.State(),
		PerformTime: paymeDetails.PerformTime(),
	}, nil
}

func (c *PaymeController) getStatement(ctx context.Context, r *paymeapi.GetStatementRequest) (*paymeapi.GetStatementResponse, *paymeapi.JSONRPCErrorResponseError) {
	entities, err := c.billingService.GetByDetailsFields(
		ctx,
		billing.Payme,
		[]billing.DetailsFieldFilter{
			{
				Path:     []string{"time"},
				Operator: billing.OpBetween,
				Value:    [2]any{r.From, r.To},
			},
		},
	)
	if err != nil || len(entities) != 1 {
		errRPC := paymeapi.PerformTransactionTransactionNotFoundError()
		return nil, &errRPC
	}

	sts := make([]paymeapi.StatementTransaction, 0, len(entities))
	for _, entity := range entities {
		paymeDetails, ok := entity.Details().(details.PaymeDetails)
		if !ok {
			log.Printf("Details is not of type PaymeDetails")
			continue
		}

		st := paymeapi.StatementTransaction{
			Id:          paymeDetails.ID(),
			Transaction: paymeDetails.Transaction(),
			Time:        paymeDetails.Time(),
			Amount:      entity.Amount().Quantity(),
			Account:     paymeDetails.Account(),
			CreateTime:  paymeDetails.CreatedTime(),
			PerformTime: paymeDetails.PerformTime(),
			CancelTime:  paymeDetails.CancelTime(),
			State:       paymeDetails.State(),
			Reason:      paymeapi.NullableInt32{},
		}

		if len(paymeDetails.Receivers()) > 0 {
			receivers := make([]paymeapi.Receiver, 0, len(paymeDetails.Receivers()))
			for _, r := range paymeDetails.Receivers() {
				receivers = append(receivers, paymeapi.Receiver{
					Id:     r.ID(),
					Amount: r.Amount(),
				})
			}
			st.Receivers = receivers
		}
		if paymeDetails.Reason() != 0 {
			reason := paymeDetails.Reason()
			st.Reason.Set(&reason)
		}

		sts = append(sts, st)
	}

	return &paymeapi.GetStatementResponse{
		Transactions: sts,
	}, nil
}
