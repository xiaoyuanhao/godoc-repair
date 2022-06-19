package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

const (
	defaultCommentFormat  = "// %s missing godoc."
	autoDescriptionFormat = "// %s %s"
)

var (
	commentFormat   string
	codePath        string
	autoDescription bool
)

func init() {
	flag.StringVar(&commentFormat, "format", defaultCommentFormat, "comment format")
	flag.StringVar(&codePath, "code-path", "", "code path")
	flag.BoolVar(&autoDescription, "auto-description", false, "enable auto description")
	flag.Parse()
}

func main() {
	// get the current working directory if code path is empty
	if codePath == "" {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("error getting current working directory: %v", err)
		}
		codePath = wd
	}
	log.Print(fmt.Sprintf("Adding default go doc to each exported type/func recursively in %s", codePath))

	//
	if err := mapDirectory(codePath, instrumentDir); err != nil {
		log.Fatalf("error while instrumenting current working directory: %v", err)
	}
}

func instrumentDir(path string) error {
	fset := token.NewFileSet()
	filter := func(info os.FileInfo) bool {
		return testsFilter(info) && generatedFilter(path, info)
	}
	pkgs, err := parser.ParseDir(fset, path, filter, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed parsing go files in directory %s: %v", path, err)
	}

	for _, pkg := range pkgs {
		if err := instrumentPkg(fset, pkg); err != nil {
			return err
		}
	}
	return nil
}

func instrumentPkg(fset *token.FileSet, pkg *ast.Package) error {
	for fileName, file := range pkg.Files {
		sourceFile, err := os.OpenFile(fileName, os.O_TRUNC|os.O_WRONLY, 0664)
		if err != nil {
			return fmt.Errorf("failed opening file %s: %v", fileName, err)
		}
		if err := instrumentFile(fset, file, sourceFile); err != nil {
			return fmt.Errorf("failed instrumenting file %s: %v", fileName, err)
		}
	}
	return nil
}

func instrumentFile(fset *token.FileSet, file *ast.File, out io.Writer) error {
	// Needed because ast does not support floating comments and deletes them.
	// In order to preserve all comments we just pre-parse it to dst which treats them as first class citizens.
	f, err := decorator.DecorateFile(fset, file)
	if err != nil {
		return fmt.Errorf("failed converting file from ast to dst: %v", err)
	}

	dst.Inspect(f, func(n dst.Node) bool {
		switch t := n.(type) {
		case *dst.FuncDecl:
			t.Decs.Start = autoDecl(t.Name, t.Decs.Start)
		case *dst.GenDecl:
			if len(t.Specs) == 1 {
				switch s := t.Specs[0].(type) {
				case *dst.TypeSpec:
					t.Decs.Start = autoDecl(s.Name, t.Decs.Start)
					return true
				case *dst.ValueSpec:
					t.Decs.Start = autoDecl(s.Names[0], t.Decs.Start)
					return true
				default:
					return true
				}
			}
			for _, spec := range t.Specs {
				switch s := spec.(type) {
				case *dst.TypeSpec:
					s.Decs.Start = autoDecl(s.Name, s.Decs.Start)
				case *dst.ValueSpec:
					s.Decs.Start = autoDecl(s.Names[0], s.Decs.Start)
				}
			}
		}
		return true
	})
	return decorator.Fprint(out, f)
}

func autoDecl(ident *dst.Ident, decorations dst.Decorations) dst.Decorations {
	if !ident.IsExported() {
		return decorations
	}

	doc := fmt.Sprintf(defaultCommentFormat, ident.Name)
	if autoDescription {
		doc = fmt.Sprintf(autoDescriptionFormat, ident.Name, mockDoc(ident.Name))
	}
	empty, emptyName, justName := containsGoDoc(decorations.All(), ident.Name)
	if empty {
		decorations.Prepend(doc)
	}
	if emptyName {
		all := decorations.All()
		first := all[0]
		first = trimPrefix(first, ident.Name)
		first = fmt.Sprintf("// %s %s", ident.Name, first)
		all[0] = first
		decorations.Replace(all...)
	}
	if justName {
		all := decorations.All()
		all[0] = doc
		decorations.Replace(all...)
	}
	return decorations
}

// return (empty, emptyName, justName)
func containsGoDoc(decs []string, name string) (bool, bool, bool) {
	if len(decs) == 0 {
		return true, false, false
	}
	first := decs[0]
	if first == fmt.Sprintf("// %s", name) || first == fmt.Sprintf("//%s", name) {
		return false, false, true
	}
	if !strings.HasPrefix(first, fmt.Sprintf("// %s ", name)) {
		return false, true, false
	}
	return false, false, false
}

func trimPrefix(doc, name string) string {
	cases := []string{
		// trim '//Name '
		fmt.Sprintf("//%s ", name),
		// trim '// Name: '
		fmt.Sprintf("// %s: ", name),
		// trim '// Name:'
		fmt.Sprintf("// %s:", name),
		// trim '//Name: '
		fmt.Sprintf("//%s: ", name),
		// trim '//Name:'
		fmt.Sprintf("//%s:", name),
		// trim '// '
		fmt.Sprintf("// "),
		// trim '//'
		fmt.Sprintf("//"),
	}
	for _, c := range cases {
		if strings.HasPrefix(doc, c) {
			return strings.TrimPrefix(doc, c)
		}
	}
	return doc
}

// mock doc, split the Name to single word
func mockDoc(name string) string {
	results := Split(name)
	for i, r := range results {
		results[i] = strings.ToLower(r)
	}
	return strings.Join(results, " ")
}

// Filter excluding go test files from directory
func testsFilter(info os.FileInfo) bool {
	return !strings.HasSuffix(info.Name(), "_test.go")
}

// Filter excluding generated go files from directory.
// Generated file is considered a file which matches one of the following:
// 1. The name of the file contains "generated"
// 2. First line of the file contains "generated" or "GENERATED"
func generatedFilter(path string, info os.FileInfo) bool {
	if strings.Contains(info.Name(), "generated") {
		return false
	}

	f, err := os.Open(path + "/" + info.Name())
	if err != nil {
		panic(fmt.Sprintf("Failed opening file %s: %v", path+"/"+info.Name(), err))
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan()
	line := scanner.Text()

	if strings.Contains(line, "generated") || strings.Contains(line, "GENERATED") {
		return false
	}
	return true
}

func mapDirectory(dir string, operation func(string) error) error {
	return filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Name() == "vendor" {
				return filepath.SkipDir
			}

			if info.IsDir() {
				return operation(path)
			}
			return nil
		})
}

// Split missing godoc.
func Split(src string) (entries []string) {
	// don't split invalid utf8
	if !utf8.ValidString(src) {
		return []string{src}
	}
	entries = []string{}
	var runes [][]rune
	lastClass := 0
	class := 0
	// split into fields based on class of unicode character
	for _, r := range src {
		switch true {
		case unicode.IsLower(r):
			class = 1
		case unicode.IsUpper(r):
			class = 2
		case unicode.IsDigit(r):
			class = 3
		default:
			class = 4
		}
		if class == lastClass {
			runes[len(runes)-1] = append(runes[len(runes)-1], r)
		} else {
			runes = append(runes, []rune{r})
		}
		lastClass = class
	}
	// handle upper case -> lower case sequences, e.g.
	// "PDFL", "oader" -> "PDF", "Loader"
	for i := 0; i < len(runes)-1; i++ {
		if unicode.IsUpper(runes[i][0]) && unicode.IsLower(runes[i+1][0]) {
			runes[i+1] = append([]rune{runes[i][len(runes[i])-1]}, runes[i+1]...)
			runes[i] = runes[i][:len(runes[i])-1]
		}
	}
	// construct []string from results
	for _, s := range runes {
		if len(s) > 0 {
			entries = append(entries, string(s))
		}
	}
	return
}
