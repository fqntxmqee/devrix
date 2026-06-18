// Command ci-lint-invariant — DM-20260618-007 W15 CI lint tool。
//
// 扫描 internal/layers/ 目录下所有 *_invariant.go + _invariant.go 文件,
// 验证:
//   1. 每个 *_invariant.go 文件存在 (在 surface / tracker / freefork / verify 包)
//   2. 每个 invariant tag 格式合法 (含 "=>")
//   3. 每个 invariant 集合至少 1 条 (否则是 placeholder, 警告)
//
// 跨 surface 冲突检测: 同名 invariant (FieldName) 在不同 surface 同时声明时,
// 若 Post 不同则报告冲突 (e.g. lsp.InvX.post != tracker.InvX.post)。
//
// 用法:
//
//	go run ./tools/ci-lint-invariant \
//	    -roots ./internal/layers/contextengine/enforce/toolrunner/surface,./internal/layers/multiagent/provision/freefork,./internal/layers/observability/diagnose/tracker,./internal/layers/evolution/verify \
//	    -fail-on-warn
//
// 退出码: 0 全部通过; 1 有 error; 2 有 warn (且 -fail-on-warn)。
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// 默认扫描 root (与 W15 tasks.md 列表对齐)。
var defaultRoots = []string{
	"./internal/layers/contextengine/enforce/toolrunner/surface",
	"./internal/layers/multiagent/provision/freefork",
	"./internal/layers/observability/diagnose/tracker",
	"./internal/layers/evolution/verify",
}

// invariantTag = "invariant:\"...\""。
const invariantTag = "invariant"

func main() {
	rootsFlag := flag.String("roots", strings.Join(defaultRoots, ","), "comma-separated root dirs to scan")
	failOnWarn := flag.Bool("fail-on-warn", false, "exit 2 if any warning emitted")
	verbose := flag.Bool("v", false, "verbose output")
	flag.Parse()

	roots := strings.Split(*rootsFlag, ",")
	report, err := scan(roots, *verbose)
	if err != nil {
		log.Fatalf("ci-lint-invariant: scan error: %v", err)
	}
	report.Print(os.Stdout)
	if report.ErrorCount() > 0 {
		os.Exit(1)
	}
	if *failOnWarn && report.WarningCount() > 0 {
		os.Exit(2)
	}
}

// Report 一次 lint 的总结。
type Report struct {
	FilesScanned   int
	Invariants     []InvariantEntry
	Errors         []string
	Warnings       []string
	ConflictGroups []ConflictGroup
}

// InvariantEntry 一条 invariant 记录。
type InvariantEntry struct {
	File   string
	Source string // 类型名
	Field  string
	Pre    string
	Post   string
}

// ConflictGroup 同名 invariant 在不同 surface 的冲突报告。
type ConflictGroup struct {
	Field string
	Posts []string
}

func (r *Report) ErrorCount() int   { return len(r.Errors) }
func (r *Report) WarningCount() int { return len(r.Warnings) }

func (r *Report) Print(w *os.File) {
	fmt.Fprintf(w, "ci-lint-invariant: scanned %d files, found %d invariants\n", r.FilesScanned, len(r.Invariants))
	fmt.Fprintf(w, "  errors=%d warnings=%d conflicts=%d\n", r.ErrorCount(), r.WarningCount(), len(r.ConflictGroups))
	for _, e := range r.Errors {
		fmt.Fprintf(w, "  ERROR: %s\n", e)
	}
	for _, wn := range r.Warnings {
		fmt.Fprintf(w, "  WARN:  %s\n", wn)
	}
	for _, cg := range r.ConflictGroups {
		fmt.Fprintf(w, "  CONFLICT: invariant %q has divergent posts %v\n", cg.Field, cg.Posts)
	}
}

func scan(roots []string, verbose bool) (*Report, error) {
	rep := &Report{}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		files, err := findInvariantFiles(root)
		if err != nil {
			return nil, err
		}
		rep.FilesScanned += len(files)
		if len(files) == 0 {
			rep.Warnings = append(rep.Warnings, "no _invariant.go files found under "+root)
			continue
		}
		for _, f := range files {
			entries, errs := parseInvariantFile(f)
			rep.Errors = append(rep.Errors, errs...)
			rep.Invariants = append(rep.Invariants, entries...)
			if len(entries) == 0 {
				rep.Warnings = append(rep.Warnings, f+" declares no invariants (empty or placeholder)")
			}
			if verbose {
				log.Printf("  scanned %s: %d invariants", f, len(entries))
			}
		}
	}
	// 跨 surface 冲突检测
	rep.ConflictGroups = detectConflicts(rep.Invariants)
	for _, cg := range rep.ConflictGroups {
		if len(cg.Posts) > 1 {
			// 只标记真的有 divergent post 的冲突
			rep.Warnings = append(rep.Warnings,
				fmt.Sprintf("invariant %q has divergent posts %v across surfaces", cg.Field, cg.Posts))
		}
	}
	return rep, nil
}

// findInvariantFiles 递归找 _invariant.go / *_invariant.go 文件。
func findInvariantFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		if name == "_invariant.go" || strings.HasSuffix(name, "_invariant.go") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// parseInvariantFile 用 go/parser 抽所有 struct field 的 invariant tag。
func parseInvariantFile(path string) ([]InvariantEntry, []string) {
	var out []InvariantEntry
	var errs []string
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, []string{path + ": " + err.Error()}
	}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			sourceName := ts.Name.Name
			for _, field := range st.Fields.List {
				if field.Tag == nil {
					continue
				}
				tagVal := strings.Trim(field.Tag.Value, "`")
				tag := reflectTag(tagVal, invariantTag)
				if tag == "" {
					continue
				}
				if !strings.Contains(tag, "=>") {
					errs = append(errs, fmt.Sprintf("%s: %s.%s missing '=>' operator (got %q)",
						path, sourceName, field.Names[0].Name, tag))
					continue
				}
				parts := strings.SplitN(tag, "=>", 2)
				pre := strings.TrimSpace(parts[0])
				post := strings.TrimSpace(parts[1])
				if pre == "" || post == "" {
					errs = append(errs, fmt.Sprintf("%s: %s.%s has empty pre or post",
						path, sourceName, field.Names[0].Name))
					continue
				}
				out = append(out, InvariantEntry{
					File:   path,
					Source: sourceName,
					Field:  field.Names[0].Name,
					Pre:    pre,
					Post:   post,
				})
			}
		}
	}
	return out, errs
}

// reflectTag 简单 key:"value" tag 解析 (避免 reflect.StructTag 依赖)。
func reflectTag(tag, key string) string {
	for tag != "" {
		// skip leading space
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}
		// key
		i = 0
		for i < len(tag) && tag[i] != ':' && tag[i] > ' ' {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		k := tag[:i]
		tag = tag[i+1:]
		// quoted value
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		quoteVal := tag[1:i]
		tag = tag[i+1:]
		if k == key {
			return quoteVal
		}
	}
	return ""
}

// detectConflicts 跨 surface 同名 invariant 的 post 冲突检测。
func detectConflicts(entries []InvariantEntry) []ConflictGroup {
	byField := map[string][]InvariantEntry{}
	for _, e := range entries {
		byField[e.Field] = append(byField[e.Field], e)
	}
	var groups []ConflictGroup
	for field, es := range byField {
		if len(es) < 2 {
			continue
		}
		posts := map[string]struct{}{}
		for _, e := range es {
			posts[e.Post] = struct{}{}
		}
		if len(posts) > 1 {
			pList := make([]string, 0, len(posts))
			for p := range posts {
				pList = append(pList, p)
			}
			groups = append(groups, ConflictGroup{Field: field, Posts: pList})
		}
	}
	return groups
}
