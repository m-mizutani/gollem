package reflexion

import (
	"context"
	"time"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// trial represents a single execution attempt (internal use only).
type trial struct {
	number     int
	trajectory *Trajectory
	evaluation *EvaluationResult
	reflection string
	endTime    time.Time
}

// startTrial starts a new trial and returns the inputs to send to the LLM.
func (s *Strategy) startTrial(ctx context.Context, state *gollem.StrategyState) ([]gollem.Input, *gollem.ExecuteResponse, error) {
	s.currentTrial++

	// Hook: OnTrialStart
	if s.hooks != nil {
		if err := s.hooks.OnTrialStart(ctx, s.currentTrial); err != nil {
			return nil, nil, goerr.Wrap(err, "hook OnTrialStart failed")
		}
	}

	// Build inputs with memory prompt if we have reflections
	inputs := []gollem.Input{}
	if s.memory.size() > 0 {
		memoryPrompt := buildMemoryPrompt(s.memory.getAll())
		inputs = append(inputs, memoryPrompt)
	}
	inputs = append(inputs, state.InitInput...)

	return inputs, nil, nil
}

// completeTrial handles the completion of a trial: evaluation, reflection, and next trial.
func (s *Strategy) completeTrial(ctx context.Context, state *gollem.StrategyState) ([]gollem.Input, *gollem.ExecuteResponse, error) {
	// Build trajectory from session history
	trajectory := buildTrajectory(ctx, state, s.currentTrial)

	// Evaluate
	evaluation, err := s.evaluator(ctx, trajectory)
	if err != nil {
		return nil, nil, goerr.Wrap(err, "evaluation failed")
	}

	// Success - finalize and return
	if evaluation.Success {
		return s.finalizeTrial(ctx, trajectory, evaluation, "")
	}

	// Failure - check if we can retry
	if s.currentTrial >= s.maxTrials {
		// Max trials reached - finalize with failure
		return s.finalizeTrial(ctx, trajectory, evaluation, "")
	}

	// Generate reflection for next trial
	reflection, err := generateReflection(ctx, s.client, trajectory, evaluation, s.memory.getAll())
	if err != nil {
		return nil, nil, goerr.Wrap(err, "reflection generation failed")
	}

	// Hook: OnReflectionGenerated
	if s.hooks != nil {
		if err := s.hooks.OnReflectionGenerated(ctx, s.currentTrial, reflection); err != nil {
			return nil, nil, goerr.Wrap(err, "hook OnReflectionGenerated failed")
		}
	}

	// Save to memory
	s.memory.add(s.currentTrial, reflection)

	// Save trial record
	t := &trial{
		number:     s.currentTrial,
		trajectory: trajectory,
		evaluation: evaluation,
		reflection: reflection,
		endTime:    time.Now(),
	}
	s.trials = append(s.trials, t)

	// Hook: OnTrialEnd
	if s.hooks != nil {
		if err := s.hooks.OnTrialEnd(ctx, s.currentTrial, evaluation); err != nil {
			return nil, nil, goerr.Wrap(err, "hook OnTrialEnd failed")
		}
	}

	// Start next trial
	return s.startTrial(ctx, state)
}

// finalizeTrial saves the final trial and returns the response.
func (s *Strategy) finalizeTrial(ctx context.Context, trajectory *Trajectory, evaluation *EvaluationResult, reflection string) ([]gollem.Input, *gollem.ExecuteResponse, error) {
	// Save final trial
	t := &trial{
		number:     s.currentTrial,
		trajectory: trajectory,
		evaluation: evaluation,
		reflection: reflection,
		endTime:    time.Now(),
	}
	s.trials = append(s.trials, t)

	// Hook: OnTrialEnd
	if s.hooks != nil {
		if err := s.hooks.OnTrialEnd(ctx, s.currentTrial, evaluation); err != nil {
			return nil, nil, goerr.Wrap(err, "hook OnTrialEnd failed")
		}
	}

	return nil, &gollem.ExecuteResponse{
		Texts: trajectory.FinalResponse,
	}, nil
}

// isTrialInProgress returns true if a trial is currently in progress.
func (s *Strategy) isTrialInProgress() bool {
	// Trial is in progress if we've started but the number of completed trials is less than current
	return s.currentTrial > 0 && len(s.trials) < s.currentTrial
}

// isTrialComplete returns true if the current trial has completed.
// A trial is complete when the LLM responds without any function calls.
func (s *Strategy) isTrialComplete(state *gollem.StrategyState) bool {
	return state.LastResponse != nil && len(state.LastResponse.FunctionCalls) == 0
}
