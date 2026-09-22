package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type AIGatewayConfig struct {
	Logger  zerolog.Logger
	Timeout time.Duration
}

type AIGateway struct {
	router  *ModelRouter
	logger  zerolog.Logger
	timeout time.Duration
}

func NewAIGateway(router *ModelRouter, config AIGatewayConfig) *AIGateway {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &AIGateway{
		router:  router,
		logger:  config.Logger,
		timeout: timeout,
	}
}

func (g *AIGateway) Generate(ctx context.Context, orgID string, request domain.AIRequest) (*domain.AIResponse, error) {
	if err := g.validateRequest(request); err != nil {
		return nil, domain.NewAIError(domain.AIErrInvalidResponse, err.Error())
	}

	model, provider, err := g.router.Route(ctx, orgID, request.Task)
	if err != nil {
		return nil, domain.NewAIError(domain.AIErrModelUnavailable, err.Error())
	}

	timeout := request.Timeout
	if timeout <= 0 {
		timeout = g.timeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request.ModelID = model.ModelID
	if request.RequestID == "" {
		request.RequestID = generateAIRequestID()
	}

	startTime := time.Now()
	resp, err := provider.Generate(ctx, request)
	latency := time.Since(startTime)

	if err != nil {
		aiErr, ok := err.(domain.AIError)
		if !ok {
			if isContextDeadlineExceeded(err) {
				return nil, domain.NewAIError(domain.AIErrProviderTimeout, "AI provider call timed out")
			}
			aiErr = domain.NewAIError(domain.AIErrUnknown, err.Error())
		}

		g.logger.Error().
			Str("provider", provider.ID()).
			Str("model", model.ModelID).
			Str("task", string(request.Task)).
			Str("request_id", request.RequestID).
			Dur("latency", latency).
			Err(err).
			Msg("AI provider call failed")

		return nil, aiErr
	}

	resp.LatencyMs = latency.Milliseconds()
	resp.ProviderID = provider.ID()
	resp.ModelID = model.ModelID

	if len(resp.Content) == 0 {
		return nil, domain.NewAIError(domain.AIErrInvalidResponse, "AI provider returned empty response")
	}

	g.logger.Info().
		Str("provider", provider.ID()).
		Str("model", model.ModelID).
		Str("task", string(request.Task)).
		Str("request_id", request.RequestID).
		Dur("latency", latency).
		Int("tokens", resp.TokensUsed).
		Msg("AI provider call completed")

	return &resp, nil
}

func (g *AIGateway) validateRequest(request domain.AIRequest) error {
	if request.Task == "" {
		return domain.NewAIError(domain.AIErrUnsupportedTask, "task type is required")
	}
	requiredCap := request.Task.RequiredCapability()
	if requiredCap == "" {
		return domain.NewAIError(domain.AIErrUnsupportedTask, "unknown task type: "+string(request.Task))
	}
	if len(request.Messages) == 0 {
		return domain.NewAIError(domain.AIErrInvalidResponse, "messages are required")
	}
	return nil
}

func isContextDeadlineExceeded(err error) bool {
	if err == nil {
		return false
	}
	ctxErr := err
	for ctxErr != nil {
		if ctxErr == context.DeadlineExceeded {
			return true
		}
		if ctxErr == context.Canceled {
			return true
		}
		ctxErr = unwrapContextError(ctxErr)
	}
	return false
}

func unwrapContextError(err error) error {
	type unwrapper interface {
		Unwrap() error
	}
	u, ok := err.(unwrapper)
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func generateAIRequestID() string {
	return "ai_" + fmt.Sprintf("req_%d", time.Now().UnixNano())
}
