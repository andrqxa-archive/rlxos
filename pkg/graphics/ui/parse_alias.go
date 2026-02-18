package ui

import "avyos.dev/pkg/graphics/uiparse"

type ValueKind = uiparse.ValueKind

const (
	ValueString = uiparse.ValueString
	ValueInt    = uiparse.ValueInt
	ValueFloat  = uiparse.ValueFloat
	ValueBool   = uiparse.ValueBool
	ValueIdent  = uiparse.ValueIdent
)

type Value = uiparse.Value
type Property = uiparse.Property
type Node = uiparse.Node
type ImportDecl = uiparse.ImportDecl
type SignalDecl = uiparse.SignalDecl
type ComponentDef = uiparse.ComponentDef
type Document = uiparse.Document

type ParseError = uiparse.ParseError
type BuildError = uiparse.BuildError

func Parse(source string) (*Document, error) {
	return uiparse.Parse(source)
}
