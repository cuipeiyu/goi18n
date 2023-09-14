package main

import (
	"go/ast"
	"go/token"
	"strconv"

	i18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type extractor struct {
	i18nPackageName string
	messages        []*i18n.Message
}

func (e *extractor) Visit(node ast.Node) ast.Visitor {
	e.extractMessages(node)
	return e
}

func (e *extractor) extractMessages(node ast.Node) {
	cl, ok := node.(*ast.CompositeLit)
	if !ok {
		return
	}
	switch t := cl.Type.(type) {
	case *ast.SelectorExpr:
		if !e.isMessageType(t) {
			return
		}
		e.extractMessage(cl)
	case *ast.ArrayType:
		if !e.isMessageType(t.Elt) {
			return
		}
		for _, el := range cl.Elts {
			ecl, ok := el.(*ast.CompositeLit)
			if !ok {
				continue
			}
			e.extractMessage(ecl)
		}
	case *ast.MapType:
		if !e.isMessageType(t.Value) {
			return
		}
		for _, el := range cl.Elts {
			kve, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			vcl, ok := kve.Value.(*ast.CompositeLit)
			if !ok {
				continue
			}
			e.extractMessage(vcl)
		}
	}
}

func (e *extractor) isMessageType(expr ast.Expr) bool {
	se := unwrapSelectorExpr(expr)
	if se == nil {
		return false
	}
	if se.Sel.Name != "Message" && se.Sel.Name != "LocalizeConfig" {
		return false
	}
	x, ok := se.X.(*ast.Ident)
	if !ok {
		return false
	}
	return x.Name == e.i18nPackageName
}

func unwrapSelectorExpr(e ast.Expr) *ast.SelectorExpr {
	switch et := e.(type) {
	case *ast.SelectorExpr:
		return et
	case *ast.StarExpr:
		se, _ := et.X.(*ast.SelectorExpr)
		return se
	default:
		return nil
	}
}

func (e *extractor) extractMessage(cl *ast.CompositeLit) {
	data := make(map[string]string)
	for _, elt := range cl.Elts {
		kve, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kve.Key.(*ast.Ident)
		if !ok {
			continue
		}
		v, ok := extractStringLiteral(kve.Value)
		if !ok {
			continue
		}
		data[key.Name] = v
	}
	if len(data) == 0 {
		return
	}
	if messageID := data["MessageID"]; messageID != "" {
		data["ID"] = messageID
	}
	e.messages = append(e.messages, i18n.MustNewMessage(data))
}

func extractStringLiteral(expr ast.Expr) (string, bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return "", false
		}
		return s, true
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		x, ok := extractStringLiteral(v.X)
		if !ok {
			return "", false
		}
		y, ok := extractStringLiteral(v.Y)
		if !ok {
			return "", false
		}
		return x + y, true
	case *ast.Ident:
		if v.Obj == nil {
			return "", false
		}
		switch z := v.Obj.Decl.(type) {
		case *ast.ValueSpec:
			if len(z.Values) == 0 {
				return "", false
			}
			s, ok := extractStringLiteral(z.Values[0])
			if !ok {
				return "", false
			}
			return s, true
		}
		return "", false
	default:
		return "", false
	}
}

func i18nPackageName(file *ast.File) string {
	for _, i := range file.Imports {
		if i.Path.Kind == token.STRING && i.Path.Value == `"github.com/nicksnyder/go-i18n/v2/i18n"` {
			if i.Name == nil {
				return "i18n"
			}
			return i.Name.Name
		}
	}
	return ""
}
