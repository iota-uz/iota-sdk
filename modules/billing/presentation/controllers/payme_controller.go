package controllers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	paymeapi "github.com/iota-uz/payme"
	paymeauth "github.com/iota-uz/payme/auth"
	"github.com/sirupsen/logrus"
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
	router.HandleFunc("", di.H(c.Handle))
}

func (c *PaymeController) Key() string {
	return c.basePath
}

func (c *PaymeController) Handle(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	var (
		req             paymeapi.JSONRPCRequest
		successResponse *paymeapi.JSONRPCSuccessResponse
		errorResponse   *paymeapi.JSONRPCErrorResponse
	)

	logger.Info("Payme JSON-RPC request received")

	if err := paymeauth.ValidateBasicAuth(r, c.payme.User, c.payme.SecretKey); err != nil {
		logger.WithError(err).Error("Payme authentication failed")
		errorResponse = &paymeapi.JSONRPCErrorResponse{
			Error: paymeapi.InsufficientPrivilegesError(),
		}
	}
	if errorResponse == nil && r.Method != http.MethodPost {
		logger.WithField("method", r.Method).Error("Invalid HTTP method for Payme")
		errorResponse = &paymeapi.JSONRPCErrorResponse{
			Error: paymeapi.MethodNotPOSTError(),
		}
	}

	if errorResponse == nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.WithError(err).Error("Failed to parse Payme JSON-RPC request")
			errorResponse = &paymeapi.JSONRPCErrorResponse{
				Error: paymeapi.JSONParseError(),
			}
		}
	}
	if errorResponse == nil {
		switch {
		case req.CancelTransactionRequestWrapper != nil:
			logger.Info("Processing Payme CancelTransaction request")
			cancelReq := req.CancelTransactionRequestWrapper
			resp, errRPC := c.cancel(r.Context(), &cancelReq.Params, logger)
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
			logger.Info("Processing Payme CheckPerformTransaction request")
			checkPerformReq := req.CheckPerformTransactionRequestWrapper
			resp, errRPC := c.checkPerform(r.Context(), &checkPerformReq.Params, logger)
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
			logger.Info("Processing Payme CheckTransaction request")
			checkReq := req.CheckTransactionRequestWrapper
			resp, errRPC := c.check(r.Context(), &checkReq.Params, logger)
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
			logger.Info("Processing Payme CreateTransaction request")
			createReq := req.CreateTransactionRequestWrapper
			resp, errRPC := c.create(r.Context(), &createReq.Params, logger)
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
			logger.Info("Processing Payme PerformTransaction request")
			performReq := req.PerformTransactionRequestWrapper
			resp, errRPC := c.perform(r.Context(), &performReq.Params, logger)
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
			logger.Info("Processing Payme GetStatement request")
			getStatementReq := req.GetStatementRequestWrapper
			resp, errRPC := c.getStatement(r.Context(), &getStatementReq.Params, logger)
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
			logger.Error("Method not found in Payme request data")
			errorResponse = &paymeapi.JSONRPCErrorResponse{
				Error: paymeapi.MethodNotFoundInDataError(),
			}
		}
	}

	if errorResponse != nil {
		logger.WithField("error_code", errorResponse.Error.Code).Error("Payme request failed")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
			logger.WithError(err).Error("Failed to write Payme error response")
		}
		return
	}

	if successResponse != nil {
		logger.Info("Payme request completed successfully")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(successResponse); err != nil {
			logger.WithError(err).Error("Failed to write Payme success response")
		}
		return
	}
}

func (c *PaymeController) check(ctx context.Context, r *paymeapi.CheckTransactionRequest, logger *logrus.Entry) (*paymeapi.CheckTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
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
		logger.WithError(err).WithField("transaction_id", r.Id).Error("Transaction not found in CheckTransaction")
		errRPC := paymeapi.CheckTransactionTransactionNotFoundError()
		return nil, &errRPC
	}
	entity := entities[0]

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		logger.Error("Details is not of type PaymeDetails in CheckTransaction")
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

	logger.WithFields(logrus.Fields{
		"transaction_id": r.Id,
		"state":          paymeDetails.State(),
	}).Info("CheckTransaction completed")

	return &resp, nil
}

func (c *PaymeController) create(ctx context.Context, r *paymeapi.CreateTransactionRequest, logger *logrus.Entry) (*paymeapi.CreateTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
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
		logger.WithError(err).WithField("transaction_id", r.Id).Error("Invalid account in CreateTransaction")
		errRPC := paymeapi.InvalidAccountError()
		return nil, &errRPC
	}

	entity := entities[0]

	amount := r.Amount / 100
	if math.Abs(entity.Amount().Quantity()-amount) >= 1e-9 {
		logger.WithFields(logrus.Fields{
			"expected_amount": entity.Amount().Quantity(),
			"provided_amount": amount,
			"transaction_id":  r.Id,
		}).Error("Invalid amount in CreateTransaction")
		errRPC := paymeapi.InvalidAmountError()
		return nil, &errRPC
	}

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		logger.Error("Details is not of type PaymeDetails in CreateTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	if paymeDetails.ID() != "" && paymeDetails.ID() != r.Id {
		logger.WithFields(logrus.Fields{
			"existing_id":  paymeDetails.ID(),
			"requested_id": r.Id,
		}).Error("Transaction ID mismatch in CreateTransaction")
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

	entity, err = c.billingService.Save(ctx, entity)
	if err != nil {
		logger.WithError(err).WithField("transaction_id", r.Id).Error("Failed to save transaction in CreateTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	paymeDetails, ok = entity.Details().(details.PaymeDetails)
	if !ok {
		logger.Error("Details is not of type PaymeDetails after save in CreateTransaction")
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

	logger.WithFields(logrus.Fields{
		"transaction_id": r.Id,
		"state":          paymeDetails.State(),
	}).Info("CreateTransaction completed successfully")

	return resp, nil
}

func (c *PaymeController) cancel(ctx context.Context, r *paymeapi.CancelTransactionRequest, logger *logrus.Entry) (*paymeapi.CancelTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
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
		logger.WithError(err).WithField("transaction_id", r.Id).Error("Transaction not found in CancelTransaction")
		errRPC := paymeapi.CancelTransactionTransactionNotFoundError()
		return nil, &errRPC
	}

	entity := entities[0]

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		logger.Error("Details is not of type PaymeDetails in CancelTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	if paymeDetails.CancelTime() == 0 {
		paymeDetails = paymeDetails.SetCancelTime(time.Now().UnixMilli())
	}

	paymeDetails = paymeDetails.SetReason(r.Reason)

	oldState := paymeDetails.State()
	oldStatus := entity.Status()

	switch paymeDetails.State() {
	case paymeapi.TransactionStateCreated:
		paymeDetails = paymeDetails.SetState(paymeapi.TransactionStateCancelledBeforeCompletion)
		entity = entity.SetStatus(billing.Canceled)
	case paymeapi.TransactionStateCompleted:
		paymeDetails = paymeDetails.SetState(paymeapi.TransactionStateCancelledAfterCompletion)
		entity = entity.SetStatus(billing.Refunded)
	}

	entity = entity.SetDetails(paymeDetails)

	entity, err = c.billingService.Save(ctx, entity)
	if err != nil {
		logger.WithError(err).WithField("transaction_id", r.Id).Error("Failed to save transaction in CancelTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	// Invoke callback for notification (non-blocking)
	if err := c.billingService.InvokeCallback(ctx, entity); err != nil {
		logger.WithError(err).WithField("transaction_id", r.Id).Warn("Callback error on status change")
	}

	paymeDetails, ok = entity.Details().(details.PaymeDetails)
	if !ok {
		logger.Error("Details is not of type PaymeDetails after save in CancelTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	logger.WithFields(logrus.Fields{
		"transaction_id": r.Id,
		"old_state":      oldState,
		"new_state":      paymeDetails.State(),
		"old_status":     oldStatus,
		"new_status":     entity.Status(),
		"reason":         r.Reason,
	}).Info("CancelTransaction completed successfully")

	return &paymeapi.CancelTransactionResponse{
		Transaction: paymeDetails.Transaction(),
		CancelTime:  paymeDetails.CancelTime(),
		State:       paymeDetails.State(),
	}, nil
}

func (c *PaymeController) checkPerform(ctx context.Context, r *paymeapi.CheckPerformTransactionRequest, logger *logrus.Entry) (*paymeapi.CheckPerformTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
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
		logger.WithError(err).Error("Invalid account in CheckPerformTransaction")
		errRPC := paymeapi.CheckPerformTransactionInvalidAccountError()
		return nil, &errRPC
	}

	entity := entities[0]

	amount := r.Amount / 100
	if math.Abs(entity.Amount().Quantity()-amount) >= 1e-9 {
		logger.WithFields(logrus.Fields{
			"expected_amount": entity.Amount().Quantity(),
			"provided_amount": amount,
		}).Error("Invalid amount in CheckPerformTransaction")
		errRPC := paymeapi.CheckPerformTransactionInvalidAmountError()
		return nil, &errRPC
	}

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		logger.Error("Details is not of type PaymeDetails in CheckPerformTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	if paymeDetails.State() != paymeapi.TransactionStateCreated {
		logger.WithField("state", paymeDetails.State()).Error("Invalid transaction state in CheckPerformTransaction")
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

	_, err = c.billingService.Save(ctx, entity)
	if err != nil {
		logger.WithError(err).Error("Failed to save transaction in CheckPerformTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	logger.Info("CheckPerformTransaction completed successfully")

	return &paymeapi.CheckPerformTransactionResponse{
		Allow: true,
	}, nil
}

func (c *PaymeController) perform(ctx context.Context, r *paymeapi.PerformTransactionRequest, logger *logrus.Entry) (*paymeapi.PerformTransactionResponse, *paymeapi.JSONRPCErrorResponseError) {
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
		logger.WithError(err).WithField("transaction_id", r.Id).Error("Transaction not found in PerformTransaction")
		errRPC := paymeapi.PerformTransactionTransactionNotFoundError()
		return nil, &errRPC
	}

	entity := entities[0]

	paymeDetails, ok := entity.Details().(details.PaymeDetails)
	if !ok {
		logger.Error("Details is not of type PaymeDetails in PerformTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	// Invoke callback to validate transaction
	if err := c.billingService.InvokeCallback(ctx, entity); err != nil {
		logger.WithError(err).WithField("transaction_id", r.Id).Error("Callback error in PerformTransaction")
		paymeDetails = paymeDetails.
			SetReason(paymeapi.CancelReasonExecutionError).
			SetState(paymeapi.TransactionStateCancelledBeforeCompletion).
			SetErrorCode(paymeapi.PerformTransactionErrorOperationNotAllowed)
		entity = entity.SetStatus(billing.Failed).SetDetails(paymeDetails)
		if _, saveErr := c.billingService.Save(ctx, entity); saveErr != nil {
			logger.WithError(saveErr).Error("Failed to save callback error in PerformTransaction")
		}
		errRPC := paymeapi.PerformTransactionOperationNotAllowedError()
		return nil, &errRPC
	}

	oldState := paymeDetails.State()
	if paymeDetails.State() == paymeapi.TransactionStateCreated {
		paymeDetails = paymeDetails.
			SetState(paymeapi.TransactionStateCompleted).
			SetPerformTime(time.Now().UnixMilli())
	}

	entity = entity.
		SetStatus(billing.Completed).
		SetDetails(paymeDetails)

	entity, err = c.billingService.Save(ctx, entity)
	if err != nil {
		logger.WithError(err).WithField("transaction_id", r.Id).Error("Failed to save transaction in PerformTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	paymeDetails, ok = entity.Details().(details.PaymeDetails)
	if !ok {
		logger.Error("Details is not of type PaymeDetails after save in PerformTransaction")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	logger.WithFields(logrus.Fields{
		"transaction_id": r.Id,
		"old_state":      oldState,
		"new_state":      paymeDetails.State(),
		"new_status":     billing.Completed,
	}).Info("PerformTransaction completed successfully")

	return &paymeapi.PerformTransactionResponse{
		Transaction: paymeDetails.Transaction(),
		State:       paymeDetails.State(),
		PerformTime: paymeDetails.PerformTime(),
	}, nil
}

func (c *PaymeController) getStatement(ctx context.Context, r *paymeapi.GetStatementRequest, logger *logrus.Entry) (*paymeapi.GetStatementResponse, *paymeapi.JSONRPCErrorResponseError) {
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
	if err != nil {
		logger.WithError(err).WithFields(logrus.Fields{
			"from": r.From,
			"to":   r.To,
		}).Error("Failed to get transactions in GetStatement")
		errRPC := paymeapi.InternalSystemError()
		return nil, &errRPC
	}

	sts := make([]paymeapi.StatementTransaction, 0, len(entities))
	for _, entity := range entities {
		paymeDetails, ok := entity.Details().(details.PaymeDetails)
		if !ok {
			logger.Error("Details is not of type PaymeDetails in GetStatement, skipping")
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

	sort.Slice(sts, func(i, j int) bool {
		return sts[i].Time < sts[j].Time
	})

	logger.WithFields(logrus.Fields{
		"from":              r.From,
		"to":                r.To,
		"transaction_count": len(sts),
	}).Info("GetStatement completed successfully")

	return &paymeapi.GetStatementResponse{
		Transactions: sts,
	}, nil
}
