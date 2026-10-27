package app

import (
	"database/sql"
	"strings"
)

type mcpTransactionRemovalInput struct {
	Keep   *mcpSeenItem  `json:"keep,omitempty"`
	Items  []mcpSeenItem `json:"items"`
	Reason string        `json:"reason"`
}
type mcpTransactionRemoval struct {
	Keep    *transactionRemovalSnapshot  `json:"keep,omitempty"`
	Removed []transactionRemovalSnapshot `json:"removed"`
}

func (a *App) prepareMCPTransactionRemoval(tx *sql.Tx, identity mcpIdentity, b mcpChange) (*mcpTransactionRemoval, error) {
	in := b.Removal
	if in == nil || len(in.Items) < 1 || len(in.Items) > 100 || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 500 {
		return nil, fail(400, "Select 1–100 transactions and provide a reason of at most 500 bytes")
	}
	merge := b.Operation == "merge_transactions"
	if merge != (in.Keep != nil) {
		return nil, fail(400, "Merge requires one keeper; deletion must not specify a keeper")
	}
	out := &mcpTransactionRemoval{}
	seen := map[int64]bool{}
	if merge {
		keep, err := a.transactionRemovalSnapshot(tx, identity.User, *in.Keep)
		if err != nil {
			return nil, err
		}
		out.Keep, seen[keep.ID] = &keep, true
	}
	for _, item := range in.Items {
		if seen[item.ID] {
			return nil, fail(400, "Each transaction can appear only once, including the keeper")
		}
		seen[item.ID] = true
		snapshot, err := a.transactionRemovalSnapshot(tx, identity.User, item)
		if err != nil {
			return nil, err
		}

		out.Removed = append(out.Removed, snapshot)
	}
	return out, validateTransactionRemovalSelection(tx, out.Keep, out.Removed, in.Reason)
}

func (a *App) authorizeMCPTransactionRemoval(q queryer, identity mcpIdentity, p mcpPermissions, exact mcpExactChange) error {
	if exact.Removal == nil || len(exact.Removal.Removed) < 1 || len(exact.Removal.Removed) > 100 ||
		(exact.Change.Operation == "merge_transactions" && (!p.Capabilities.MergeTransactions || exact.Removal.Keep == nil)) ||
		(exact.Change.Operation == "delete_transactions" && (!p.Capabilities.DeleteTransactions || exact.Removal.Keep != nil)) || p.Constraints.MissingOnly {
		return fail(403, "This connection does not permit transaction removal")
	}
	items := append([]transactionRemovalSnapshot(nil), exact.Removal.Removed...)
	if exact.Removal.Keep != nil {
		items = append(items, *exact.Removal.Keep)
	}
	for _, item := range items {
		// Use bound account evidence for replay after deletion; current grants still apply.
		if !p.allowsAccount(item.AccountID) || !ruleAccess(q, a, identity.User, item.AccountID) {
			return fail(403, "Account editor access required")
		}
	}
	return nil
}
