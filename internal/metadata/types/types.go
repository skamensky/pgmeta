package types

// ObjectType represents the type of database object
type ObjectType string

const (
	TypeTable      ObjectType = "table"
	TypeView       ObjectType = "view"
	TypeFunction   ObjectType = "function"
	TypeTrigger    ObjectType = "trigger"
	TypeIndex      ObjectType = "index"
	TypeConstraint ObjectType = "constraint"
)

// DBObject represents a database object
type DBObject struct {
	Type       ObjectType
	Schema     string
	Name       string
	Definition string
	TableName  string // For indexes, triggers, and constraints - stores the parent table name
}

// QueryOptions contains options for database queries
type QueryOptions struct {
	Types      []ObjectType
	Schema     string
	Database   string
	NameRegex  string
}

// IsValidType checks if a given type is valid
func IsValidType(t ObjectType) bool {
	validTypes := map[ObjectType]bool{
		TypeTable:      true,
		TypeView:       true,
		TypeFunction:   true,
		TypeTrigger:    true,
		TypeIndex:      true,
		TypeConstraint: true,
	}
	return validTypes[t]
}

// ContainsAny checks if the slice contains any of the given elements
func ContainsAny(slice []ObjectType, elements ...ObjectType) bool {
	if len(slice) == 0 {
		return true // If no types specified, include all
	}
	for _, s := range slice {
		for _, e := range elements {
			if s == e {
				return true
			}
		}
	}
	return false
}