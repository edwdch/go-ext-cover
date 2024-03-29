package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"golang.org/x/tools/cover"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
)

func main() {
	app := &cli.App{
		Name:  "go-ext-cover",
		Usage: "go extension coverage tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "file",
				Aliases:     []string{"f"},
				Usage:       "coverage file",
				Required:    false,
				DefaultText: "coverage.out",
			},
			&cli.StringFlag{
				Name:        "outputFile",
				Aliases:     []string{"o"},
				Usage:       "output file",
				Required:    false,
				DefaultText: "coverage.json",
			},
			&cli.StringFlag{
				Name:     "outputDir",
				Aliases:  []string{"d"},
				Usage:    "output directory",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			fileName := getOrDefault(c, "file", "coverage.out")
			outputFile := getOrDefault(c, "outputFile", "coverage.json")
			dir := c.String("outputDir")

			if err := createDir(dir); err != nil {
				return err
			}

			profiles, err := cover.ParseProfiles(fileName)
			if err != nil {
				return err
			}

			// get overall coverage
			total, covered, err := getOverallCoverage(profiles)
			if err != nil {
				return err
			}

			coverage := Coverage{
				LineMissed:  total - covered,
				LineCovered: covered,
			}

			funcInfos, err := getFunctionInfos(profiles)
			if err != nil {
				return err
			}

			methodMissed := int64(0)
			methodCovered := int64(0)

			for _, info := range funcInfos {
				if info.isCovered {
					methodCovered++
				} else {
					methodMissed++
				}
			}

			coverage.MethodMissed = methodMissed
			coverage.MethodCovered = methodCovered

			// write to file
			file, err := json.MarshalIndent(coverage, "", "  ")
			if err != nil {
				return err
			}

			return ioutil.WriteFile(path.Join(dir, outputFile), file, 0644)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func getOrDefault(c *cli.Context, name string, defaultValue string) string {
	if c.IsSet(name) {
		return c.String(name)
	}
	return defaultValue
}

func createDir(dir string) error {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	return nil
}

func getOverallCoverage(profiles []*cover.Profile) (int64, int64, error) {
	var total, covered int64
	for _, profile := range profiles {
		for _, block := range profile.Blocks {
			total += int64(block.NumStmt)
			if block.Count > 0 {
				covered += int64(block.NumStmt)
			}
		}
	}
	return total, covered, nil
}

func getFunctionInfos(profiles []*cover.Profile) ([]*funcInfo, error) {
	var funcInfos []*funcInfo
	for _, profile := range profiles {
		fn := profile.FileName
		file, err := findFile(fn)
		if err != nil {
			return nil, err
		}
		funcs, err := findFuncs(file)
		if err != nil {
			return nil, err
		}
		// Now match up functions and profile blocks.
		for _, f := range funcs {
			c := f.coverage(profile)
			funcInfos = append(funcInfos,
				&funcInfo{fileName: file,
					functionName:      f.name,
					functionStartLine: f.startLine,
					functionEndLine:   f.endLine,
					isCovered:         c > 0})
		}
	}
	return funcInfos, nil
}

type Coverage struct {
	LineMissed    int64 `json:"lineMissed"`
	LineCovered   int64 `json:"lineCovered"`
	MethodMissed  int64 `json:"methodMissed"`
	MethodCovered int64 `json:"methodCovered"`
}

type funcInfo struct {
	fileName          string
	functionName      string
	functionStartLine int
	functionEndLine   int
	isCovered         bool
}

// findFuncs parses the file and returns a slice of FuncExtent descriptors.
func findFuncs(name string) ([]*FuncExtent, error) {
	fset := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fset, name, nil, 0)
	if err != nil {
		return nil, err
	}
	visitor := &FuncVisitor{
		fset:    fset,
		name:    name,
		astFile: parsedFile,
	}
	ast.Walk(visitor, visitor.astFile)
	return visitor.funcs, nil
}

// FuncExtent describes a function's extent in the source by file and position.
type FuncExtent struct {
	name      string
	startLine int
	startCol  int
	endLine   int
	endCol    int
}

// FuncVisitor implements the visitor that builds the function position list for a file.
type FuncVisitor struct {
	fset    *token.FileSet
	name    string // Name of file.
	astFile *ast.File
	funcs   []*FuncExtent
}

// Visit implements the ast.Visitor interface.
func (v *FuncVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		start := v.fset.Position(n.Pos())
		end := v.fset.Position(n.End())
		fe := &FuncExtent{
			name:      n.Name.Name,
			startLine: start.Line,
			startCol:  start.Column,
			endLine:   end.Line,
			endCol:    end.Column,
		}
		v.funcs = append(v.funcs, fe)
	}
	return v
}

// coverage returns the fraction of the statements in the function that were covered, as a numerator and denominator.
func (f *FuncExtent) coverage(profile *cover.Profile) (num int64) {
	// We could avoid making this n^2 overall by doing a single scan and annotating the functions,
	// but the sizes of the data structures is never very large and the scan is almost instantaneous.
	var covered int64
	// The blocks are sorted, so we can stop counting as soon as we reach the end of the relevant block.
	for _, b := range profile.Blocks {
		if b.StartLine > f.endLine || (b.StartLine == f.endLine && b.StartCol >= f.endCol) {
			// Past the end of the function.
			break
		}
		if b.EndLine < f.startLine || (b.EndLine == f.startLine && b.EndCol <= f.startCol) {
			// Before the beginning of the function
			continue
		}
		if b.Count > 0 {
			covered += int64(b.NumStmt)
		}
	}
	return covered
}

// findFile finds the location of the named file in GOROOT, GOPATH etc.
func findFile(file string) (string, error) {
	dir, file := filepath.Split(file)
	pkg, err := build.Import(dir, ".", build.FindOnly)
	if err != nil {
		return "", fmt.Errorf("can't find %q: %v", file, err)
	}
	return filepath.Join(pkg.Dir, file), nil
}
