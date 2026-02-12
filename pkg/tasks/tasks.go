// tasks defines the task interface, a task should be able
package tasks

import (
	"context"
	"fmt"
	"reflect"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
	"github.com/controlplaneio/sandbox-probe/pkg/models"
)

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

type Task interface {
	// GetName returns the name of the task
	GetName() string
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
