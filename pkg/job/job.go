package job

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	logger "github.com/openshift-online/ocm-service-common/pkg/ocmlogger"
	"github.com/segmentio/ksuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Metadata contains the identifying information for a Job, such as its command name and description.
type Metadata struct {
	Use         string // CLI name of the job.
	Description string // Description shown in CLI help output.
}

// Job defines a unit of work that can be registered and executed by the job framework.
//
// A Job provides metadata, configures command-line flags, defines a set of tasks to run,
// and specifies how many workers should process those tasks concurrently.
type Job interface {
	GetMetadata() Metadata
	AddFlags(flags *pflag.FlagSet)
	GetTasks() ([]Task, error)
	GetWorkerCount() int
}

// CommandBuilder builds a Cobra CLI command that wraps registered jobs.
//
// It supports optional lifecycle hooks and task wrappers for additional behavior.
type CommandBuilder struct {
	registry JobRegistry
	ctx      context.Context
	// beforeJob is an optional function that gets executed before Job is run.
	beforeJob func(ctx context.Context) error
	// afterJob is an optional function that gets executed after Job is run.
	afterJob func(ctx context.Context)
	// panicHandler is an optional function that accepts interface and can deal with it how it wants.
	// An example use can be to capture any errors and report it to sentry. Not setting panicHandler means any panics
	// encountered will be silently recovered.
	panicHandler    func(ctx context.Context, any interface{})
	metricsReporter MetricsReporter
}

func (b *CommandBuilder) SetRegistry(registry JobRegistry) *CommandBuilder {
	b.registry = registry
	return b
}

func (b *CommandBuilder) SetContext(ctx context.Context) *CommandBuilder {
	b.ctx = ctx
	return b
}

func (b *CommandBuilder) SetBeforeJob(fn func(ctx context.Context) error) *CommandBuilder {
	b.beforeJob = fn
	return b
}
func (b *CommandBuilder) SetAfterJob(fn func(ctx context.Context)) *CommandBuilder {
	b.afterJob = fn
	return b
}

func (b *CommandBuilder) SetPanicHandler(fn func(ctx context.Context, any interface{})) *CommandBuilder {
	b.panicHandler = fn
	return b
}

func (b *CommandBuilder) SetMetricsReporter(reporter MetricsReporter) *CommandBuilder {
	b.metricsReporter = reporter
	return b
}

func (b *CommandBuilder) Build() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run-job",
		Short: "Run job",
		Long:  "Run job",
	}
	for _, job := range b.registry.jobs {
		job := job // capture loop variable
		subCmd := &cobra.Command{
			Use:  job.GetMetadata().Use,
			Long: job.GetMetadata().Description,
			// We don't need this info if job fails.
			SilenceUsage: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				err := validateJob(job)
				if err != nil {
					return err
				}
				if b.metricsReporter == nil {
					b.metricsReporter = NewStdoutReporter()
				}
				err = jobRunner{BeforeJob: b.beforeJob, AfterJob: b.afterJob, PanicHandler: b.panicHandler, MetricsReporter: b.metricsReporter}.Run(b.ctx, job, job.GetWorkerCount())
				if err != nil {
					return err
				}
				return nil
			},
		}
		job.AddFlags(subCmd.Flags())
		cmd.AddCommand(subCmd)
	}

	return cmd
}

func validateJob(job Job) error {
	if job.GetWorkerCount() < 1 {
		return errors.New("number of workers must be greater than zero")
	}
	return nil
}

type JobRegistry struct {
	jobs []Job
}

func NewJobRegistry() *JobRegistry {
	return &JobRegistry{}
}

func (r *JobRegistry) AddJob(job Job) {
	if job == nil {
		return
	}
	r.jobs = append(r.jobs, job)
}

// Task represents a unit of work that can be processed by a worker.
type Task interface {
	Process(ctx context.Context) error
	TaskName() string
}

// taskQueue is a thread-safe FIFO queue of tasks.
type taskQueue struct {
	Tasks []Task
	mu    sync.Mutex
}

// Add appends a task to the queue.
//
// Note: This method is not thread-safe unless used with external synchronization.
func (q *taskQueue) Add(task Task) {
	q.Tasks = append(q.Tasks, task)
}

func (q *taskQueue) GetTask() Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.Tasks) == 0 {
		return nil
	}
	task := q.Tasks[0]
	q.Tasks = q.Tasks[1:]
	return task
}

func newTaskQueue() *taskQueue {
	return &taskQueue{mu: sync.Mutex{}}
}

// WorkerPool runs a fixed number of workers to process tasks from a queue.
type workerPool struct {
	Queue            *taskQueue
	Workers          int
	PanicHandler     func(ctx context.Context, any interface{})
	MetricsCollector *MetricsCollector
}

// Run starts the worker pool and processes tasks until the queue is empty.
func (wp *workerPool) Run(ctx context.Context) {
	ulog := logger.NewOCMLogger(ctx)
	var wg sync.WaitGroup

	for i := 0; i < wp.Workers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for {
				task := wp.Queue.GetTask()
				if task == nil {
					// No more tasks left
					ulog.Info("No more tasks in queue")
					return
				}
				func() {
					taskId := ksuid.New().String()

					taskCtx := AddTraceContext(ctx, "workerId", strconv.Itoa(workerId))
					taskCtx = AddTraceContext(taskCtx, "taskName", task.TaskName())
					taskCtx = AddTraceContext(taskCtx, "taskId", taskId)

					defer func(taskCtx context.Context) {
						if err := recover(); err != nil {
							wp.MetricsCollector.IncTaskFailed()
							logger.NewOCMLogger(taskCtx).Contextual().Error(fmt.Errorf("<panic summary should go here>"), fmt.Sprintf("[Task %s] Panic", task.TaskName()), "exception", err)
							if wp.PanicHandler != nil {
								wp.PanicHandler(taskCtx, err)
							}
						}
					}(taskCtx)

					logger.NewOCMLogger(taskCtx).Contextual().Info("Processing task", "workerId", workerId, "taskId", taskId)
					err := task.Process(taskCtx)
					if err != nil {
						wp.MetricsCollector.IncTaskFailed()
						logger.NewOCMLogger(ctx).Contextual().Error(err, fmt.Sprintf("[Task %s] Failed", task.TaskName()))
					} else {
						wp.MetricsCollector.IncTaskSuccess()
					}
				}()
			}
		}(i)
	}

	wg.Wait()
}

type runner interface {
	Run(context.Context, Job, int) error
}

var _ runner = &jobRunner{}
var _ runner = &TestRunner{}

// JobRunner is responsible for executing a job and managing its lifecycle hooks and task wrappers.
type jobRunner struct {
	BeforeJob       func(ctx context.Context) error
	AfterJob        func(ctx context.Context)
	PanicHandler    func(ctx context.Context, any interface{})
	MetricsReporter MetricsReporter
}

// Run executes the given job using a worker pool.
//
// It first invokes the BeforeJob hook (if defined). Then, it enqueues all tasks and delegates to worker pool for execution.
// After all tasks are processed, the AfterJob hook is called.
func (jr jobRunner) Run(ctx context.Context, job Job, workerCount int) error {
	ctx = AddTraceContext(ctx, "jobName", job.GetMetadata().Use)
	ctx = AddTraceContext(ctx, "jobId", ksuid.New().String())
	ulog := logger.NewOCMLogger(ctx)

	defer func() {
		if err := recover(); err != nil {
			// Panic in main goroutine
			if jr.PanicHandler != nil {
				jr.PanicHandler(ctx, err)
			}

			ulog.Contextual().Error(fmt.Errorf("<panic summary should go here>"), fmt.Sprintf("[Job %s] Panic", job.GetMetadata().Use), "exception", err)
			// We are purposefully re-throwing panic in main goroutine, so that job fails, and we don't suppress any errors.
			// This is opposite of how we handle panic in worker pool, where we need to handle any panics from individual tasks
			// so we can protect other workers from executing.
			panic(err)
		}
	}()

	if jr.BeforeJob != nil {
		ulog.Contextual().Info("executing beforeJob hook")
		err := jr.BeforeJob(ctx)
		if err != nil {
			ulog.Contextual().Error(err, fmt.Sprintf("[Job %s] Error executing beforeJob hook", job.GetMetadata().Use))
			return err
		}
	}

	taskTotal := 0
	taskQueue := newTaskQueue()

	tasks, err := job.GetTasks()

	if err != nil {
		ulog.Contextual().Error(err, fmt.Sprintf("[Job %s] Error getting tasks", job.GetMetadata().Use))
		return err
	}
	for _, task := range tasks {
		taskQueue.Add(task)
		taskTotal += 1
	}
	metricsCollector := NewMetricsCollector(job.GetMetadata().Use)
	metricsCollector.SetTaskTotal(uint32(taskTotal))

	ulog.Contextual().Info("queued all the tasks")

	pool := workerPool{Queue: taskQueue, Workers: workerCount, PanicHandler: jr.PanicHandler, MetricsCollector: metricsCollector}
	pool.Run(ctx)

	if jr.AfterJob != nil {
		ulog.Contextual().Info("executing afterJob hook")
		jr.AfterJob(ctx)
	}

	// For now, we report metrics only once at the end. In the future, we may need to support periodic reporting or
	// synchronous updates (e.g., when counters are modified) to integrate with push-based systems like Prometheus Pushgateway.
	jr.MetricsReporter.Report(metricsCollector)

	if metricsCollector.taskTotal == 0 {
		// this can happen when there are no tasks!
		ulog.Contextual().Info("No tasks to run!")
		return nil
	}
	// For now return error when all tasks fail. This can be configurable for e.g. return error when 80% of tasks fail.
	if metricsCollector.taskFailed == metricsCollector.taskTotal {
		err := errors.New("all tasks failed")
		return err
	}

	ulog.Contextual().Info("job executed successfully")
	return nil
}

// TestRunner is a lightweight JobRunner implementation to enable for easy testing of job logic.
type TestRunner struct{}

func (tr TestRunner) Run(ctx context.Context, job Job, workerCount int) error {
	taskTotal := 0
	taskQueue := newTaskQueue()

	tasks, err := job.GetTasks()

	if err != nil {
		return err
	}
	for _, task := range tasks {
		taskQueue.Add(task)
		taskTotal += 1
	}
	metricsCollector := NewMetricsCollector(job.GetMetadata().Use)
	metricsCollector.SetTaskTotal(uint32(taskTotal))

	pool := workerPool{Queue: taskQueue, Workers: workerCount, PanicHandler: nil, MetricsCollector: metricsCollector}
	pool.Run(ctx)

	if metricsCollector.taskFailed == metricsCollector.taskTotal {
		err := errors.New("all tasks failed")
		return err
	}
	return nil
}
