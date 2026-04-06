package scheduler

import (
	"context"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/agent"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

// AgentScheduler runs agents on configurable intervals.
type AgentScheduler struct {
	runner *agent.Runner
	agents []scheduledAgent
	logger *logger.ModuleLogger
}

type scheduledAgent struct {
	agent    agent.Agent
	interval time.Duration
	lastRun  time.Time
}

type AgentSchedulerConfig struct {
	InterestProfileInterval time.Duration
	TopicClusterInterval    time.Duration
	DailyBriefTime          int // hour of day (0-23) to run daily brief
}

func DefaultAgentSchedulerConfig() AgentSchedulerConfig {
	return AgentSchedulerConfig{
		InterestProfileInterval: 1 * time.Hour,
		TopicClusterInterval:    30 * time.Minute,
		DailyBriefTime:          2, // 2 AM
	}
}

func NewAgentScheduler(deps agent.Deps, cfg AgentSchedulerConfig) *AgentScheduler {
	runner := agent.NewRunner(deps)

	agents := []scheduledAgent{
		{agent: agent.NewInterestProfileAgent(deps), interval: cfg.InterestProfileInterval},
		{agent: agent.NewTopicClusterAgent(deps), interval: cfg.TopicClusterInterval},
	}

	return &AgentScheduler{
		runner: runner,
		agents: agents,
		logger: logger.NewModuleFromEnv("scheduler"),
	}
}

func (s *AgentScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Run interest profile and topic cluster once at startup
	for i := range s.agents {
		s.tryRun(ctx, &s.agents[i])
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("refresh", "agent", "cancelled", "agent scheduler stopped")
			return
		case <-ticker.C:
			now := time.Now()
			for i := range s.agents {
				sa := &s.agents[i]
				if now.Sub(sa.lastRun) >= sa.interval {
					s.tryRun(ctx, sa)
				}
			}
		}
	}
}

func (s *AgentScheduler) tryRun(ctx context.Context, sa *scheduledAgent) {
	sa.lastRun = time.Now()
	if err := s.runner.RunAgent(ctx, sa.agent); err != nil {
		s.logger.Warn("refresh", "agent", "failed", "scheduled agent run failed", "agent", sa.agent.Name(), "error", err.Error())
	}
}

// BriefScheduler runs topic brief agents at specific times.
type BriefScheduler struct {
	runner         *agent.Runner
	dailyAgent     agent.Agent
	weeklyAgent    agent.Agent
	monthlyAgent   agent.Agent
	dailyBriefHour int
	logger         *logger.ModuleLogger
}

func NewBriefScheduler(deps agent.Deps, dailyBriefHour int) *BriefScheduler {
	return &BriefScheduler{
		runner:         agent.NewRunner(deps),
		dailyAgent:     agent.NewTopicBriefAgent(deps, "daily"),
		weeklyAgent:    agent.NewTopicBriefAgent(deps, "weekly"),
		monthlyAgent:   agent.NewTopicBriefAgent(deps, "monthly"),
		dailyBriefHour: dailyBriefHour,
		logger:         logger.NewModuleFromEnv("scheduler"),
	}
}

func (s *BriefScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	var lastDailyDate string
	var lastWeeklyWeek int
	var lastMonthlyMonth time.Month

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("refresh", "brief", "cancelled", "brief scheduler stopped")
			return
		case <-ticker.C:
			now := time.Now()

			// Daily brief: run once per day after the configured hour
			todayDate := now.Format("2006-01-02")
			if now.Hour() >= s.dailyBriefHour && todayDate != lastDailyDate {
				lastDailyDate = todayDate
				if err := s.runner.RunAgent(ctx, s.dailyAgent); err != nil {
					s.logger.Warn("refresh", "brief", "failed", "daily brief failed", "error", err.Error())
				}
			}

			// Weekly brief: run on Monday
			_, week := now.ISOWeek()
			if now.Weekday() == time.Monday && now.Hour() >= s.dailyBriefHour && week != lastWeeklyWeek {
				lastWeeklyWeek = week
				if err := s.runner.RunAgent(ctx, s.weeklyAgent); err != nil {
					s.logger.Warn("refresh", "brief", "failed", "weekly brief failed", "error", err.Error())
				}
			}

			// Monthly brief: run on 1st of month
			if now.Day() == 1 && now.Hour() >= s.dailyBriefHour && now.Month() != lastMonthlyMonth {
				lastMonthlyMonth = now.Month()
				if err := s.runner.RunAgent(ctx, s.monthlyAgent); err != nil {
					s.logger.Warn("refresh", "brief", "failed", "monthly brief failed", "error", err.Error())
				}
			}
		}
	}
}
