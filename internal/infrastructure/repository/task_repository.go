package repository

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/google/uuid"
)

type TaskRepository struct {
	filePath string
	mu       sync.RWMutex
	tasks    map[string]*entities.ScheduledTask
}

func NewTaskRepository(dataDir string) (*TaskRepository, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	repo := &TaskRepository{
		filePath: dataDir + "/scheduled_tasks.json",
		tasks:    make(map[string]*entities.ScheduledTask),
	}

	// 加载已存在的任务
	if err := repo.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load tasks: %w", err)
	}

	return repo, nil
}

// load 从文件加载任务
func (r *TaskRepository) load() error {
	data, err := ioutil.ReadFile(r.filePath)
	if err != nil {
		return err
	}

	var tasks []*entities.ScheduledTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks = make(map[string]*entities.ScheduledTask)
	for _, task := range tasks {
		r.tasks[task.ID] = task
	}

	return nil
}

// save 保存任务到文件
func (r *TaskRepository) save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.saveUnlocked()
}

// saveUnlocked 保存任务到文件（内部使用，调用时必须已经持有锁）
func (r *TaskRepository) saveUnlocked() error {
	tasks := make([]*entities.ScheduledTask, 0, len(r.tasks))
	for _, task := range r.tasks {
		tasks = append(tasks, task)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(r.filePath, data, 0644)
}

// Create 创建新任务
func (r *TaskRepository) Create(task *entities.ScheduledTask) error {
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
	return r.saveUnlocked()
}

// Update 更新任务
func (r *TaskRepository) Update(task *entities.ScheduledTask) error {
	task.UpdatedAt = time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tasks[task.ID]; !exists {
		return fmt.Errorf("task not found: %s", task.ID)
	}
	r.tasks[task.ID] = task
	return r.saveUnlocked()
}

// Delete 删除任务
func (r *TaskRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tasks, id)
	return r.saveUnlocked()
}

// GetByID 根据ID获取任务
func (r *TaskRepository) GetByID(id string) (*entities.ScheduledTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, exists := r.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	return task, nil
}

// GetAll 获取所有任务
func (r *TaskRepository) GetAll() ([]*entities.ScheduledTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tasks := make([]*entities.ScheduledTask, 0, len(r.tasks))
	for _, task := range r.tasks {
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetByUserID 获取用户创建的任务
func (r *TaskRepository) GetByUserID(userID int64) ([]*entities.ScheduledTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tasks := make([]*entities.ScheduledTask, 0)
	for _, task := range r.tasks {
		if task.CreatedBy == userID {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// UpdateLastRunTime 更新最后运行时间
func (r *TaskRepository) UpdateLastRunTime(id string, runTime time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, exists := r.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.LastRunAt = &runTime
	task.UpdatedAt = time.Now()

	return r.saveUnlocked()
}

// UpdateNextRunTime 更新下次运行时间
func (r *TaskRepository) UpdateNextRunTime(id string, nextTime time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, exists := r.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.NextRunAt = &nextTime
	task.UpdatedAt = time.Now()

	return r.saveUnlocked()
}
