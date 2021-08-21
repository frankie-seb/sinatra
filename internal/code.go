package internal

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/mod/modfile"
)

var modregex = regexp.MustCompile(`module ([^\s]*)`)

var gopaths []string

func init() {
	gopaths = filepath.SplitList(build.Default.GOPATH)
	for i, p := range gopaths {
		gopaths[i] = filepath.ToSlash(filepath.Join(p, "src"))
	}
}

// goModuleRoot returns the root of the current go module if there is a go.mod file in the directory tree
// If not, it returns false
func goModuleRoot(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	dir = filepath.ToSlash(dir)
	modDir := dir
	assumedPart := ""
	for {
		f, err := ioutil.ReadFile(filepath.Join(modDir, "go.mod"))
		if err == nil {
			// found it, stop searching
			return string(modregex.FindSubmatch(f)[1]) + assumedPart, true
		}

		assumedPart = "/" + filepath.Base(modDir) + assumedPart
		parentDir, err := filepath.Abs(filepath.Join(modDir, ".."))
		if err != nil {
			panic(err)
		}

		if parentDir == modDir {
			// Walked all the way to the root and didnt find anything :'(
			break
		}
		modDir = parentDir
	}
	return "", false
}

// ImportPathForDir takes a path and returns a golang import path for the package
func ImportPathForDir(dir string) (res string) {
	dir, err := filepath.Abs(dir)

	if err != nil {
		panic(err)
	}
	dir = filepath.ToSlash(dir)

	modDir, ok := goModuleRoot(dir)
	if ok {
		return modDir
	}

	for _, gopath := range gopaths {
		if len(gopath) < len(dir) && strings.EqualFold(gopath, dir[0:len(gopath)]) {
			return dir[len(gopath)+1:]
		}
	}

	return ""
}

func GetFunctionNamesFromDir(dir string, ignore []string) ([]string, error) {
	var a []string
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, dir, nil, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to parse package: %v", err)
	}

	for _, pack := range packs {
		for fileName, file := range pack.Files {
			simpleName := strings.TrimPrefix(fileName, dir+"/")
			if !Contains(ignore, simpleName) {
				a = append(a, GetFunctionNamesFromAstFile(file)...)
			}
		}
	}
	return a, nil
}

func GetFunctionNamesFromAstFile(node *ast.File) []string {
	var a []string

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if ok {
			a = append(a, fn.Name.Name)
		}
		return true
	})
	return a
}

func GetResolverFunctionNamesFromDir(dir string, ignore []string) ([]string, error) {
	var a []string
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, dir, nil, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to parse package: %v", err)
	}

	for _, pack := range packs {
		for fileName, file := range pack.Files {
			simpleName := strings.TrimPrefix(fileName, dir+"/")
			if !Contains(ignore, simpleName) {
				a = append(a, GetResolverFunctionNamesFromAstFile(file)...)
			}
		}
	}
	return a, nil
}

func GetResolverFunctionNamesFromAstFile(node *ast.File) []string {
	var a []string

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if ok {
			include := false
			if fn.Recv != nil {
				for _, r := range fn.Recv.List {
					s := fmt.Sprintf("%s", r.Type)
					include = strings.Contains(s, "queryResolver") || strings.Contains(s, "mutationResolver")
				}
				if include {
					a = append(a, fn.Name.Name)
				}
			}
		}
		return true
	})
	return a
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

var pathRegex *regexp.Regexp //nolint:gochecknoglobals

func GetRootImportPath() string {
	importPath, err := rootImportPath()
	if err != nil {
		log.Err(err).Msg(
			"could not detect root import path %v")
		return ""
	}
	return importPath
}

func rootImportPath() (string, error) {
	projectPath, err := getWorkingPath()
	if err != nil {
		// TODO: adhering to your original error handling
		//  should consider doing something here rather than continuing
		//  since this step occurs during generation, panicing or fatal error should be okay
		return "", fmt.Errorf("error while getting working directory %w", err)
	}
	if hasGoMod(projectPath) {
		modulePath, err := getModulePath(projectPath)
		if err != nil {
			// TODO: adhering to your original error handling
			//  should consider doing something here rather than continuing
			//  since this step occurs during generation, panicing or fatal error should be okay
			return "", fmt.Errorf("error while getting module path %w", err)
		}
		return modulePath, nil
	}

	return gopathImport(projectPath), nil
}

// getWorkingPath gets the current working directory
func getWorkingPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return wd, nil
}

func hasGoMod(projectPath string) bool {
	filePath := path.Join(projectPath, "go.mod")
	return FileExists(filePath)
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func getModulePath(projectPath string) (string, error) {
	filePath := path.Join(projectPath, "go.mod")
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error while trying to read go mods path %w", err)
	}

	modPath := modfile.ModulePath(file)
	if modPath == "" {
		return "", fmt.Errorf("could not determine mod path")
	}
	return modPath, nil
}

func gopathImport(dir string) string {
	return strings.TrimPrefix(pathRegex.FindString(dir), "src/")
}

func AppendIfMissing(slice []string, v string) []string {
	if SliceContains(slice, v) {
		return slice
	}
	return append(slice, v)
}

func SliceContains(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}
