package svc

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vision-cli/common/cases"
	"github.com/vision-cli/common/execute"
	"github.com/vision-cli/common/file"
)

const (
	ProtoDir     = "proto"
	defaultProto = "service.proto"
	Handlers     = "handlers/handlers.go"
	manifests    = "manifests.yml"
	nameSize     = 50
)

var (
	protoVersionExp = regexp.MustCompile(`.(v[0-9]+);`)
	rpcServiceExp   = regexp.MustCompile(`service ([a-zA-Z0-9]+)`)
	rpcMethodExp    = regexp.MustCompile(`rpc ([a-zA-Z0-9]+)`)
)

var Fsglob = fs.Glob
var Osrename = os.Rename
var Filepathwalkdir = filepath.WalkDir

func ImageRegistry(image string) string {
	i := strings.LastIndex(image, "/")
	return image[:i]
}

func ImageProject(image string) string {
	i := strings.LastIndex(image, "/")
	return regexp.MustCompile(`[a-z0-9-]+`).FindString(image[i:])
}

// FindImage returns the image found in manifests.yml
func FindImage(serviceDir string, executor execute.Executor) (string, error) {
	query := `select(.kind == "Deployment") | .spec.template.spec.containers[0].image`
	yq := exec.Command("yq", query, manifests)

	image, err := executor.Output(yq, serviceDir, "searching for service image")
	if err != nil {
		return "", err
	}
	image = strings.TrimSpace(image)

	if !regexp.MustCompile(`.+/([a-z0-9-]+\.){2}[a-z0-9-]+:\w[\w.-]*`).MatchString(image) {
		return "", fmt.Errorf("image %q incorrectly formated", image)
	}

	return image, nil
}

func FindProtoVersion(serviceDir string) (string, error) {
	f, err := firstProto(serviceDir)
	if err != nil {
		return "", fmt.Errorf("finding first proto file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "package") {
			match := protoVersionExp.FindStringSubmatch(line)
			if len(match) == 0 {
				return "", fmt.Errorf("package version not declared in %s", f.Name())
			}
			return match[1], nil
		}
	}
	return "", fmt.Errorf("version not found in %s", f.Name())
}

// FindProtoMethods returns names of all methods declared in
func FindProtoMethods(serviceDir string) (map[string][]string, int, error) {
	f, err := firstProto(serviceDir)
	if err != nil {
		return nil, 0, fmt.Errorf("finding first proto file: %w", err)
	}
	defer f.Close()

	methods := make(map[string][]string)
	count := 0

	scanner := bufio.NewScanner(f)
	rpcService := ""
	for scanner.Scan() {
		line := scanner.Text()

		match := rpcServiceExp.FindStringSubmatch(line)
		if len(match) > 0 {
			rpcService = match[1]
		}

		method := rpcMethodExp.FindStringSubmatch(line)
		if len(method) > 0 {
			methods[rpcService] = append(methods[rpcService], method[1])
			count++
		}
	}

	return methods, count, nil
}

// CleanProto removes any generated protobuf go files and ensures the correct name for .proto file.
func CleanProto(serviceDir string) error {
	protoPath := filepath.Join(serviceDir, ProtoDir)

	// 	find existing proto file
	files, err := protoFiles(protoPath)
	if err != nil {
		return err
	}

	// avoid overwriting existing user proto
	if len(files) > 1 {
		if files[0] == defaultProto {
			files = files[1:]
		}
	}

	// rename the proto to namespace/service
	parts := strings.Split(serviceDir, string(os.PathSeparator))
	if len(parts) <= 1 {
		return fmt.Errorf("unable to ascertain service namespace")
	}
	newName := protoName(parts[len(parts)-2], parts[len(parts)-1])

	err = Osrename(filepath.Join(protoPath, files[0]), filepath.Join(protoPath, newName))
	if err != nil {
		return fmt.Errorf("renaming service proto file: %w", err)
	}

	// delete old associated go files
	err = Filepathwalkdir(protoPath, func(path string, d fs.DirEntry, err error) error {
		if filepath.Ext(d.Name()) == ".go" || d.Name() == defaultProto {
			if err = os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("deleting go files in %s: %w", protoPath, err)
	}

	return nil
}

func firstProto(serviceDir string) (*os.File, error) {
	protoPath := filepath.Join(serviceDir, ProtoDir)

	files, err := protoFiles(protoPath)
	if err != nil {
		return nil, fmt.Errorf("finding service proto: %w", err)
	}

	f, err := file.Open(filepath.Join(protoPath, files[0]))
	if err != nil {
		return nil, fmt.Errorf("opening first proto file in %s", protoPath)
	}
	return f, nil
}

// protofiles returns filenames of all .proto files in protoPath
func protoFiles(protoPath string) ([]string, error) {
	files, err := Fsglob(os.DirFS(protoPath), "*.proto")
	if err != nil {
		return nil, fmt.Errorf("finding proto files in %s: %w", protoPath, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no proto files found in %s", protoPath)
	}

	return files, nil
}

func HasProto(serviceDir string) bool {
	protoPath := filepath.Join(serviceDir, ProtoDir)
	return file.Exists(protoPath)
}

func HasProxy(serviceDir string) bool {
	if !HasProto(serviceDir) {
		return false
	}

	protoPath := filepath.Join(serviceDir, ProtoDir)

	gwFiles, err := fs.Glob(os.DirFS(protoPath), "*.gw.go")
	if err != nil || len(gwFiles) == 0 {
		return false
	}
	return true
}

func GenProto(serviceDir string, expose bool, executor execute.Executor) error {
	var err error

	gen := exec.Command("make", "proto")
	if err = executor.Errors(gen, serviceDir, "generating protobuf files"); err != nil {
		return err
	}

	if expose {
		proxy := exec.Command("make", "proxy")
		if err = executor.Errors(proxy, serviceDir, "generating protobuf gateway files"); err != nil {
			return err
		}
	}

	return nil
}

func protoName(namespace, serviceName string) string {
	return fmt.Sprintf("%s_%s.proto", cases.Snake(namespace), cases.Snake(serviceName))
}

func WorkflowName(namespace, serviceName string) string {
	return fmt.Sprintf("%s-%s.yml", namespace, serviceName)
}

func ProjectName() string {
	path, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to retrieve working directory: %v", err)
	}

	name := cases.Kebab(filepath.Base(path))
	if name == "" {
		log.Fatalf("creating project name from cwd (%s)", name)
	}
	return name
}

// WalkAll calls serviceFunc for every service found in the RootServiceDir
func WalkAll(RootServicesDir string, serviceFunc func(fullPath, namespace, serviceName string) error) error {
	return fs.WalkDir(os.DirFS(RootServicesDir), ".",
		func(path string, d fs.DirEntry, err error) error {
			// entries directly under RootServicesDir (e.g. namespace dir) have depth 0
			targetDepth := 1
			depth := strings.Count(path, string(os.PathSeparator))
			fullPath := filepath.Join(RootServicesDir, path)

			// report permissions interference
			if errors.Is(err, fs.ErrPermission) {
				return fs.SkipDir
			}

			// avoid extra calls to same service
			if depth > targetDepth {
				return fs.SkipDir
			}
			if !d.IsDir() || depth != targetDepth {
				return nil
			}

			// path should always be <namespace>/<serviceName> (depth 1)
			parts := strings.Split(path, string(os.PathSeparator))
			return serviceFunc(fullPath, parts[0], parts[1])
		})
}

func HasHandlers(serviceDir string) bool {
	return file.Exists(filepath.Join(serviceDir, Handlers))
}
