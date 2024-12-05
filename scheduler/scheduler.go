package scheduler

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

type BackupJob interface {
	Run()
}

type SchedulerParams struct {
	Logger zerolog.Logger
}

func NewScheduler(params SchedulerParams) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		logger: params.Logger,
		jobs:   make(map[cron.EntryID]BackupJob),
	}
}

type Scheduler struct {
	cron   *cron.Cron
	jobs   map[cron.EntryID]BackupJob
	logger zerolog.Logger
}

// Start the scheduler in its own routine.
func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) AddBackupJob(ctx context.Context, schedule string, job BackupJob) error {
	entry, err := s.cron.AddJob(schedule, job)
	if err != nil {
		return fmt.Errorf("could not add backup job: %w", err)
	}

	s.jobs[entry] = job

	return nil
}

func (s *Scheduler) RemoveJobs() {
	for entry := range s.jobs {
		s.cron.Remove(entry)
		delete(s.jobs, entry)
	}
}
