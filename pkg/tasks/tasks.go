// tasks defines the task interface, a task should be able
package tasks

import (
	"context"
	"fmt"
	"reflect"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
	"github.com/controlplaneio/sandbox-probe/pkg/models"
)

type baseTask struct {
	Task
	name        string
	description string
}

func (t *baseTask) GetName() string {
	return t.name
}

func (t *baseTask) GetDescription() string {
	return t.description
}

const (
	WRITEABLEPATHS            = "writeable_paths"
	SENSITIVEREADABLEPATHS    = "sensitive_readable_paths"
	EXTERNALHOSTDNSRESOLUTION = "external_host_dns_resolution"
	EXTERNALHOSTCONNECTIVITY  = "external_host_connectivity"
	UDPPORTSOPEN              = "udp_ports_open"
	TCPPORTSOPEN              = "tcp_ports_open"
	PROXYDETECTION            = "proxy_detection"
	UNIXSOCKETDETECTION       = "unix_socket_detection"
	PROCESSDETECTION          = "process_detection"
	PARENTPROCESSDETECTION    = "parent_process_detection"
	MOUNTEDVOLUMESDETECTION   = "mounted_volumes_detections"
	USERCONTEXTDETECTION      = "user_context_detection"
	HOSTNAMEDETECTION         = "hostname_detection"
	SANDBOXDETECTION          = "sandbox_detection"
)

var expectedTypes = map[string]reflect.Type{
	WRITEABLEPATHS:            reflect.TypeOf([]string{}),
	SENSITIVEREADABLEPATHS:    reflect.TypeOf([]string{}),
	EXTERNALHOSTDNSRESOLUTION: reflect.TypeOf([]string{}),
	EXTERNALHOSTCONNECTIVITY:  reflect.TypeOf([]string{}),
	UDPPORTSOPEN:              reflect.TypeOf([]int{}),
	TCPPORTSOPEN:              reflect.TypeOf([]int{}),
	PROXYDETECTION:            reflect.TypeOf(&models.ProxyConfig{}),
	UNIXSOCKETDETECTION:       reflect.TypeOf([]string{}),
	PROCESSDETECTION:          reflect.TypeOf(&models.Process{}),
	PARENTPROCESSDETECTION:    reflect.TypeOf(&models.Process{}),
	MOUNTEDVOLUMESDETECTION:   reflect.TypeOf([]string{}),
	USERCONTEXTDETECTION:      reflect.TypeOf(&models.UserIdentity{}),
	HOSTNAMEDETECTION:         reflect.TypeOf(""),
	SANDBOXDETECTION:          reflect.TypeOf(""),
}

// Registry with wrapper functions
var taskRegistry = map[string]func() Task{
	"baseline_path_task":         func() Task { return NewPathTask() },
	"baseline_network_task":      func() Task { return NewNetworkTask() },
	"baseline_proxy_task":        func() Task { return NewProxyTask() },
	"baseline_socket_task":       func() Task { return NewSocketTask() },
	"baseline_process_task":      func() Task { return NewProcessTask() },
	"baseline_user_context_task": func() Task { return NewUserContextTask() },
	"baseline_hostname_task":     func() Task { return NewHostnameTask() },
	"baseline_sandbox_task":      func() Task { return NewSandboxTask() },
	"baseline_mount_task":        func() Task { return NewMountTask() },
	"ps_all_task":                func() Task { return NewPSAllTask() },
	"ps_parent_task":             func() Task { return NewPSParentTask() },
	"ps_single_task":             func() Task { return NewPSSingleTask() },
}

// Registry with wrapper functions
var taskSetRegistry = map[string]func() []Task{
	"baseline": func() []Task { return GetBaselineTasks() },
	"ps":       func() []Task { return GetPSTasks() },
}

type Task interface {
	// GetName returns the name of the task
	GetName() string
	// GetDescription returns the description of the task
	GetDescription() string
	// Run executes the task producing Findings
	Run(ctx context.Context) ([]*reportv1.Finding, error)
}

func Validate(f *reportv1.Finding) error {
	if f.Value == nil {
		return fmt.Errorf("%s value cannot be nil", f.FindingType)
	}

	// Convert structpb.Value to native Go value for type checking
	nativeValue := f.Value.AsInterface()

	if t, ok := expectedTypes[f.FindingType]; ok {
		if reflect.TypeOf(nativeValue) != t {
			return fmt.Errorf("%s value must be %s, got %T", f.FindingType, t, nativeValue)
		}
		return nil
	}

	return nil
}

// GetTasksByName returns the given task
func GetTasksByName(names []string) ([]Task, error) {
	var tasksByNames []Task
	for _, name := range names {
		task, err := getTaskByName(name)
		if err != nil {
			return nil, err
		}
		tasksByNames = append(tasksByNames, task)
	}
	return tasksByNames, nil
}

// getTaskByName returns the given task
func getTaskByName(name string) (Task, error) {
	task, ok := taskRegistry[name]
	if !ok {
		return nil, fmt.Errorf("Task with name '%s' doesn't exist. Valid tasks are %v", name, GetAllTasksNames())
	}
	return task(), nil
}

// GetAllTasksNames returns all the tasks names
func GetAllTasksNames() []string {
	var names []string
	for name, _ := range taskRegistry {
		names = append(names, name)
	}
	return names
}

// GetAllTaskSetsNames returns all the tasksets names
func GetAllTaskSetsNames() []string {
	var names []string
	for name, _ := range taskSetRegistry {
		names = append(names, name)
	}
	// all and none are a special keywords
	names = append(names, "all", "none")
	return names
}

func GetTaskSetsTasks(taskSets []string) ([]Task, error) {
	var taskSetsTasks []Task
	for _, taskset := range taskSets {
		taskSetTasks, err := getTaskSetTasks(taskset)
		if err != nil {
			return nil, err
		}
		taskSetsTasks = append(taskSetsTasks, taskSetTasks...)
	}
	return taskSetsTasks, nil
}

// GetTaskSetTasks gets all tasks for a given taskset
func getTaskSetTasks(taskset string) ([]Task, error) {
	if taskSetTasks, ok := taskSetRegistry[taskset]; ok {
		return taskSetTasks(), nil
		// special keyword all to return all tasks
	} else if taskset == "all" {
		var allTasks []Task
		for _, getTasksFn := range taskSetRegistry {
			allTasks = append(allTasks, getTasksFn()...)
		}
		return allTasks, nil
		// special keyword none to return no tasks
	} else if taskset == "none" {
		return []Task{}, nil
	}

	return nil, fmt.Errorf("Taskset with name '%s' doesn't exists. Valid names are %v", taskset, GetAllTaskSetsNames())
}
