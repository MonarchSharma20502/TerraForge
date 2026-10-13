// Package analyze: the classifier maps azurerm resource types onto catalog kinds
// so an analyzed node carries the icon, category and stack the palette already
// knows. Types the catalog does not know - other clouds, kubernetes, null - keep
// their raw type and are labelled with the provider local name, so they still
// render as nodes instead of being dropped.
package analyze

import (
	"strings"

	"github.com/autonation/autonation/internal/catalog"
)

// Classifier turns resource types into catalog metadata.
type Classifier struct {
	cat    *catalog.Catalog
	byType map[string]*catalog.Entry
}

// NewClassifier builds the reverse index over a catalog.
func NewClassifier(cat *catalog.Catalog) *Classifier {
	c := &Classifier{
		cat:    cat,
		byType: map[string]*catalog.Entry{},
	}
	if cat == nil {
		return c
	}
	for _, kind := range cat.KindList() {
		entry, ok := cat.Get(kind)
		if !ok || entry.AzureType == "" {
			continue
		}
		c.byType[entry.AzureType] = entry
	}
	return c
}

// Classify fills the catalog-derived fields of a node. A node whose type is not
// an azurerm kind keeps Kind empty and takes its provider local name as its
// category, which still earns it a tier colour in the canvas.
func (c *Classifier) Classify(n *Node) {
	if entry, ok := c.byType[n.Type]; ok {
		n.Kind = entry.Kind
		n.Icon = entry.Icon
		n.Category = entry.Category
		return
	}
	n.Kind = ""
	n.Icon = ""
	n.Category = providerOf(n.Type)
}

// providerOf returns the provider local name of a resource type: the segment
// before the first underscore. azurerm_linux_web_app becomes azurerm; a type
// with no underscore is returned as-is.
func providerOf(resourceType string) string {
	if i := strings.IndexByte(resourceType, '_'); i > 0 {
		return resourceType[:i]
	}
	return resourceType
}

// IsAzure reports whether a node is an azurerm resource the catalog knows.
func (c *Classifier) IsAzure(n *Node) bool {
	_, ok := c.byType[n.Type]
	return ok
}

// KindOf returns the catalog kind for a resource type, or "" when unknown.
func (c *Classifier) KindOf(resourceType string) string {
	if entry, ok := c.byType[resourceType]; ok {
		return entry.Kind
	}
	return ""
}

// CategoryOf returns the catalog category for a resource type, or the provider
// local name when the type is unknown to the catalog.
func (c *Classifier) CategoryOf(resourceType string) string {
	if entry, ok := c.byType[resourceType]; ok {
		return entry.Category
	}
	return providerOf(resourceType)
}
