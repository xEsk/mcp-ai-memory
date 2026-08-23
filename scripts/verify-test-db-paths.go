package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	root := "."
	if len(os.Args) == 2 {
		root = os.Args[1]
	} else if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/verify-test-db-paths.go [repository-root]")
		os.Exit(2)
	}
	unsafePaths, err := findUnsafeTestDatabasePaths(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify test database paths: %v\n", err)
		os.Exit(1)
	}
	if len(unsafePaths) > 0 {
		for _, unsafePath := range unsafePaths {
			fmt.Fprintln(os.Stderr, unsafePath)
		}
		os.Exit(1)
	}
	fmt.Println("test database paths are scoped to t.TempDir()")
}

func findUnsafeTestDatabasePaths(root string) ([]string, error) {
	var unsafePaths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "dist") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		parents := map[ast.Node]ast.Node{}
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				return true
			}
			ast.Inspect(node, func(child ast.Node) bool {
				if child != nil && child != node {
					if _, exists := parents[child]; !exists {
						parents[child] = node
					}
					return false
				}
				return true
			})
			return true
		})
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				if containsDatabasePath(declaration) {
					position := fileSet.Position(declaration.Pos())
					unsafePaths = append(unsafePaths, fmt.Sprintf("%s:%d: package-level declaration references memory.db", path, position.Line))
				}
				continue
			}
			if function.Body == nil {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				for _, expression := range databasePathArguments(call) {
					tempVariables, stringConstants, tempWorkingDirectory := contextBefore(function.Body, expression.Pos())
					if isTempDerived(expression, tempVariables) || (tempWorkingDirectory && isRelativeDatabasePath(expression, stringConstants)) {
						continue
					}
					position := fileSet.Position(expression.Pos())
					unsafePaths = append(unsafePaths, fmt.Sprintf("%s:%d: %s passes a non-temporary path to a database operation", path, position.Line, function.Name.Name))
				}
				return true
			})
			seenExpressions := map[ast.Expr]bool{}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				candidate, ok := node.(ast.Expr)
				if !ok {
					return true
				}
				tempVariables, stringConstants, tempWorkingDirectory := contextBefore(function.Body, candidate.Pos())
				if !isDatabasePathExpression(candidate, stringConstants) {
					return true
				}
				expression := candidate
				for parent := parents[expression]; parent != nil; parent = parents[parent] {
					parentExpression, ok := parent.(ast.Expr)
					if !ok {
						break
					}
					expression = parentExpression
				}
				if seenExpressions[expression] {
					return true
				}
				seenExpressions[expression] = true
				if value, valueOK := constantString(expression, stringConstants); valueOK && filepath.Base(value) == value {
					return true
				}
				if !isTempDerived(expression, tempVariables) && !(tempWorkingDirectory && isRelativeDatabasePath(expression, stringConstants)) {
					position := fileSet.Position(candidate.Pos())
					unsafePaths = append(unsafePaths, fmt.Sprintf("%s:%d: %s uses memory.db without a t.TempDir()-derived path", path, position.Line, function.Name.Name))
				}
				return true
			})
		}
		return nil
	})
	return unsafePaths, err
}

func databasePathArguments(call *ast.CallExpr) []ast.Expr {
	name := ""
	switch function := call.Fun.(type) {
	case *ast.Ident:
		name = function.Name
	case *ast.SelectorExpr:
		name = function.Sel.Name
	}
	switch name {
	case "OpenSQLite", "BackupDatabase", "RebuildDatabaseFTS", "InspectSchema":
		if len(call.Args) > 1 {
			return []ast.Expr{call.Args[1]}
		}
	case "CreateBackup":
		if len(call.Args) > 2 {
			return []ast.Expr{call.Args[2]}
		}
	case "RestoreBackup":
		if len(call.Args) > 2 {
			return []ast.Expr{call.Args[1], call.Args[2]}
		}
	}
	return nil
}

func contextBefore(body *ast.BlockStmt, before token.Pos) (map[string]bool, map[string]string, bool) {
	tempVariables := map[string]bool{}
	stringConstants := map[string]string{}
	tempWorkingDirectory := false
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil || node.Pos() >= before {
			return false
		}
		switch statement := node.(type) {
		case *ast.AssignStmt:
			for index, expression := range statement.Rhs {
				if index < len(statement.Lhs) {
					if identifier, ok := statement.Lhs[index].(*ast.Ident); ok {
						tempVariables[identifier.Name] = isTempDerived(expression, tempVariables)
						if value, ok := constantString(expression, stringConstants); ok {
							stringConstants[identifier.Name] = value
						} else {
							delete(stringConstants, identifier.Name)
						}
					}
				}
			}
			return false
		case *ast.ValueSpec:
			for index, expression := range statement.Values {
				if index < len(statement.Names) {
					tempVariables[statement.Names[index].Name] = isTempDerived(expression, tempVariables)
					if value, ok := constantString(expression, stringConstants); ok {
						stringConstants[statement.Names[index].Name] = value
					} else {
						delete(stringConstants, statement.Names[index].Name)
					}
				}
			}
			return false
		case *ast.CallExpr:
			if len(statement.Args) != 1 {
				return true
			}
			selector, ok := statement.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier, identifierOK := selector.X.(*ast.Ident)
			if identifierOK && identifier.Name == "t" && selector.Sel.Name == "Chdir" && isTempDerived(statement.Args[0], tempVariables) {
				tempWorkingDirectory = true
			}
		}
		return true
	})
	return tempVariables, stringConstants, tempWorkingDirectory
}

func isTempDerived(expression ast.Expr, tempVariables map[string]bool) bool {
	derived := false
	ast.Inspect(expression, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			if tempVariables[value.Name] {
				derived = true
			}
		case *ast.CallExpr:
			selector, ok := value.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier, identifierOK := selector.X.(*ast.Ident)
			if identifierOK && identifier.Name == "t" && selector.Sel.Name == "TempDir" {
				derived = true
			}
		}
		return !derived
	})
	return derived
}

func containsDatabasePath(node ast.Node) bool {
	contains := false
	ast.Inspect(node, func(node ast.Node) bool {
		if expression, ok := node.(ast.Expr); ok && isDatabasePathExpression(expression, nil) {
			contains = true
		}
		return !contains
	})
	return contains
}

func isDatabasePathExpression(expression ast.Expr, constants map[string]string) bool {
	value, ok := constantString(expression, constants)
	return ok && strings.Contains(value, "memory.db")
}

func constantString(expression ast.Expr, constants map[string]string) (string, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		literal, err := strconv.Unquote(value.Value)
		return literal, err == nil
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := constantString(value.X, constants)
		right, rightOK := constantString(value.Y, constants)
		return left + right, leftOK && rightOK
	case *ast.ParenExpr:
		return constantString(value.X, constants)
	case *ast.Ident:
		literal, ok := constants[value.Name]
		return literal, ok
	default:
		return "", false
	}
}

func isRelativeDatabasePath(expression ast.Expr, constants map[string]string) bool {
	if value, ok := constantString(expression, constants); ok {
		return !filepath.IsAbs(value)
	}
	relative := false
	ast.Inspect(expression, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || !isDatabasePathExpressionInTree(call, constants) {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		identifier, identifierOK := selector.X.(*ast.Ident)
		if identifierOK && identifier.Name == "filepath" && selector.Sel.Name == "Join" {
			if first, firstOK := constantString(call.Args[0], constants); firstOK && !filepath.IsAbs(first) {
				relative = true
			}
		}
		return !relative
	})
	return relative
}

func isDatabasePathExpressionInTree(node ast.Node, constants map[string]string) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		if expression, ok := node.(ast.Expr); ok && isDatabasePathExpression(expression, constants) {
			found = true
		}
		return !found
	})
	return found
}
