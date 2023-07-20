package placeholders

import (
	"fmt"
	"path/filepath"
	"strings"

	api_v1 "github.com/vision-cli/api/v1"
	"github.com/vision-cli/common/transpiler/model"
)

type Placeholders struct {
	RegistryServer string
	*api_v1.PluginPlaceholders
}

const (
	ArgsCommandIndex = 0
	ArgsNameIndex    = 1
)

func SetupPlaceholders(req api_v1.PluginRequest) (*Placeholders, error) {
	if err := model.NameErr(req.Args[ArgsNameIndex]); err != nil {
		return nil, err
	}
	serviceName := req.Args[ArgsNameIndex]
	registryComponents := strings.Split(req.Placeholders.Registry, "/")
	if len(registryComponents) == 0 {
		return nil, fmt.Errorf("invalid registry server: %s", req.Placeholders.Registry)
	}
	p := &Placeholders{
		RegistryServer:     registryComponents[0],
		PluginPlaceholders: &req.Placeholders,
	}
	p.ServiceName = serviceName
	p.ServiceDirectory = filepath.Join(
		req.Placeholders.ServicesDirectory,
		req.Placeholders.ServiceNamespace,
		serviceName)
	p.ServiceFqn = filepath.Join(
		req.Placeholders.ServiceFqn,
		serviceName)
	return p, nil
}
