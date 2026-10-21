package sema

// grantingBorrowAncestor follows recorded reborrow edges within the same
// canonical storage root. Access through a descendant uses the authority
// granted by its ancestors; siblings and descendants do not grant authority
// to their parent. All represented parent edges must grant the relationship;
// unknown, cyclic or loop-carried edges cannot supply a transitive proof.
// Rules: rules/memory/borrowing.md — §§2.5,3.2(4),4(3),10(6–8);
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§43,60–62.
func (a *Analyzer) grantingBorrowAncestor(place Place, record borrowRecord) bool {
	if place.ReferenceHolder == "" || record.Holder == "" {
		return false
	}
	if place.ReferenceHolder == record.Holder {
		return true
	}
	if place.RootID == 0 {
		return false
	}
	active, done, memo := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var grants func(string) bool
	grants = func(holder string) bool {
		if holder == record.Holder {
			return true
		}
		if active[holder] {
			return false
		}
		if done[holder] {
			return memo[holder]
		}
		active[holder] = true
		matched, valid := false, true
		for _, edge := range a.borrows[place.Root] {
			if isDeferredUseKind(edge.Kind) || edge.Holder != holder {
				continue
			}
			if edge.Place.RootID != 0 && edge.Place.RootID != place.RootID {
				continue
			}
			matched = true
			parent := edge.Place.ReferenceHolder
			if edge.Place.RootID == 0 || edge.Place.AmbiguousProvenance || edge.LoopCarried || parent == "" || !grants(parent) {
				valid = false
				break
			}
		}
		active[holder], done[holder] = false, true
		memo[holder] = matched && valid
		return memo[holder]
	}
	return grants(place.ReferenceHolder)
}
