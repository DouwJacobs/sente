package app

import "finance-tracker/internal/statements"

type FNBTransaction = statements.FNBTransaction
type FNBReport = statements.FNBReport
type FNBCoverage = statements.FNBCoverage
type FNBNormalizedRun = statements.FNBNormalizedRun

func NormalizeFNBReport(report FNBReport, selectedBankID string) (FNBNormalizedRun, error) {
	return statements.NormalizeFNBReport(report, selectedBankID)
}
