package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerPool manages a pool of worker goroutines
// Пул воркеров для обработки событий агентов
type WorkerPool struct {
	// workers количество активных воркеров
	workers int
	
	// jobChan канал для заданий
	jobChan chan Job
	
	// workerStatuses статусы воркеров
	workerStatuses []WorkerStatus
	
	// mu protects concurrent access
	mu sync.RWMutex
	
	// startedAt время начала работы
	startedAt time.Time
	
	// wg waits for all workers to finish
	wg sync.WaitGroup
	
	// shutdown flag for graceful shutdown
	shutdown atomic.Bool
	
	// totalProcessed общее количество обработанных событий
	totalProcessed atomic.Int64
	
	// totalErrors общее количество ошибок
	totalErrors atomic.Int64
}

// Job represents a work unit
type Job struct {
	// Agent агент для обработки
	Agent Agent
	
	// Event событие для обработки
	Event Event
	
	// Priority приоритет обработки
	Priority int
	
	// Done channel for completion signaling
	Done chan error
	
	// Timestamp время создания
	Timestamp time.Time
}

// NewWorkerPool создает пул воркеров
func NewWorkerPool(size int) *WorkerPool {
	wp := &WorkerPool{
		workers:      size,
		jobChan:      make(chan Job, size*10), // Буфер 10 заданий на воркера
		workerStatuses: make([]WorkerStatus, size),
		startedAt:    time.Now(),
	}
	
	// Инициализируем статусы
	for i := 0; i < size; i++ {
		wp.workerStatuses[i] = WorkerStatus{
			WorkerID: fmt.Sprintf("worker-%d", i),
			Status:   "idle",
		}
	}
	
	return wp
}

// Start запускает воркеры
func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.workerLoop(ctx, i)
	}
}

// workerLoop основной цикл воркера
func (wp *WorkerPool) workerLoop(ctx context.Context, index int) {
	defer wp.wg.Done()
	
	status := &wp.workerStatuses[index]
	
	for {
		select {
		case <-ctx.Done():
			status.Status = "stopped"
			return
		case job, ok := <-wp.jobChan:
			if !ok {
				// Канал закрыт, завершаем
				return
			}
			
			// Обработка
			wp.processJob(ctx, job, index, status)
		}
	}
}

// processJob обрабатывает задание
func (wp *WorkerPool) processJob(ctx context.Context, job Job, index int, status *WorkerStatus) {
	// Обновляем статус
	status.Status = "processing"
	status.LastEventTime = time.Now()
	
	// Обработка
	err := job.Agent.Tick(ctx, job.Event)
	
	if err != nil {
		wp.totalErrors.Add(1)
		status.ErrorCount++
		if job.Done != nil {
			job.Done <- err
		}
	} else {
		wp.totalProcessed.Add(1)
		status.ProcessedCount++
		if job.Done != nil {
			job.Done <- nil
		}
	}
	
	// Возвращаем статус в idle
	status.Status = "idle"
}

// Submit добавляет задание в очередь
func (wp *WorkerPool) Submit(job Job) error {
	if wp.shutdown.Load() {
		return fmt.Errorf("worker pool is shutdown")
	}
	
	select {
	case wp.jobChan <- job:
		return nil
	default:
		return fmt.Errorf("worker pool queue is full")
	}
}

// SubmitAsync добавляет задание без ожидания
func (wp *WorkerPool) SubmitAsync(job Job) {
	select {
	case wp.jobChan <- job:
	default:
		// Очередь полная, игнорируем
		// TODO: добавить backpressure или fallback
	}
}

// ProcessBatch обрабатывает batch событий
func (wp *WorkerPool) ProcessBatch(ctx context.Context, events []Event, agent Agent) error {
	for _, event := range events {
		job := Job{
			Agent:     agent,
			Event:     event,
			Timestamp: time.Now(),
			Done:      make(chan error, 1),
		}
		
		if err := wp.Submit(job); err != nil {
			return fmt.Errorf("submit job: %w", err)
		}
	}
	
	return nil
}

// Shutdown завершает работу воркеров
func (wp *WorkerPool) Shutdown(ctx context.Context) error {
	wp.shutdown.Store(true)
	
	// Закрываем канал заданий
	close(wp.jobChan)
	
	// Ждем завершения
	wp.wg.Wait()
	
	return nil
}

// WaitCompletion ждет завершения всех текущих заданий
func (wp *WorkerPool) WaitCompletion(ctx context.Context) error {
	// TODO: реализовать ожидание
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Status возвращает статус пула
func (wp *WorkerPool) Status() PoolStatus {
	return PoolStatus{
		WorkersCount:      wp.workers,
		TotalProcessed:    wp.totalProcessed.Load(),
		TotalErrors:       wp.totalErrors.Load(),
		StartedAt:         wp.startedAt,
		Uptime:            time.Since(wp.startedAt),
		WorkerStatuses:    wp.workerStatuses,
		QueueCapacity:     cap(wp.jobChan),
		QueueDepth:        len(wp.jobChan),
	}
}

// PoolStatus статус пула воркеров
type PoolStatus struct {
	WorkersCount    int         `json:"workers_count"`
	TotalProcessed  int64       `json:"total_processed"`
	TotalErrors     int64       `json:"total_errors"`
	StartedAt       time.Time   `json:"started_at"`
	Uptime          time.Duration `json:"uptime"`
	WorkerStatuses  []WorkerStatus `json:"worker_statuses"`
	QueueCapacity   int         `json:"queue_capacity"`
	QueueDepth      int         `json:"queue_depth"`
}

// Statistics возвращает статистику
func (wp *WorkerPool) Statistics() map[string]interface{} {
	status := wp.Status()
	return map[string]interface{}{
		"workers_count":       status.WorkersCount,
		"total_processed":     status.TotalProcessed,
		"total_errors":        status.TotalErrors,
		"error_rate":          float64(status.TotalErrors) / float64(status.TotalProcessed),
		"uptime_seconds":      status.Uptime.Seconds(),
		"queue_capacity":      status.QueueCapacity,
		"queue_depth":         status.QueueDepth,
		"queue_utilization":   float64(status.QueueDepth) / float64(status.QueueCapacity),
	}
}

// DynamicScale динамически масштабирует пул
func (wp *WorkerPool) DynamicScale(ctx context.Context, targetSize int) error {
	// TODO: реализовать динамическое масштабирование
	// Добавить или удалить воркеры по load
	return nil
}

// AdaptiveTune настраивает параметры пула
func (wp *WorkerPool) AdaptiveTune() {
	status := wp.Status()
	
	// Если очередь заполняется > 80%, увеличиваем воркеров
	if float64(status.QueueDepth)/float64(status.QueueCapacity) > 0.8 {
		// TODO: увеличить количество воркеров
	}
	
	// Если очередь пустая > 2 минуты, уменьшить воркеров
	if status.QueueDepth == 0 && time.Since(status.StartedAt) > 2*time.Minute {
		// TODO: уменьшить количество воркеров
	}
}

// Metrics возвращает метрики для мониторинга
func (wp *WorkerPool) Metrics() map[string]float64 {
	stats := wp.Statistics()
	
	return map[string]float64{
		"workers":                  float64(stats["workers_count"].(int)),
		"total_processed":          float64(stats["total_processed"].(int64)),
		"total_errors":             float64(stats["total_errors"].(int64)),
		"uptime_seconds":           stats["uptime_seconds"].(float64),
		"queue_utilization":        stats["queue_utilization"].(float64),
	}
}
