package run

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vision-cli/common/execute"
	"github.com/vision-cli/common/file"
	"github.com/vision-cli/common/module"
	"github.com/vision-cli/common/tmpl"
	"github.com/vision-cli/common/workspace"
	"github.com/vision-cli/vision-plugin-service-v1/placeholders"
	"github.com/vision-cli/vision-plugin-service-v1/svc"
)

const (
	goTemplateDir = "_templates/go"
	workflowDir   = ".github/workflows"
)

//go:embed all:_templates/go
var templateFiles embed.FS

func Create(p *placeholders.Placeholders, executor execute.Executor, t tmpl.TmplWriter) error {
	var err error

	if file.Exists(p.ServiceDirectory) {
		return fmt.Errorf("service %q already exists", p.ServiceName)
	}

	if err = tmpl.GenerateFS(templateFiles, goTemplateDir, p.ServiceDirectory, p, false, t); err != nil {
		return fmt.Errorf("generating the service structure from the template: %w", err)
	}

	if err = generateGoFiles(p.ServiceDirectory, executor); err != nil {
		return fmt.Errorf("generating go files with target dir: [%s]: %w", p.ServiceDirectory, err)
	}

	if err = genWorkflow(p); err != nil {
		return fmt.Errorf("generating service workflow with target dir: [%s]: %w", p.ServiceDirectory, err)
	}

	if err = workspace.Use(".", p.ServicesDirectory, executor); err != nil {
		return fmt.Errorf("adding service to workspace: %w", err)
	}

	return nil
}

func generateGoFiles(serviceDir string, executor execute.Executor) error {
	var err error

	if err = svc.CleanProto(serviceDir); err != nil {
		return fmt.Errorf("cleaning proto dir: %w", err)
	}

	if err = svc.GenProto(serviceDir, false, executor); err != nil {
		return fmt.Errorf("generating files in proto dir: %w", err)
	}

	if err = module.Tidy(serviceDir, executor); err != nil {
		return fmt.Errorf("tidying module: %w", err)
	}

	return nil
}

//go:embed _templates/workflows/go.yml.tmpl
var goWorkflow string

func genWorkflow(p *placeholders.Placeholders) error {
	workflowName := svc.WorkflowName(p.ServiceNamespace, p.ServiceName)

	if err := Generate(goWorkflow, workflowDir, workflowName, p); err != nil {
		return fmt.Errorf("generating service workflow: %w", err)
	}

	return nil
}

// Generate writes template to filename in the targetDir, substituting placeholder values.
// Any existing files will be overwritten.
func Generate(template string, targetDir string, filename string, p any) error {
	t, err := tmpl.New(targetDir, template)
	if err != nil {
		return fmt.Errorf("parsing template file: %w", err)
	}

	if err = file.CreateDir(targetDir); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}
	newF, err := os.Create(filepath.Join(targetDir, filename))
	if err != nil {
		return fmt.Errorf("creating target file: %w", err)
	}
	defer newF.Close()

	if err = t.Execute(newF, p); err != nil {
		return fmt.Errorf("writing contents to %s: %w", filename, err)
	}

	return nil
}
