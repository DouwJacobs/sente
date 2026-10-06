package app

import (
	"database/sql"
	"encoding/json"
)

type mcpCategoryAssignment struct {
	ID           int64 `json:"id"`
	Version      int64 `json:"version"`
	CategoryID   int64 `json:"category_id"`
	AllocationID int64 `json:"allocation_id,omitempty" jsonschema:"Required for a split; omit only when the transaction has exactly one allocation"`
}

func (a *App) prepareCategoryAssignments(tx *sql.Tx, identity mcpIdentity, in mcpChange) ([]mcpExactEdit, error) {
	if len(in.Categorizations) < 1 || len(in.Categorizations) > 100 {
		return nil, fail(400, "Choose 1–100 category assignments")
	}
	exact := []mcpExactEdit{}
	positions := map[int64]int{}
	targets := map[int64]bool{}
	for index, assignment := range in.Categorizations {
		if assignment.ID <= 0 || assignment.Version <= 0 || assignment.CategoryID <= 0 || assignment.AllocationID < 0 {
			return nil, fail(400, "Provide positive transaction, version, category and allocation IDs")
		}
		if queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=?", assignment.CategoryID) != 1 {
			return nil, fail(400, "Choose an existing category")
		}
		position, exists := positions[assignment.ID]
		if !exists {
			before, err := a.mcpTransaction(tx, identity.User, assignment.ID)
			if err != nil {
				return nil, err
			}
			if before.Transfer {
				return nil, fail(400, "Category assignment does not change transfers")
			}
			if before.Version != assignment.Version {
				return nil, fail(409, "Transaction changed; reload before preparing categories")
			}
			after := before
			after.Allocations = append([]Allocation{}, before.Allocations...)
			position = len(exact)
			positions[assignment.ID] = position
			exact = append(exact, mcpExactEdit{ID: assignment.ID, Before: before, After: after})
		}
		if exact[position].Before.Version != assignment.Version {
			return nil, fail(400, "Repeated transaction assignments must use the same version")
		}
		rows, err := data(tx, "SELECT id,category_id FROM allocations WHERE transaction_id=? ORDER BY id", assignment.ID)
		if err != nil {
			return nil, err
		}
		target := assignment.AllocationID
		if target == 0 {
			if len(rows) != 1 {
				return nil, fail(400, "Split transactions require an explicit allocation_id")
			}
			target = num(rows[0]["id"])
		}
		found := false
		for i, row := range rows {
			if num(row["id"]) != target {
				continue
			}
			found = true
			if targets[target] {
				return nil, fail(400, "Each allocation can be assigned only once")
			}
			if row["category_id"] != nil {
				return nil, fail(400, "Category assignment only fills missing categories")
			}
			targets[target] = true
			id := assignment.CategoryID
			exact[position].After.Allocations[i].CategoryID = &id
		}
		if !found {
			return nil, fail(400, "Allocation does not belong to this transaction")
		}
		in.Categorizations[index].AllocationID = target
	}
	return exact, nil
}
func (a *App) applyCategoryAssignments(tx *sql.Tx, identity mcpIdentity, exact mcpExactChange) (any, error) {
	expected, err := a.prepareCategoryAssignments(tx, identity, exact.Change)
	if err != nil {
		return nil, err
	}
	// Revalidate exact approved effects rather than regenerating different edits.
	if len(expected) != len(exact.Transactions) {
		return nil, fail(409, "Category proposal changed; prepare it again")
	}
	for i, edit := range expected {
		old, _ := json.Marshal(exact.Transactions[i])
		fresh, _ := json.Marshal(edit)
		if string(old) != string(fresh) {
			return nil, fail(409, "Category proposal is stale; prepare it again")
		}
	}
	for _, assignment := range exact.Change.Categorizations {
		res, err := tx.Exec("UPDATE allocations SET category_id=? WHERE id=? AND transaction_id=? AND category_id IS NULL", assignment.CategoryID, assignment.AllocationID, assignment.ID)
		if err != nil {
			return nil, err
		}
		if err = affected(res); err != nil {
			return nil, err
		}
	}
	for _, edit := range expected {
		res, err := tx.Exec("UPDATE transactions SET version=version+1,reviewed_by=NULL,reviewed_at=NULL WHERE id=? AND version=? AND is_transfer=0", edit.ID, edit.Before.Version)
		if err != nil {
			return nil, err
		}
		if err = affected(res); err != nil {
			return nil, err
		}
		account := queryInt(tx, "SELECT account_id FROM transactions WHERE id=?", edit.ID)
		if err = acceptCategorized(tx, identity.User, account, edit.ID); err != nil {
			return nil, err
		}
		if err = markSeen(tx, identity.User.ID, edit.ID, edit.Before.Version+1); err != nil {
			return nil, err
		}
		if err = audit(tx, identity.User, account, "transaction", edit.ID, "category_assigned", map[string]any{"before": edit.Before, "after": edit.After}); err != nil {
			return nil, err
		}
	}
	return map[string]any{"updated": len(expected), "assigned_allocations": len(exact.Change.Categorizations)}, nil
}
