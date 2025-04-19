package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stupid-simple/backup/scheduler"
)

type MockBackupJob struct {
	mock.Mock
}

func (m *MockBackupJob) Run() {
	m.Called()
}

func TestNewScheduler(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	s := scheduler.NewScheduler(scheduler.SchedulerParams{
		Logger: logger,
	})

	assert.NotNil(t, s, "Scheduler should not be nil")
}

func TestScheduler_AddBackupJob(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	s := scheduler.NewScheduler(scheduler.SchedulerParams{
		Logger: logger,
	})

	mockJob := new(MockBackupJob)

	err := s.AddBackupJob(context.Background(), "* * * * *", mockJob)
	assert.NoError(t, err, "Should add job without error")

	// Test with invalid schedule.
	err = s.AddBackupJob(context.Background(), "invalid-schedule", mockJob)
	assert.Error(t, err, "Should return error with invalid schedule")
}

func TestScheduler_StartStop(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	s := scheduler.NewScheduler(scheduler.SchedulerParams{
		Logger: logger,
	})

	mockJob := new(MockBackupJob)
	mockJob.On("Run").Return()

	err := s.AddBackupJob(context.Background(), "* * * * *", mockJob)
	assert.NoError(t, err)

	// Start the scheduler.
	s.Start()

	// Stop the scheduler after a short delay.
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	// No assertions here as we're just testing that Start and Stop don't panic.
}

func TestScheduler_RemoveJobs(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	s := scheduler.NewScheduler(scheduler.SchedulerParams{
		Logger: logger,
	})

	mockJob1 := new(MockBackupJob)
	mockJob2 := new(MockBackupJob)

	err := s.AddBackupJob(context.Background(), "* * * * *", mockJob1)
	assert.NoError(t, err)

	err = s.AddBackupJob(context.Background(), "*/5 * * * *", mockJob2)
	assert.NoError(t, err)

	// Remove all jobs.
	s.RemoveJobs()

	// We can't directly test that jobs were removed since the map is private.
	// But we can indirectly test by adding the same jobs again.
	err = s.AddBackupJob(context.Background(), "* * * * *", mockJob1)
	assert.NoError(t, err, "Should be able to add job again after removal")
}
