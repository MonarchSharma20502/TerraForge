package catalog

import "sort"

// sortStrings sorts in place. Kept here so the catalog package has no other
// dependency on ordering behaviour.
func sortStrings(s []string) {
	sort.Strings(s)
}
