package plugin_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vision-cli/common/mocks"
	"github.com/vision-cli/vision-plugin-service-v1/plugin"
	"github.com/vision-cli/vision-plugin-service-v1/svc"
)

func TestHandle_WithValidUsageInput_ReturnsUsageResponseString(t *testing.T) {
	e := mocks.NewMockExecutor()
	tw := mocks.NewMockTmplWriter()
	result := plugin.Handle(CreateRequest(t, "usage"), &e, &tw)
	expected := `{"Version":"0.1.0","Use":"service","Short":"manage service","Long":"manage service using a standard template","Example":"vision service create myService","Subcommands":["create"],"Flags":[],"RequiresConfig":false}`
	assert.Equal(t, expected, result)
}

func TestHandle_WithValidConfigInput_ReturnsConfigResponseString(t *testing.T) {
	e := mocks.NewMockExecutor()
	tw := mocks.NewMockTmplWriter()
	result := plugin.Handle(CreateRequest(t, "config"), &e, &tw)
	expected := `{"Defaults":[]}`
	assert.Equal(t, expected, result)
}

func TestHandle_WithInValidInput_ReturnsErrorString(t *testing.T) {
	e := mocks.NewMockExecutor()
	tw := mocks.NewMockTmplWriter()
	result := plugin.Handle("X"+CreateRequest(t, "run"), &e, &tw)
	expected := `{"Result":"","Error":"invalid character 'X' looking for beginning of value"}`
	assert.Equal(t, expected, result)
}

func TestHandle_WithInValidCommand_ReturnsErrorString(t *testing.T) {
	e := mocks.NewMockExecutor()
	tw := mocks.NewMockTmplWriter()
	req := CreateRequest(t, "avengers")
	result := plugin.Handle(req, &e, &tw)
	expected := `{"Result":"","Error":"unknown command"}`
	assert.Equal(t, expected, result)
}

func TestHandle_WithValidRunInput_ReturnsRunResponseString(t *testing.T) {
	oldsvcfsglog := svc.Fsglob
	defer func() { svc.Fsglob = oldsvcfsglog }()
	svc.Fsglob = func(fsys fs.FS, pattern string) (matches []string, err error) {
		return []string{"default/file.proto"}, nil
	}

	oldosrename := svc.Osrename
	defer func() { svc.Osrename = oldosrename }()
	svc.Osrename = func(oldpath string, newpath string) error {
		return nil
	}

	oldfilepathwalkdir := svc.Filepathwalkdir
	defer func() { svc.Filepathwalkdir = oldfilepathwalkdir }()
	svc.Filepathwalkdir = func(root string, fn fs.WalkDirFunc) error {
		return nil
	}

	e := mocks.NewMockExecutor()
	tw := mocks.NewMockTmplWriter()
	req := CreateRequest(t, "run")
	req = strings.Replace(req, `"Args":[]`, `"Args":["create","mything"]`, 1)
	result := plugin.Handle(req, &e, &tw)
	expected := `{"Result":"SUCCESS!","Error":""}`
	assert.Equal(t, expected, result)
}

func CreateRequest(t *testing.T, command string) string {
	t.Helper()
	var testReq = `
{
	"Command":"` + command + `",
	"Args":[],
	"Flags":[],
	"Placeholders": {
		"ProjectRoot":"",
		"ProjectName":"",
		"ProjectDirectory":"",
		"ProjectFqn":"",
		"Registry":"",
		"Remote":"",
		"Branch":"",
		"Version":"",
		"ServicesFqn":"",
		"ServicesDirectory":"",
		"GatewayServiceName":"",
		"GatewayFqn":"",
		"GraphqlServiceName":"",
		"GraphqlFqn":"",
		"LibsFqn":"",
		"LibsDirectory":"",
		"ServiceNamespace":"default",
		"ServiceVersionedNamespace":"",
		"ServiceName":"",
		"ServiceFqn":"",
		"ServiceDirectory":"",
		"InfraDirectory":"",
		"ProtoPackage":""
		}
}	
`
	return testReq
}
