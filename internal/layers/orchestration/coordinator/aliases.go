package coordinator

import "github.com/devrix/devrix/internal/layers/orchestration/workmodel"

type (
	Task         = workmodel.Task
	TaskManager  = workmodel.TaskManager
	TaskStore    = workmodel.TaskStore
	DiskTaskStore = workmodel.DiskTaskStore
)

var (
	NewTask           = workmodel.NewTask
	NewTaskManager    = workmodel.NewTaskManager
	NewDiskTaskStore  = workmodel.NewDiskTaskStore
)
