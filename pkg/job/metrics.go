package job

import (
	"context"
	"sync"

	logger "github.com/openshift-online/ocm-service-common/pkg/ocmlogger"
)

type MetricsReporter interface {
	Report(metricsCollector *MetricsCollector)
}

// MetricsCollector uses locking to ensure we get point-in-time snapshot of the whole data. This snapshot data will be
// then used to report metrics.
type MetricsCollector struct {
	mu          sync.Mutex
	jobName     string
	taskTotal   uint32
	taskSuccess uint32
	taskFailed  uint32
}

func NewMetricsCollector(jobName string) *MetricsCollector {
	return &MetricsCollector{jobName: jobName}
}

func (m *MetricsCollector) SetTaskTotal(total uint32) {
	m.taskTotal = total
}
func (m *MetricsCollector) IncTaskSuccess() {
	m.mu.Lock()
	m.taskSuccess++
	m.mu.Unlock()
}
func (m *MetricsCollector) IncTaskFailed() {
	m.mu.Lock()
	m.taskFailed++
	m.mu.Unlock()
}

func (m *MetricsCollector) Snapshot() MetricsCollector {
	m.mu.Lock()
	defer m.mu.Unlock()

	return MetricsCollector{
		jobName:     m.jobName,
		taskTotal:   m.taskTotal,
		taskSuccess: m.taskSuccess,
		taskFailed:  m.taskFailed,
	}

}

type StdoutReporter struct {
}

func (r StdoutReporter) Report(metricsCollector *MetricsCollector) {
	// use snapshot for point-in-time data
	snapshot := metricsCollector.Snapshot()
	logger.NewOCMLogger(context.Background()).Contextual().Info("Printing metrics to STDOUT", "task_total", snapshot.taskTotal, "task_success", snapshot.taskSuccess, "task_failed", snapshot.taskFailed)
}

func NewStdoutReporter() MetricsReporter {
	return StdoutReporter{}
}
