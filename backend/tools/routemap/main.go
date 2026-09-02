// Command routemap reads the server's own source and prints a map of every
// HTTP route: which mux it is mounted on, therefore what authorization it
// sits behind, which handler serves it, and which store methods that handler
// reaches.
//
// It reads the AST rather than matching text, so it cannot drift from the
// code the way a hand-drawn diagram does. Re-run it after touching main.go
// or a handler and the diagram is correct again:
//
//	go run ./tools/routemap -o docs/ROUTEMAP.md
//
// The output is Markdown with Mermaid diagrams, which GitHub renders natively
// and VS Code renders with the Markdown Preview Mermaid Support extension.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// middleware names are wrappers rather than the handler being mounted, so
// they are stepped over when working out what actually serves a route.
var middleware = map[string]bool{
	"RateLimit":       true,
	"RequireAuth":     true,
	"RequireAdmin":    true,
	"CORS":            true,
	"SecurityHeaders": true,
}

// tiers maps a mux variable in main.go to the authorization a route mounted
// on it sits behind. The nesting is built by hand here because it is three
// lines in main.go and inferring it would be more code than it is worth.
var tiers = map[string]string{
	"mux":    "public",
	"apiMux": "public",
	"authed": "bearer",
	"admin":  "owner",
}

type route struct {
	Method  string
	Path    string
	Mux     string
	Tier    string
	Handler string
	Limit   string
}

type handlerInfo struct {
	// StoreCalls are methods reached on *store.Store, directly or through a
	// package-local helper.
	StoreCalls []string
	// localCalls are other funcs in the same package, used to close over
	// helpers like ownedRoom and meFor before reporting.
	localCalls []string
}

func main() {
	root := flag.String("root", ".", "module root containing cmd/ and internal/")
	out := flag.String("o", "", "write to this file instead of stdout")
	flag.Parse()

	routes, err := parseRoutes(filepath.Join(*root, "cmd", "server", "main.go"))
	if err != nil {
		fatal(err)
	}
	handlers, err := parseHandlers(filepath.Join(*root, "internal", "api"))
	if err != nil {
		fatal(err)
	}
	resolve(handlers)

	doc := render(routes, handlers)

	if *out == "" {
		fmt.Print(doc)
		return
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*out, []byte(doc), 0o644); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "routemap: wrote %s (%d routes)\n", *out, len(routes))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "routemap:", err)
	os.Exit(1)
}

// parseRoutes pulls every Handle and HandleFunc call out of main.go.
//
// A pattern carrying a method ("POST /api/rooms") is a route. One without
// ("/api/rooms") is a mount point that nests one mux inside another, and is
// skipped: it serves nothing itself.
func parseRoutes(path string) ([]route, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var routes []route
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc") {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		tier, ok := tiers[recv.Name]
		if !ok {
			return true
		}
		pattern, ok := stringLit(call.Args[0])
		if !ok {
			return true
		}

		method, p := splitPattern(pattern)
		if method == "" {
			// A mount, not a route.
			return true
		}

		routes = append(routes, route{
			Method:  method,
			Path:    p,
			Mux:     recv.Name,
			Tier:    tier,
			Handler: handlerName(call.Args[1]),
			Limit:   rateLimit(call.Args[1]),
		})
		return true
	})

	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Tier != routes[j].Tier {
			return tierRank(routes[i].Tier) < tierRank(routes[j].Tier)
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}

func tierRank(t string) int {
	switch t {
	case "public":
		return 0
	case "bearer":
		return 1
	case "owner":
		return 2
	}
	return 3
}

// splitPattern separates Go 1.22 mux patterns into method and path. A pattern
// with no method returns an empty method.
func splitPattern(pattern string) (method, path string) {
	parts := strings.SplitN(pattern, " ", 2)
	if len(parts) == 2 && strings.ToUpper(parts[0]) == parts[0] && parts[0] != "" {
		return parts[0], strings.TrimSpace(parts[1])
	}
	return "", pattern
}

// handlerName digs past middleware wrappers to the thing actually serving the
// route: api.Register inside api.RateLimit(api.Register(db), ...).
func handlerName(expr ast.Expr) string {
	// A literal written at the mount point has no name to report, and
	// descending into it would name whatever it happens to call first.
	if _, ok := expr.(*ast.FuncLit); ok {
		return "inline func"
	}

	var found string
	ast.Inspect(expr, func(n ast.Node) bool {
		if found != "" {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if middleware[sel.Sel.Name] {
			// Keep descending; the real handler is an argument to this.
			return true
		}
		found = pkg.Name + "." + sel.Sel.Name
		return false
	})
	if found == "" {
		return "inline func"
	}
	return found
}

// rateLimit reports the budget on a route wrapped in api.RateLimit, as it is
// written in the source — "5 / 10*time.Minute" — so the diagram does not have
// to agree with main.go about what a duration means.
func rateLimit(expr ast.Expr) string {
	var limit string
	ast.Inspect(expr, func(n ast.Node) bool {
		if limit != "" {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 3 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "RateLimit" {
			return true
		}
		limit = exprText(call.Args[1]) + " / " + exprText(call.Args[2])
		return false
	})
	return limit
}

// exprText renders the small expressions that appear as rate-limit arguments.
// Anything more involved than a literal or a binary expression is reported as
// "?" rather than guessed at.
func exprText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		return v.Value
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
	case *ast.BinaryExpr:
		return exprText(v.X) + v.Op.String() + exprText(v.Y)
	}
	return "?"
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// parseHandlers records, for every top-level func in internal/api that takes a
// *store.Store, which methods it calls on it and which package-local funcs it
// delegates to.
func parseHandlers(dir string) (map[string]*handlerInfo, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, err
	}

	out := make(map[string]*handlerInfo)
	for _, pkg := range pkgs {
		// Two passes: the set of local func names has to be known before a
		// body can be told "calls a helper" from "calls the standard library".
		local := make(map[string]bool)
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
					local[fn.Name.Name] = true
				}
			}
		}

		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || fn.Body == nil {
					continue
				}
				storeParam := storeParamName(fn)
				info := &handlerInfo{}
				collect(fn.Body, storeParam, local, info)
				if len(info.StoreCalls) > 0 || len(info.localCalls) > 0 {
					out[fn.Name.Name] = info
				}
			}
		}
	}
	return out, nil
}

// storeParamName returns the parameter holding a *store.Store, or "" when the
// func does not take one.
func storeParamName(fn *ast.FuncDecl) string {
	if fn.Type.Params == nil {
		return ""
	}
	for _, field := range fn.Type.Params.List {
		star, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "store" || sel.Sel.Name != "Store" {
			continue
		}
		if len(field.Names) > 0 {
			return field.Names[0].Name
		}
	}
	return ""
}

func collect(body *ast.BlockStmt, storeParam string, local map[string]bool, info *handlerInfo) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if x, ok := fun.X.(*ast.Ident); ok && storeParam != "" && x.Name == storeParam {
				info.StoreCalls = appendUnique(info.StoreCalls, fun.Sel.Name)
			}
		case *ast.Ident:
			if local[fun.Name] {
				info.localCalls = appendUnique(info.localCalls, fun.Name)
			}
		}
		return true
	})
}

// resolve folds each helper's store calls into the handlers that call it, so
// CreateRoom is reported as reaching RoomByID through ownedRoom. Iterated to a
// fixpoint because helpers call helpers.
func resolve(handlers map[string]*handlerInfo) {
	for pass := 0; pass < 8; pass++ {
		changed := false
		for _, info := range handlers {
			for _, callee := range info.localCalls {
				target, ok := handlers[callee]
				if !ok {
					continue
				}
				for _, m := range target.StoreCalls {
					before := len(info.StoreCalls)
					info.StoreCalls = appendUnique(info.StoreCalls, m)
					if len(info.StoreCalls) != before {
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}
	for _, info := range handlers {
		sort.Strings(info.StoreCalls)
	}
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

func render(routes []route, handlers map[string]*handlerInfo) string {
	var b strings.Builder

	b.WriteString("# Route map\n\n")
	b.WriteString("Generated by `go run ./tools/routemap`. Do not edit by hand — ")
	b.WriteString("re-run it instead, so the diagram cannot drift from `main.go`.\n\n")

	// ---- table ----
	b.WriteString("## Every route\n\n")
	b.WriteString("| Route | Auth | Rate limit | Handler | Store methods |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, r := range routes {
		limit := r.Limit
		if limit == "" {
			limit = "—"
		}
		calls := "—"
		if info, ok := handlers[shortName(r.Handler)]; ok && len(info.StoreCalls) > 0 {
			calls = "`" + strings.Join(info.StoreCalls, "`, `") + "`"
		}
		fmt.Fprintf(&b, "| `%s %s` | %s | %s | `%s` | %s |\n",
			r.Method, r.Path, r.Tier, limit, r.Handler, calls)
	}

	// ---- route diagram ----
	b.WriteString("\n## Routes by authorization tier\n\n")
	b.WriteString("```mermaid\nflowchart LR\n")
	for _, tier := range []string{"public", "bearer", "owner"} {
		var inTier []route
		for _, r := range routes {
			if r.Tier == tier {
				inTier = append(inTier, r)
			}
		}
		if len(inTier) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  subgraph %s[\"%s\"]\n", tier, tierLabel(tier))
		fmt.Fprintf(&b, "    direction LR\n")
		for _, r := range inTier {
			id := nodeID("r", r.Method+r.Path)
			fmt.Fprintf(&b, "    %s[\"%s %s\"]\n", id, r.Method, escape(r.Path))
		}
		b.WriteString("  end\n")
	}
	seen := map[string]bool{}
	for _, r := range routes {
		hid := nodeID("h", r.Handler)
		if !seen[hid] {
			fmt.Fprintf(&b, "  %s(\"%s\")\n", hid, r.Handler)
			seen[hid] = true
		}
		fmt.Fprintf(&b, "  %s --> %s\n", nodeID("r", r.Method+r.Path), hid)
	}
	b.WriteString("```\n")

	// ---- store diagram ----
	b.WriteString("\n## Handlers to store methods\n\n")
	b.WriteString("Which part of the database each handler can reach. ")
	b.WriteString("A handler touching more of the store than you expected is worth a second look.\n\n")
	b.WriteString("```mermaid\nflowchart LR\n")
	drawn := map[string]bool{}
	for _, r := range routes {
		info, ok := handlers[shortName(r.Handler)]
		if !ok || len(info.StoreCalls) == 0 {
			continue
		}
		hid := nodeID("h", r.Handler)
		if !drawn[hid] {
			fmt.Fprintf(&b, "  %s(\"%s\")\n", hid, r.Handler)
			drawn[hid] = true
		}
		for _, m := range info.StoreCalls {
			sid := nodeID("s", m)
			if !drawn[sid] {
				fmt.Fprintf(&b, "  %s[(\"%s\")]\n", sid, m)
				drawn[sid] = true
			}
			fmt.Fprintf(&b, "  %s --> %s\n", hid, sid)
		}
	}
	b.WriteString("```\n")

	return b.String()
}

func tierLabel(tier string) string {
	switch tier {
	case "public":
		return "Public - no token"
	case "bearer":
		return "RequireAuth - session token"
	case "owner":
		return "RequireAdmin - owner only"
	}
	return tier
}

// shortName drops the package qualifier, since handlerInfo is keyed by the
// func name as declared.
func shortName(handler string) string {
	if i := strings.LastIndex(handler, "."); i >= 0 {
		return handler[i+1:]
	}
	return handler
}

// nodeID turns a route or handler into a Mermaid-safe identifier.
func nodeID(prefix, s string) string {
	var b strings.Builder
	b.WriteString(prefix)
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// escape keeps braces in path parameters from being read as Mermaid syntax.
func escape(s string) string {
	s = strings.ReplaceAll(s, "{", "#123;")
	return strings.ReplaceAll(s, "}", "#125;")
}
