package app

import (
	"finance-tracker/internal/money"
	"finance-tracker/internal/statements"
)

// Aliases preserve the application facade while adapters own normalized models.
type SourceRow = statements.SourceRow
type ParsedFile = statements.ParsedFile
type inputFile = statements.InputFile

func expand(files []inputFile) ([]inputFile, error) { return statements.Expand(files) }
func parseFile(f inputFile) ParsedFile              { return statements.ParseFile(f) }
func Cents(s string) (int64, error)                 { return money.Cents(s) }
