package formatters

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"slices"
	"strings"

	"github.com/samber/lo"
)

const (
	CONST      token.Token = token.CONST
	FUNC       token.Token = token.FUNC
	IMPORT     token.Token = token.IMPORT
	METHOD     token.Token = token.Token(math.MaxInt) // go/token doesn't have a METHOD token
	TYPE       token.Token = token.TYPE
	VAR        token.Token = token.VAR
	mainMethod string      = "main"
)

var (
	declOrder = map[token.Token]int{
		IMPORT: 0,
		CONST:  1,
		VAR:    2,
		TYPE:   3,
		FUNC:   4,
	}
	newline = []byte("\n")
)

type aifiFormatter struct{}

// AifiFormatter is a code formatter that sorts Go declarations in the following order:
// 1. Imports
// 2. Constants
// 3. Variables
// 4. Types
// 5. Functions
//
// Within each category, declarations are sorted alphabetically, treating whole numbers in names as numeric values.
// The "main" function always comes first among functions.
// Methods are sorted immediately after the type they belong to, and alphabetically by method name if multiple methods belong to the same type.
// Comments associated with declarations are preserved and moved along with their respective declarations.
func (aifiFormatter) Format(filename string, src []byte) ([]byte, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	} else if len(file.Decls) == 0 {
		return bytes.Clone(src), nil
	}

	decls := getDecls(file, src)
	firstDeclStart := decls[0].Pos()
	lastDeclEnd := decls[len(decls)-1].End()

	slices.SortFunc(decls, func(a, b *declaration) int {
		if a.Tok == METHOD {
			return a.compareMethodToDecl(b)
		} else if b.Tok == METHOD {
			return -b.compareMethodToDecl(a)
		} else if a.Tok != b.Tok {
			return cmp.Compare(declOrder[a.Tok], declOrder[b.Tok])
		}

		switch a.Tok {
		case IMPORT, CONST, VAR:
			return cmp.Compare(a.OriginalOrder, b.OriginalOrder) // stable sort
		case TYPE:
			return compareStringsWithWholeNumbers(a.getTypeName(), b.getTypeName())
		case FUNC:
			return compareFuncNames(a.getFunctionName(), b.getFunctionName())
		}
		panic(fmt.Errorf("unsupported token.Token: %v", a.Tok))
	})

	rewritten := bytes.Join(lo.Map(decls, func(decl *declaration, _ int) []byte { return decl.Text }), newline)
	return append(append(src[0:firstDeclStart-1], rewritten...), src[lastDeclEnd:]...), nil
}

// A Go declaration, either a function/method or a general declaration (import, const, type, var).
type declaration struct {
	ast.Node
	Body          *ast.BlockStmt    // function body; or nil for external (non-Go) function
	Doc           *ast.CommentGroup // associated documentation; or nil
	Lparen        token.Pos         // position of '(', if any
	Name          *ast.Ident        // function/method name
	OriginalOrder int               // original order in source file, for stable sorting of imports, consts, and vars
	Recv          *ast.FieldList    // receiver (methods); or nil (functions)
	Rparen        token.Pos         // position of ')', if any
	Specs         []ast.Spec        // *ImportSpec, *TypeSpec, or *ValueSpec, if any
	Text          []byte            // original text of declaration
	Tok           token.Token       //
	TokPos        token.Pos         // position of Tok
	Type          *ast.FuncType     // function signature: type and value parameters, results, and position of "func" keyword
}

// Since methods are tied to types, we want to sort them immediately after the type declaration they belong to.
// If multiple methods belong to the same type, sort them alphabetically by method name.
func (decl *declaration) compareMethodToDecl(other *declaration) int {
	receiverName := decl.getReceiverTypeName()
	switch other.Tok {
	case IMPORT, CONST, VAR:
		return 1
	case TYPE:
		typeName := other.getTypeName()
		if typeName == receiverName {
			return 1 // method goes after the type declaration
		}
		return compareStringsWithWholeNumbers(receiverName, typeName)
	case METHOD:
		if c := compareStringsWithWholeNumbers(receiverName, other.getReceiverTypeName()); c == 0 {
			return compareStringsWithWholeNumbers(decl.getFunctionName(), other.getFunctionName())
		} else {
			return c
		}
	case FUNC:
		return -1
	default:
		panic(fmt.Errorf("unsupported token.Token: %v", other.Tok))
	}
}

func (decl *declaration) getFunctionName() string {
	if decl.Tok != FUNC && decl.Tok != METHOD {
		return ""
	}
	return decl.Name.Name
}

// If the declaration is a method, return the name of the receiver type (without any pointer or generic syntax).
func (decl *declaration) getReceiverTypeName() string {
	if decl.Tok != METHOD {
		return ""
	}

	var rec func(expr ast.Expr) string
	rec = func(expr ast.Expr) string {
		switch expr := expr.(type) {
		case *ast.Ident:
			return expr.Name
		case *ast.IndexExpr:
			return rec(expr.X)
		case *ast.IndexListExpr:
			return rec(expr.X)
		case *ast.StarExpr:
			return rec(expr.X)
		default:
			panic(fmt.Errorf("unsupported receiver type: %T", expr))
		}
	}
	return rec(decl.Recv.List[0].Type)
}

// If the declaration is a type declaration, return the name of the type.
func (decl *declaration) getTypeName() string {
	if decl.Tok != TYPE {
		return ""
	}
	return decl.Specs[0].(*ast.TypeSpec).Name.Name
}

// A node that spans from the start of one node to the end of another.
type rangeNode struct {
	end   ast.Node
	start ast.Node
}

func (rn *rangeNode) End() token.Pos {
	return rn.end.End()
}

func (rn *rangeNode) Pos() token.Pos {
	return rn.start.Pos()
}

// Compare two function names, treating whole numbers in the names as numeric values.
// The "main" function always comes first.
func compareFuncNames(a, b string) int {
	if a == mainMethod {
		return -1
	} else if b == mainMethod {
		return 1
	}
	return compareStringsWithWholeNumbers(a, b)
}

// Compare two strings, treating whole numbers in the strings as numeric values.
// For example, "item2" < "item10" because 2 < 10.
func compareStringsWithWholeNumbers(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] != b[j] {
			return cmp.Compare(a[i], b[j])
		} else if !isDigit(a[i]) {
			i++
			j++
			continue
		}

		var aNum, bNum string
		aNum, i = parseNumber(a, i)
		bNum, j = parseNumber(b, j)
		if aNum != bNum {
			if aLen, bLen := len(aNum), len(bNum); aLen != bLen {
				return cmp.Compare(aLen, bLen)
			}
			return cmp.Compare(aNum, bNum)
		}
	}

	if i < len(a) {
		return 1
	} else if j < len(b) {
		return -1
	}
	return 0
}

// Convert an ast.Decl to a *declaration, capturing the original source text.
func getDecl(src []byte, decl ast.Decl, node ast.Node, order int) *declaration {
	switch decl := decl.(type) {
	case *ast.FuncDecl:
		return &declaration{
			Node:          node,
			Body:          decl.Body,
			Tok:           lo.Ternary(decl.Recv == nil, FUNC, METHOD),
			Doc:           decl.Doc,
			Name:          decl.Name,
			OriginalOrder: order,
			Recv:          decl.Recv,
			Text:          src[node.Pos()-1 : node.End()],
			Type:          decl.Type,
		}
	case *ast.GenDecl:
		return &declaration{
			Node:          node,
			Doc:           decl.Doc,
			Lparen:        decl.Lparen,
			OriginalOrder: order,
			Rparen:        decl.Rparen,
			Specs:         decl.Specs,
			Text:          src[node.Pos()-1 : node.End()],
			Tok:           decl.Tok,
			TokPos:        decl.TokPos,
		}
	default:
		panic(fmt.Errorf("unsupported ast.Decl: %T", decl))
	}
}

// Since we're going to be moving around declarations, we need to do something with the comments.
// Set the start of the Nth Decl to the start of the first comment block that comes after the end of the (N-1)th Decl.
// This means multiple comment blocks between two Decls will all be prepended, in order, to the second Decl.
// Comment blocks before the first Decl and after the last Decl are ignored.
func getDecls(file *ast.File, src []byte) []*declaration {
	leftBound := newlinePosAfterPackageDecl(file, src)
	if leftBound == token.NoPos {
		leftBound = 1 // start of file
	}

	res := make([]*declaration, len(file.Decls))
	for i, j := 0, 0; i < len(file.Decls); i++ {
		for j < len(file.Comments) && file.Comments[j].Pos() < leftBound {
			j++ // skip all comments before the end of the last block
		}

		node := rangeNode{start: file.Decls[i], end: file.Decls[i]}
		if j < len(file.Comments) && file.Comments[j].Pos() < file.Decls[i].Pos() {
			node.start = file.Comments[j] // attach all comment blocks before this declaration to it
		}

		res[i] = getDecl(src, file.Decls[i], &node, i)
		leftBound = file.Decls[i].End()
	}
	return res
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// Find the position of the first newline character after the package declaration.
// Returns token.NoPos if the package declaration is not found or if there is no newline after it.
func newlinePosAfterPackageDecl(file *ast.File, src []byte) token.Pos {
	if file.Name == nil {
		return token.NoPos
	}

	// Pos is 1-based, so subtract 1 to get a 0-based index into src.
	// Then search for the next newline after the package declaration.
	// The declaration starts at file.Package, which is the position of the 'p' in "package".
	// We want to find the newline after the entire package declaration line.
	// If there are comments on the same line, we still want to include them.
	// So we look for the first newline character after the package declaration.
	indexOfNewlineAfterPackage := bytes.Index(src[file.Package-1:], newline)
	if indexOfNewlineAfterPackage == -1 {
		return token.NoPos
	}
	return token.Pos(indexOfNewlineAfterPackage + int(file.Package)) // +1 to convert back to 1-based position
}

// Parse a whole number starting at index i in string s.
// Returns the number as a string (with leading zeros removed) and the index of the first character after the number.
func parseNumber(s string, i int) (string, int) {
	j := i + 1
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	return strings.TrimLeft(s[i:j], "0"), j
}
