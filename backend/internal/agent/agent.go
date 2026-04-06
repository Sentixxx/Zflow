package agent

import (
	"context"
	"fmt"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

// Agent defines the interface for all agent types.
type Agent interface {
	Name() string
	Run(ctx context.Context) error
}

// Deps holds shared dependencies injected into agents.
type Deps struct {
	AgentRepo  repository.AgentRepository
	FeedRepo   repository.FeedRepository
	VectorRepo repository.VectorRepository
	LLMCall    LLMCallFunc
}

// LLMCallFunc calls the LLM with a system prompt and user prompt, returning the response text.
type LLMCallFunc func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

// Runner executes agents and records their runs.
type Runner struct {
	deps   Deps
	logger *logger.ModuleLogger
}

func NewRunner(deps Deps) *Runner {
	return &Runner{
		deps:   deps,
		logger: logger.NewModuleFromEnv("agent"),
	}
}

// RunAgent executes a single agent, recording the run in the database.
func (r *Runner) RunAgent(ctx context.Context, a Agent) error {
	run, err := r.deps.AgentRepo.CreateAgentRun(ctx, a.Name(), "")
	if err != nil {
		return fmt.Errorf("create agent run: %w", err)
	}

	r.logger.Info("run", a.Name(), "started", "agent started")

	if err := a.Run(ctx); err != nil {
		_ = r.deps.AgentRepo.FailAgentRun(ctx, run.ID, err.Error())
		r.logger.Warn("run", a.Name(), "failed", "agent failed", "error", err.Error())
		return err
	}

	_ = r.deps.AgentRepo.CompleteAgentRun(ctx, run.ID, "", 0, 0)
	r.logger.Info("run", a.Name(), "ok", "agent completed")
	return nil
}
